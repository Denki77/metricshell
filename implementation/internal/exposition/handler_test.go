package exposition

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
	"github.com/Denki77/metricshell/implementation/internal/probe"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

func TestFilterSyntaxPrecedenceAndCounts(t *testing.T) {
	validated := parseApplication(t, `{"schema_version":1,"families":[`+
		`{"name":"application_jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]},`+
		`{"name":"application_debug_cache","help":"","type":"gauge","series":[{"labels":{},"value":"2"}]},`+
		`{"name":"other","help":"","type":"gauge","series":[{"labels":{},"value":"3"}]}`+
		`]}`)
	filter, err := NewFilter([]string{"prefix:application_", "prefix:application_"}, []string{"name:application_debug_cache"})
	if err != nil {
		t.Fatal(err)
	}
	includeRules, excludeRules := filter.RuleCounts()
	included, excluded := filter.Counts(validated.Families())
	if includeRules != 1 || excludeRules != 1 || included != 1 || excluded != 2 {
		t.Fatalf("rules=%d/%d families=%d/%d", includeRules, excludeRules, included, excluded)
	}
	for _, invalid := range []string{"", "name:", "prefix:*", "regex:x", "name:with space", "NAME:x"} {
		if _, err := NewFilter([]string{invalid}, nil); err == nil {
			t.Errorf("selector %q accepted", invalid)
		}
	}
}

func TestHandlerNegotiatesFormatsCompressionFilteringAndProbes(t *testing.T) {
	holder, registry, state := handlerState(t)
	configuration := DefaultConfig()
	configuration.Listen = "127.0.0.1:0"
	configuration.Include = []string{"prefix:application_"}
	configuration.Exclude = []string{"name:application_debug"}
	handler, err := NewHandler(configuration, holder, registry, probe.New(state), func() []byte { return []byte("{\"safe\":true}\n") })
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name     string
		accept   string
		encoding string
		format   Format
	}{
		{"default", "", "", Prometheus},
		{"prometheus", "text/plain; version=0.0.4", "", Prometheus},
		{"openmetrics", "application/openmetrics-text; version=1.0.0", "", OpenMetrics},
		{"gzip", "application/openmetrics-text", "gzip", OpenMetrics},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, MetricsPath, nil)
			request.Header.Set("Accept", test.accept)
			request.Header.Set("Accept-Encoding", test.encoding)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != ContentType(test.format) {
				t.Fatalf("response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
			body := response.Body.Bytes()
			if test.encoding == "gzip" {
				reader, err := gzip.NewReader(bytes.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				body, err = io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
			}
			content := string(body)
			if !strings.Contains(content, "application_jobs ") || strings.Contains(content, "application_debug ") || strings.Contains(content, "other_metric ") || !strings.Contains(content, "metricshell_build_info") {
				t.Fatalf("filter mismatch:\n%s", content)
			}
			if strings.HasSuffix(content, "# EOF\n") != (test.format == OpenMetrics) {
				t.Fatalf("EOF mismatch:\n%s", content)
			}
		})
	}

	for _, path := range []string{probe.HealthPath, probe.ReadinessPath, DebugPath} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("%s status=%d", path, response.Code)
		}
	}
	view := registry.View()
	if counterValue(view, selfmetric.ExpositionRequestsTotal, map[string]string{"format": "openmetrics", "outcome": "success"}) != 2 {
		t.Fatal("OpenMetrics success counter mismatch")
	}
}

func TestHandlerFailuresBeforeSuccessAndSaturation(t *testing.T) {
	holder, registry, state := handlerState(t)
	configuration := DefaultConfig()
	configuration.Listen = "127.0.0.1:0"
	configuration.ResponseBytes = 1
	handler, err := NewHandler(configuration, holder, registry, probe.New(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Header().Get("Content-Type"), "version=0.0.4") {
		t.Fatalf("preflight failure committed success metadata: %d %v", response.Code, response.Header())
	}

	for _, test := range []struct {
		method string
		path   string
		accept string
		want   int
	}{
		{http.MethodPost, MetricsPath, "", http.StatusMethodNotAllowed},
		{http.MethodGet, "/missing", "", http.StatusNotFound},
		{http.MethodGet, MetricsPath, "application/json", http.StatusNotAcceptable},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		request.Header.Set("Accept", test.accept)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Errorf("%s %s status=%d want=%d", test.method, test.path, response.Code, test.want)
		}
	}

	configuration.ResponseBytes = 1 << 20
	configuration.Concurrent = 1
	handler, _ = NewHandler(configuration, holder, registry, probe.New(state), nil)
	blocked := newBlockingResponseWriter()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
		close(done)
	}()
	<-blocked.entered
	busy := httptest.NewRecorder()
	handler.ServeHTTP(busy, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if busy.Code != http.StatusServiceUnavailable {
		t.Fatalf("saturated status=%d", busy.Code)
	}
	close(blocked.release)
	<-done
}

func TestServerBindsAndDrains(t *testing.T) {
	holder, registry, state := handlerState(t)
	configuration := DefaultConfig()
	configuration.Listen = "127.0.0.1:0"
	handler, _ := NewHandler(configuration, holder, registry, probe.New(state), nil)
	server, err := Bind(configuration, handler)
	if err != nil {
		t.Fatal(err)
	}
	server.Start()
	response, err := http.Get("http://" + server.Address().String() + MetricsPath)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", response.StatusCode)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

type stateSource struct {
	state lifecycle.State
}

func (source *stateSource) State() lifecycle.State { return source.state }

func handlerState(t *testing.T) (*snapshot.Holder, *selfmetric.Registry, *stateSource) {
	t.Helper()
	validated := parseApplication(t, `{"schema_version":1,"families":[`+
		`{"name":"application_jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]},`+
		`{"name":"application_debug","help":"","type":"gauge","series":[{"labels":{},"value":"2"}]},`+
		`{"name":"other_metric","help":"","type":"gauge","series":[{"labels":{},"value":"3"}]}`+
		`]}`)
	holder := snapshot.NewHolder(snapshot.Zero())
	active, err := holder.Install(validated)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := selfmetric.New(buildinfo.Info{Version: "test", Revision: "test"}, selfmetric.FinalWaitImmediate, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	registry.SetActiveSnapshot(active)
	return holder, registry, &stateSource{state: lifecycle.Running}
}

func parseApplication(t *testing.T, content string) snapshot.ValidatedSnapshot {
	t.Helper()
	validated, err := snapshot.Parse([]byte(content), snapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	return validated
}

func counterValue(view selfmetric.View, name string, labels map[string]string) uint64 {
	for _, family := range view.Families {
		if family.Name != name {
			continue
		}
		for _, sample := range family.Samples {
			matched := len(sample.Labels) == len(labels)
			for _, label := range sample.Labels {
				matched = matched && labels[label.Name] == label.Value
			}
			if matched {
				return sample.Counter
			}
		}
	}
	return 0
}

type blockingResponseWriter struct {
	header  http.Header
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingResponseWriter() *blockingResponseWriter {
	return &blockingResponseWriter{header: make(http.Header), entered: make(chan struct{}), release: make(chan struct{})}
}

func (writer *blockingResponseWriter) Header() http.Header { return writer.header }
func (writer *blockingResponseWriter) WriteHeader(int)     {}
func (writer *blockingResponseWriter) Write(content []byte) (int, error) {
	writer.once.Do(func() { close(writer.entered) })
	<-writer.release
	return len(content), nil
}
