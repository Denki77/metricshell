package exposition

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/probe"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

const (
	MetricsPath = "/metrics"
	DebugPath   = "/debug/config"
)

type Config struct {
	Listen          string
	ResponseBytes   int
	Concurrent      int
	WriteTimeout    time.Duration
	Include         []string
	Exclude         []string
	ReadHeaderLimit time.Duration
}

func DefaultConfig() Config {
	return Config{
		Listen: "0.0.0.0:9090", ResponseBytes: 8 << 20, Concurrent: 32,
		WriteTimeout: 30 * time.Second, ReadHeaderLimit: 5 * time.Second,
	}
}

func (configuration Config) Validate() error {
	host, _, err := net.SplitHostPort(configuration.Listen)
	if err != nil || host == "" || configuration.ResponseBytes < 1 || configuration.Concurrent < 1 || configuration.WriteTimeout <= 0 || configuration.ReadHeaderLimit <= 0 {
		return EncodingErrorValue
	}
	_, err = NewFilter(configuration.Include, configuration.Exclude)
	return err
}

type SnapshotSource interface {
	Active() snapshot.ActiveSnapshot
}

type MetricsSource interface {
	View() selfmetric.View
	SetGauge(string, map[string]string, float64) error
	AddCounter(string, map[string]string, uint64) error
}

type DebugView func() []byte
type FailureObserver func(Outcome, int)

type Handler struct {
	configuration Config
	snapshots     SnapshotSource
	metrics       MetricsSource
	filter        *Filter
	limiter       *Limiter
	probes        http.Handler
	debug         DebugView
	failures      []FailureObserver
	mu            sync.Mutex
	inflight      int
}

func NewHandler(configuration Config, snapshots SnapshotSource, metrics MetricsSource, probes http.Handler, debug DebugView, failures ...FailureObserver) (*Handler, error) {
	if snapshots == nil || metrics == nil || probes == nil || configuration.Validate() != nil {
		return nil, EncodingErrorValue
	}
	filter, err := NewFilter(configuration.Include, configuration.Exclude)
	if err != nil {
		return nil, err
	}
	limiter, err := NewLimiter(configuration.Concurrent)
	if err != nil {
		return nil, err
	}
	handler := &Handler{configuration: configuration, snapshots: snapshots, metrics: metrics, filter: filter, limiter: limiter, probes: probes, debug: debug, failures: failures}
	include, exclude := filter.RuleCounts()
	_ = metrics.SetGauge(selfmetric.FilterRules, map[string]string{"kind": "include"}, float64(include))
	_ = metrics.SetGauge(selfmetric.FilterRules, map[string]string{"kind": "exclude"}, float64(exclude))
	return handler, nil
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case probe.HealthPath, probe.ReadinessPath:
		handler.probes.ServeHTTP(response, request)
	case DebugPath:
		handler.serveDebug(response, request)
	case MetricsPath:
		handler.serveMetrics(response, request)
	default:
		writeFailure(response, http.StatusNotFound, "not found\n")
	}
}

func (handler *Handler) serveDebug(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		writeFailure(response, http.StatusMethodNotAllowed, "method not allowed\n")
		return
	}
	content := []byte("{}\n")
	if handler.debug != nil {
		content = handler.debug()
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(content)
}

func (handler *Handler) serveMetrics(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		writeFailure(response, http.StatusMethodNotAllowed, "method not allowed\n")
		return
	}
	format, ok := negotiateFormat(request.Header.Get("Accept"))
	if !ok {
		writeFailure(response, http.StatusNotAcceptable, "not acceptable\n")
		return
	}
	encoding, ok := negotiateEncoding(request.Header.Get("Accept-Encoding"))
	if !ok {
		writeFailure(response, http.StatusNotAcceptable, "not acceptable\n")
		return
	}
	if !handler.limiter.Acquire() {
		writeFailure(response, http.StatusServiceUnavailable, "exposition busy\n")
		return
	}
	defer handler.limiter.Release()
	handler.changeInflight(1)
	defer handler.changeInflight(-1)

	active := handler.snapshots.Active()
	families := active.Validated().Families()
	included, excluded := handler.filter.Counts(families)
	_ = handler.metrics.SetGauge(selfmetric.FilterFamilies, map[string]string{"outcome": "included"}, float64(included))
	_ = handler.metrics.SetGauge(selfmetric.FilterFamilies, map[string]string{"outcome": "excluded"}, float64(excluded))
	prepared, err := Prepare(active, handler.metrics.View(), handler.filter.Includes, format, encoding, handler.configuration.ResponseBytes)
	if err != nil {
		outcome := EncodingError
		if errors.Is(err, ErrResponseLimit) {
			outcome = ResponseLimit
		}
		handler.observe(format, outcome, 0)
		handler.notifyFailure(outcome, http.StatusServiceUnavailable)
		writeFailure(response, http.StatusServiceUnavailable, "exposition unavailable\n")
		return
	}
	outcome := Write(request.Context(), response, prepared, handler.configuration.WriteTimeout)
	handler.observe(format, outcome, prepared.UncompressedBytes())
	if outcome != Success {
		handler.notifyFailure(outcome, http.StatusOK)
	}
}

func (handler *Handler) notifyFailure(outcome Outcome, status int) {
	for _, observer := range handler.failures {
		if observer != nil {
			observer(outcome, status)
		}
	}
}

func (handler *Handler) changeInflight(delta int) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.inflight += delta
	_ = handler.metrics.SetGauge(selfmetric.ExpositionInflight, nil, float64(handler.inflight))
}

func (handler *Handler) observe(format Format, outcome Outcome, responseBytes int) {
	_ = handler.metrics.AddCounter(selfmetric.ExpositionRequestsTotal, map[string]string{"format": string(format), "outcome": string(outcome)}, 1)
	if responseBytes > 0 {
		_ = handler.metrics.SetGauge(selfmetric.ExpositionResponseBytes, map[string]string{"format": string(format)}, float64(responseBytes))
	}
}

func negotiateFormat(value string) (Format, bool) {
	if strings.TrimSpace(value) == "" {
		return Prometheus, true
	}
	for _, item := range strings.Split(value, ",") {
		media, quality := parseWeightedToken(item)
		if quality == 0 {
			continue
		}
		if media == "application/openmetrics-text" {
			return OpenMetrics, true
		}
	}
	for _, item := range strings.Split(value, ",") {
		media, quality := parseWeightedToken(item)
		if quality > 0 && (media == "text/plain" || media == "text/*" || media == "*/*") {
			return Prometheus, true
		}
	}
	return "", false
}

func negotiateEncoding(value string) (Encoding, bool) {
	if strings.TrimSpace(value) == "" {
		return Identity, true
	}
	identityAllowed := true
	for _, item := range strings.Split(value, ",") {
		name, quality := parseWeightedToken(item)
		if name == "gzip" && quality > 0 {
			return Gzip, true
		}
		if name == "identity" && quality == 0 || name == "*" && quality == 0 {
			identityAllowed = false
		}
	}
	return Identity, identityAllowed
}

func parseWeightedToken(value string) (string, float64) {
	parts := strings.Split(value, ";")
	name := strings.ToLower(strings.TrimSpace(parts[0]))
	quality := 1.0
	for _, parameter := range parts[1:] {
		key, raw, found := strings.Cut(strings.TrimSpace(parameter), "=")
		if found && strings.EqualFold(key, "q") {
			parsed, err := strconv.ParseFloat(raw, 64)
			if err != nil || parsed < 0 || parsed > 1 {
				return name, 0
			}
			quality = parsed
		}
	}
	return name, quality
}

func writeFailure(response http.ResponseWriter, status int, body string) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.WriteHeader(status)
	_, _ = response.Write([]byte(body))
}

type Server struct {
	listener net.Listener
	server   *http.Server
	done     chan error
}

func Bind(configuration Config, handler http.Handler) (*Server, error) {
	if handler == nil || configuration.Validate() != nil {
		return nil, EncodingErrorValue
	}
	listener, err := net.Listen("tcp", configuration.Listen)
	if err != nil {
		return nil, err
	}
	server := &Server{
		listener: listener,
		server: &http.Server{
			Handler: handler, ReadHeaderTimeout: configuration.ReadHeaderLimit,
			WriteTimeout: configuration.WriteTimeout, MaxHeaderBytes: 16 << 10,
		},
		done: make(chan error, 1),
	}
	return server, nil
}

func (server *Server) Start() {
	go func() {
		err := server.server.Serve(server.listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		server.done <- err
	}()
}

func (server *Server) Address() net.Addr { return server.listener.Addr() }

func (server *Server) Shutdown(ctx context.Context) error {
	err := server.server.Shutdown(ctx)
	serveErr := <-server.done
	if err != nil {
		return err
	}
	return serveErr
}
