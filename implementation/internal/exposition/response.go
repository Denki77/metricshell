package exposition

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

type Format string

const (
	Prometheus  Format = "prometheus"
	OpenMetrics Format = "openmetrics"
)

type Encoding string

const (
	Identity Encoding = "identity"
	Gzip     Encoding = "gzip"
)

type Outcome string

const (
	Success       Outcome = "success"
	WriteError    Outcome = "write_error"
	ResponseLimit Outcome = "response_limit"
	EncodingError Outcome = "encoding_error"
	Timeout       Outcome = "timeout"
)

var ErrResponseLimit = errors.New("exposition response exceeds limit")

type FamilyFilter func(snapshot.Family) bool

type Prepared struct {
	format            Format
	encoding          Encoding
	generation        uint64
	uncompressedBytes int
	body              []byte
}

func (response Prepared) Format() Format         { return response.format }
func (response Prepared) Encoding() Encoding     { return response.encoding }
func (response Prepared) Generation() uint64     { return response.generation }
func (response Prepared) UncompressedBytes() int { return response.uncompressedBytes }
func (response Prepared) Body() []byte           { return append([]byte(nil), response.body...) }

func Prepare(active snapshot.ActiveSnapshot, metrics selfmetric.View, filter FamilyFilter, format Format, encoding Encoding, limit int) (Prepared, error) {
	if limit < 1 || (encoding != Identity && encoding != Gzip) {
		return Prepared{}, EncodingErrorValue
	}
	body, err := Encode(active.Validated(), metrics, filter, format)
	if err != nil {
		return Prepared{}, err
	}
	if len(body) > limit {
		return Prepared{}, ErrResponseLimit
	}
	prepared := Prepared{format: format, encoding: encoding, generation: active.Generation(), uncompressedBytes: len(body), body: body}
	if encoding == Gzip {
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err := writer.Write(body); err != nil {
			return Prepared{}, err
		}
		if err := writer.Close(); err != nil {
			return Prepared{}, err
		}
		prepared.body = compressed.Bytes()
	}
	return prepared, nil
}

var EncodingErrorValue = errors.New("invalid exposition encoding configuration")

func Write(ctx context.Context, response http.ResponseWriter, prepared Prepared, timeout time.Duration) Outcome {
	if ctx == nil || response == nil || timeout <= 0 {
		return EncodingError
	}
	if err := ctx.Err(); err != nil {
		return Timeout
	}
	deadline := time.Now().Add(timeout)
	controller := http.NewResponseController(response)
	if err := controller.SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return WriteError
	}
	response.Header().Set("Content-Type", ContentType(prepared.format))
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	if prepared.encoding == Gzip {
		response.Header().Set("Content-Encoding", "gzip")
		response.Header().Set("Vary", "Accept-Encoding")
	}
	response.Header().Set("Content-Length", integerString(len(prepared.body)))
	response.WriteHeader(http.StatusOK)
	written, err := response.Write(prepared.body)
	if err == nil && written != len(prepared.body) {
		err = io.ErrShortWrite
	}
	if err != nil {
		if ctx.Err() != nil || time.Now().After(deadline) {
			return Timeout
		}
		return WriteError
	}
	if ctx.Err() != nil {
		return Timeout
	}
	return Success
}

func ContentType(format Format) string {
	if format == OpenMetrics {
		return "application/openmetrics-text; version=1.0.0; charset=utf-8"
	}
	return "text/plain; version=0.0.4; charset=utf-8"
}

type Limiter struct {
	capacity chan struct{}
}

func NewLimiter(concurrent int) (*Limiter, error) {
	if concurrent < 1 {
		return nil, EncodingErrorValue
	}
	return &Limiter{capacity: make(chan struct{}, concurrent)}, nil
}

func (limiter *Limiter) Acquire() bool {
	select {
	case limiter.capacity <- struct{}{}:
		return true
	default:
		return false
	}
}

func (limiter *Limiter) Release() { <-limiter.capacity }

func integerString(value int) string {
	var buffer [20]byte
	index := len(buffer)
	if value == 0 {
		return "0"
	}
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[index:])
}
