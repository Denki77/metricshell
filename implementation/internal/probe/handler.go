package probe

import (
	"net/http"

	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
)

const (
	HealthPath    = "/healthz"
	ReadinessPath = "/readyz"
)

type StateSource interface {
	State() lifecycle.State
}

type Handler struct {
	state StateSource
}

func New(state StateSource) *Handler {
	return &Handler{state: state}
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != HealthPath && request.URL.Path != ReadinessPath {
		write(response, http.StatusNotFound, "not found\n")
		return
	}
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		write(response, http.StatusMethodNotAllowed, "method not allowed\n")
		return
	}

	health, readiness := lifecycle.ProbeStatuses(handler.state.State())
	if request.URL.Path == HealthPath {
		switch health {
		case http.StatusOK:
			write(response, health, "ok\n")
		case http.StatusInternalServerError:
			write(response, health, "failed\n")
		default:
			write(response, http.StatusServiceUnavailable, "unavailable\n")
		}
		return
	}

	if readiness == http.StatusOK {
		write(response, readiness, "ready\n")
		return
	}
	if readiness == 0 {
		write(response, http.StatusServiceUnavailable, "unavailable\n")
		return
	}
	write(response, readiness, "not ready\n")
}

func write(response http.ResponseWriter, status int, body string) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.WriteHeader(status)
	_, _ = response.Write([]byte(body))
}
