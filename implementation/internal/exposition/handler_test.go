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
	"github.com/Denki77/metricshell/implementation/internal/finalwait"
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
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("repeated shutdown: %v", err)
	}
}

func TestFinalResponseCountsOnlyCompleteFrozenGeneration(t *testing.T) {
	holder, registry, state := handlerState(t)
	configuration := DefaultConfig()
	configuration.Listen = "127.0.0.1:0"
	handler, _ := NewHandler(configuration, holder, registry, probe.New(state), nil)

	preFinal, err := finalwait.New(finalwait.Defaults(), holder.Active().Generation())
	if err != nil {
		t.Fatal(err)
	}
	preFinalResponses := make(chan FinalResponse, 1)
	handler.SetFinalWait(preFinal, func(response FinalResponse) { preFinalResponses <- response })
	preFinalResponse := httptest.NewRecorder()
	handler.ServeHTTP(preFinalResponse, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if preFinalResponse.Code != http.StatusOK {
		t.Fatalf("pre-final response status=%d", preFinalResponse.Code)
	}
	select {
	case got := <-preFinalResponses:
		t.Fatalf("pre-final response was tracked: %+v", got)
	default:
	}

	waiter, cancel, result, responses := startFinalWait(t, handler, holder.Active().Generation(), 1)
	defer cancel()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("complete response status=%d", response.Code)
	}
	if got := <-result; got.Reason != finalwait.ReasonRequiredScrapes || got.Completed != 1 {
		t.Fatalf("wait result=%+v", got)
	}
	if got := <-responses; got.Outcome != FinalCompleted || !got.Counted || got.Completed != 1 {
		t.Fatalf("final response=%+v", got)
	}
	if waiter.State().Completed != 1 {
		t.Fatalf("completed=%d", waiter.State().Completed)
	}
	rejected := httptest.NewRecorder()
	handler.ServeHTTP(rejected, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if rejected.Code != http.StatusServiceUnavailable {
		t.Fatalf("post-threshold status=%d", rejected.Code)
	}

	wrongHandler, _ := NewHandler(configuration, holder, registry, probe.New(state), nil)
	wrong, wrongCancel, wrongResult, wrongResponses := startFinalWait(t, wrongHandler, holder.Active().Generation()+1, 1)
	wrongResponse := httptest.NewRecorder()
	wrongHandler.ServeHTTP(wrongResponse, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if got := <-wrongResponses; got.Outcome != FinalIneligible || got.Counted || got.Completed != 0 {
		t.Fatalf("wrong-generation response=%+v", got)
	}
	wrongCancel()
	if got := <-wrongResult; got.Reason != finalwait.ReasonExternalTermination || wrong.State().Completed != 0 {
		t.Fatalf("wrong-generation result=%+v state=%+v", got, wrong.State())
	}
}

func TestFinalResponseClassifiesPartialCancelledAndExcludedRequests(t *testing.T) {
	holder, registry, state := handlerState(t)
	configuration := DefaultConfig()
	configuration.Listen = "127.0.0.1:0"
	handler, _ := NewHandler(configuration, holder, registry, probe.New(state), nil)
	waiter, cancel, result, responses := startFinalWait(t, handler, holder.Active().Generation(), 2)

	short := &shortResponseWriter{header: make(http.Header)}
	handler.ServeHTTP(short, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if got := <-responses; got.Outcome != FinalWriteError || got.Counted {
		t.Fatalf("short response=%+v", got)
	}

	zero := &zeroResponseWriter{header: make(http.Header)}
	handler.ServeHTTP(zero, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if got := <-responses; got.Outcome != FinalWriteError || got.Counted {
		t.Fatalf("zero-byte response=%+v", got)
	}

	cancelledContext, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()
	cancelled := httptest.NewRequest(http.MethodGet, MetricsPath, nil).WithContext(cancelledContext)
	handler.ServeHTTP(httptest.NewRecorder(), cancelled)
	if got := <-responses; got.Outcome != FinalCancelled || got.Counted {
		t.Fatalf("cancelled response=%+v", got)
	}

	for _, path := range []string{probe.HealthPath, probe.ReadinessPath, DebugPath} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	select {
	case got := <-responses:
		t.Fatalf("probe/debug produced final response=%+v", got)
	default:
	}
	if waiter.State().Completed != 0 {
		t.Fatalf("failed/excluded requests completed=%d", waiter.State().Completed)
	}
	cancel()
	<-result
}

func TestFinalThresholdClosesAdmissionAndDrainsOnlyAcceptedHandlers(t *testing.T) {
	holder, registry, state := handlerState(t)
	configuration := DefaultConfig()
	configuration.Listen = "127.0.0.1:0"
	handler, _ := NewHandler(configuration, holder, registry, probe.New(state), nil)
	waiter, cancel, result, _ := startFinalWait(t, handler, holder.Active().Generation(), 1)
	defer cancel()

	blocked := newBlockingResponseWriter()
	blockedDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
		close(blockedDone)
	}()
	<-blocked.entered
	complete := httptest.NewRecorder()
	handler.ServeHTTP(complete, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if got := <-result; got.Reason != finalwait.ReasonRequiredScrapes || waiter.State().Completed != 1 {
		t.Fatalf("threshold result=%+v state=%+v", got, waiter.State())
	}

	late := httptest.NewRecorder()
	handler.ServeHTTP(late, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if late.Code != http.StatusServiceUnavailable {
		t.Fatalf("late status=%d", late.Code)
	}
	expired, stop := context.WithCancel(context.Background())
	stop()
	if handler.Drain(expired) {
		t.Fatal("zero-grace drain completed with an accepted handler blocked")
	}
	close(blocked.release)
	<-blockedDone
	if !handler.Drain(context.Background()) || waiter.State().Completed != 1 {
		t.Fatalf("accepted handler did not drain or threshold changed: %+v", waiter.State())
	}
}

func startFinalWait(t *testing.T, handler *Handler, generation uint64, required int) (*finalwait.Waiter, context.CancelFunc, <-chan finalwait.Result, <-chan FinalResponse) {
	t.Helper()
	configuration := finalwait.Defaults()
	configuration.RequiredScrapes = required
	waiter, err := finalwait.New(configuration, generation)
	if err != nil {
		t.Fatal(err)
	}
	responses := make(chan FinalResponse, 16)
	handler.SetFinalWait(waiter, func(response FinalResponse) { responses <- response })
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan finalwait.Result, 1)
	go func() {
		got, waitErr := waiter.Wait(ctx)
		if waitErr != nil {
			t.Errorf("final wait: %v", waitErr)
		}
		result <- got
	}()
	deadline := time.After(time.Second)
	for !waiter.State().Active {
		select {
		case <-deadline:
			t.Fatal("final wait did not become active")
		default:
		}
	}
	return waiter, cancel, result, responses
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

type zeroResponseWriter struct {
	header http.Header
}

func (writer *zeroResponseWriter) Header() http.Header { return writer.header }
func (writer *zeroResponseWriter) WriteHeader(int)     {}
func (writer *zeroResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}
