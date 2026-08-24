package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"

	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
	metricProbe "github.com/Denki77/metricshell/implementation/internal/probe"
)

type stateSource struct {
	mu    sync.RWMutex
	state lifecycle.State
}

func (source *stateSource) State() lifecycle.State {
	source.mu.RLock()
	defer source.mu.RUnlock()
	return source.state
}

func (source *stateSource) set(state lifecycle.State) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.state = state
}

func main() {
	source := &stateSource{}
	server := httptest.NewServer(metricProbe.New(source))
	defer server.Close()

	requests := 0
	for _, state := range lifecycle.States {
		source.set(state)
		health, readiness := lifecycle.ProbeStatuses(state)
		if health == 0 {
			health = http.StatusServiceUnavailable
		}
		if readiness == 0 {
			readiness = http.StatusServiceUnavailable
		}
		for path, want := range map[string]int{metricProbe.HealthPath: health, metricProbe.ReadinessPath: readiness} {
			response, err := http.Get(server.URL + path)
			if err != nil {
				fail(err)
			}
			_, readErr := io.Copy(io.Discard, response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil || response.StatusCode != want {
				fail(fmt.Errorf("%s %s returned %d, want %d", state, path, response.StatusCode, want))
			}
			requests++
		}
	}

	request, err := http.NewRequest(http.MethodPost, server.URL+metricProbe.HealthPath, nil)
	if err != nil {
		fail(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fail(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed || response.Header.Get("Allow") != http.MethodGet {
		fail(fmt.Errorf("POST healthz returned %d", response.StatusCode))
	}
	requests++

	response, err = http.Get(server.URL + "/unknown")
	if err != nil {
		fail(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		fail(fmt.Errorf("unknown path returned %d", response.StatusCode))
	}
	requests++

	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
		"event":            "fixture.probes",
		"states_checked":   len(lifecycle.States),
		"requests_checked": requests,
	}); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
