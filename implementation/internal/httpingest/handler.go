package httpingest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

const (
	Path      = "/v1/metrics"
	MediaType = "application/vnd.metricshell.snapshot+json"
)

type Config struct {
	WireBytes         int
	DecodedBytes      int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

func DefaultConfig() Config {
	return Config{
		WireBytes: 2 << 20, DecodedBytes: 2 << 20, ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second,
		MaxHeaderBytes: 8 << 10,
	}
}

func (configuration Config) Validate() error {
	if configuration.WireBytes < 1 || configuration.DecodedBytes < 1 || configuration.ReadHeaderTimeout <= 0 ||
		configuration.ReadTimeout <= 0 || configuration.WriteTimeout <= 0 || configuration.IdleTimeout <= 0 || configuration.MaxHeaderBytes < 1 {
		return errors.New("invalid HTTP ingestion configuration")
	}
	return nil
}

type Handler struct {
	configuration Config
	publisher     ingestion.Publisher
}

func NewHandler(configuration Config, publisher ingestion.Publisher) (*Handler, error) {
	if err := configuration.Validate(); err != nil || publisher == nil {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("publisher is required")
	}
	return &Handler{configuration: configuration, publisher: publisher}, nil
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != Path {
		http.NotFound(response, request)
		return
	}
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeNACK(response, http.StatusMethodNotAllowed, "method")
		return
	}
	if !acceptedMediaType(request.Header.Get("Content-Type")) {
		writeNACK(response, http.StatusUnsupportedMediaType, "media_type")
		return
	}
	encoding := strings.ToLower(strings.TrimSpace(request.Header.Get("Content-Encoding")))
	if encoding == "" {
		encoding = "identity"
	}
	if encoding != "identity" && encoding != "gzip" {
		writeNACK(response, http.StatusUnsupportedMediaType, "encoding")
		return
	}
	if request.ContentLength > int64(handler.configuration.WireBytes) {
		writeNACK(response, http.StatusRequestEntityTooLarge, string(snapshot.ReasonPayloadLimit))
		return
	}
	wire, err := io.ReadAll(io.LimitReader(request.Body, int64(handler.configuration.WireBytes)+1))
	if err != nil {
		if request.Context().Err() != nil {
			writeNACK(response, http.StatusRequestTimeout, string(ingestion.Timeout))
			return
		}
		writeNACK(response, http.StatusBadRequest, string(snapshot.ReasonMalformed))
		return
	}
	if len(wire) > handler.configuration.WireBytes {
		writeNACK(response, http.StatusRequestEntityTooLarge, string(snapshot.ReasonPayloadLimit))
		return
	}
	decoded, reason := handler.decode(wire, encoding)
	if reason != "" {
		status := http.StatusBadRequest
		if reason == string(snapshot.ReasonPayloadLimit) {
			status = http.StatusRequestEntityTooLarge
		}
		writeNACK(response, status, reason)
		return
	}
	result := handler.publisher.Publish(request.Context(), ingestion.HTTP, decoded)
	switch result.Outcome {
	case ingestion.Accepted:
		writeACK(response, result.Generation)
	case ingestion.Rejected:
		writeNACK(response, rejectionStatus(result.Reason), string(result.Reason))
	case ingestion.Busy:
		writeNACK(response, http.StatusTooManyRequests, string(ingestion.Busy))
	case ingestion.Timeout:
		writeNACK(response, http.StatusRequestTimeout, string(ingestion.Timeout))
	default:
		writeNACK(response, http.StatusInternalServerError, string(snapshot.ReasonInternal))
	}
}

func (handler *Handler) decode(wire []byte, encoding string) ([]byte, string) {
	var reader io.Reader = bytes.NewReader(wire)
	if encoding == "gzip" {
		compressed, err := gzip.NewReader(reader)
		if err != nil {
			return nil, string(snapshot.ReasonMalformed)
		}
		defer compressed.Close()
		reader = compressed
	}
	decoded, err := io.ReadAll(io.LimitReader(reader, int64(handler.configuration.DecodedBytes)+1))
	if err != nil {
		return nil, string(snapshot.ReasonMalformed)
	}
	if len(decoded) > handler.configuration.DecodedBytes {
		return nil, string(snapshot.ReasonPayloadLimit)
	}
	return decoded, ""
}

func acceptedMediaType(value string) bool {
	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(mediaType) {
	case "application/json":
		return len(parameters) == 0
	case MediaType:
		return len(parameters) == 1 && parameters["version"] == "1"
	default:
		return false
	}
}

func rejectionStatus(reason snapshot.Reason) int {
	switch reason {
	case snapshot.ReasonPayloadLimit:
		return http.StatusRequestEntityTooLarge
	case snapshot.ReasonSeriesLimit, snapshot.ReasonLabelLimit, snapshot.ReasonNameLimit, snapshot.ReasonPolicy:
		return http.StatusUnprocessableEntity
	case snapshot.ReasonFrozen:
		return http.StatusConflict
	case snapshot.ReasonInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

func writeACK(response http.ResponseWriter, generation uint64) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	fmt.Fprintf(response, "{\"schema_version\":1,\"status\":\"ack\",\"generation\":%d}\n", generation)
}

func writeNACK(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	encoded, _ := json.Marshal(code)
	fmt.Fprintf(response, "{\"schema_version\":1,\"status\":\"nack\",\"code\":%s}\n", encoded)
}

func ValidateListenAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return errors.New("HTTP ingestion address requires a loopback host")
	}
	addresses, err := net.LookupIP(host)
	if err != nil || len(addresses) == 0 {
		return errors.New("HTTP ingestion host cannot be resolved")
	}
	for _, address := range addresses {
		if !address.IsLoopback() {
			return errors.New("HTTP ingestion address is not loopback")
		}
	}
	return nil
}

func NewServer(address string, handler *Handler, configuration Config) (*http.Server, error) {
	if handler == nil {
		return nil, errors.New("handler is required")
	}
	if err := configuration.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateListenAddress(address); err != nil {
		return nil, err
	}
	return &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: configuration.ReadHeaderTimeout,
		ReadTimeout: configuration.ReadTimeout, WriteTimeout: configuration.WriteTimeout,
		IdleTimeout: configuration.IdleTimeout, MaxHeaderBytes: configuration.MaxHeaderBytes,
	}, nil
}
