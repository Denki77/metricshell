package probe

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
)

type fixedState lifecycle.State

func (state fixedState) State() lifecycle.State { return lifecycle.State(state) }

func TestStateByEndpointContract(t *testing.T) {
	want := map[lifecycle.State]map[string]struct {
		status int
		body   string
	}{
		lifecycle.Initializing:     {HealthPath: {200, "ok\n"}, ReadinessPath: {503, "not ready\n"}},
		lifecycle.StartingWorkload: {HealthPath: {200, "ok\n"}, ReadinessPath: {503, "not ready\n"}},
		lifecycle.Running:          {HealthPath: {200, "ok\n"}, ReadinessPath: {200, "ready\n"}},
		lifecycle.Stopping:         {HealthPath: {200, "ok\n"}, ReadinessPath: {503, "not ready\n"}},
		lifecycle.Finalizing:       {HealthPath: {200, "ok\n"}, ReadinessPath: {503, "not ready\n"}},
		lifecycle.FinalWait:        {HealthPath: {200, "ok\n"}, ReadinessPath: {503, "not ready\n"}},
		lifecycle.Failed:           {HealthPath: {500, "failed\n"}, ReadinessPath: {503, "not ready\n"}},
		lifecycle.Terminated:       {HealthPath: {503, "unavailable\n"}, ReadinessPath: {503, "unavailable\n"}},
	}

	for _, state := range lifecycle.States {
		for _, path := range []string{HealthPath, ReadinessPath} {
			t.Run(string(state)+path, func(t *testing.T) {
				response := httptest.NewRecorder()
				New(fixedState(state)).ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				assertResponse(t, response.Result(), want[state][path].status, want[state][path].body)
			})
		}
	}
}

func TestMethodAndPathErrorsAreDeterministic(t *testing.T) {
	handler := New(fixedState(lifecycle.Running))
	for _, path := range []string{"/", "/metrics", "/healthz/", "/readyz/"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		assertResponse(t, response.Result(), http.StatusNotFound, "not found\n")
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodHead, http.MethodDelete} {
		for _, path := range []string{HealthPath, ReadinessPath} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
			if response.Header().Get("Allow") != http.MethodGet {
				t.Fatalf("Allow = %q, want GET", response.Header().Get("Allow"))
			}
			assertResponse(t, response.Result(), http.StatusMethodNotAllowed, "method not allowed\n")
		}
	}
}

func TestConcurrentLifecycleTransitionsAndProbes(t *testing.T) {
	machine, err := lifecycle.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(machine)
	transitions := []lifecycle.Event{
		lifecycle.ConfigurationValidated,
		lifecycle.WorkloadStarted,
		lifecycle.TerminationAfterSpawn,
		lifecycle.WorkloadExited,
		lifecycle.FinalizationImmediate,
	}

	var wait sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for request := 0; request < 100; request++ {
				for _, path := range []string{HealthPath, ReadinessPath} {
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
					if status := response.Code; status != 200 && status != 503 {
						t.Errorf("%s status during transition = %d", path, status)
					}
				}
			}
		}()
	}
	for _, event := range transitions {
		if err := machine.TransitionEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	wait.Wait()
}

func TestProbeRoutesNeverCompleteFinalScrape(t *testing.T) {
	var finalScrapes atomic.Int64
	handler := New(fixedState(lifecycle.FinalWait))
	router := http.NewServeMux()
	router.Handle(HealthPath, handler)
	router.Handle(ReadinessPath, handler)
	router.HandleFunc("/metrics", func(response http.ResponseWriter, _ *http.Request) {
		finalScrapes.Add(1)
		response.WriteHeader(http.StatusOK)
	})

	for request := 0; request < 100; request++ {
		for _, path := range []string{HealthPath, ReadinessPath} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		}
	}
	if count := finalScrapes.Load(); count != 0 {
		t.Fatalf("final scrape completions = %d, want 0", count)
	}
}

func assertResponse(t *testing.T, response *http.Response, status int, body string) {
	t.Helper()
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status || string(content) != body {
		t.Fatalf("response = (%d, %q), want (%d, %q)", response.StatusCode, content, status, body)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header.Get("Cache-Control"))
	}
}
