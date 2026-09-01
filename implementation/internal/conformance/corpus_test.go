package conformance

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
	"github.com/Denki77/metricshell/implementation/internal/fileingest"
	"github.com/Denki77/metricshell/implementation/internal/httpingest"
	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
	"github.com/Denki77/metricshell/implementation/internal/socketingest"
)

type corpusCase struct {
	name     string
	content  []byte
	limits   snapshot.Limits
	outcome  ingestion.Outcome
	reason   snapshot.Reason
	freeze   bool
	internal bool
}

var validComplete = []byte(`{"schema_version":1,"families":[` +
	`{"name":"requests","help":"done","type":"counter","series":[{"labels":{"method":"GET"},"value":"42.0"}]},` +
	`{"name":"temperature","help":"","type":"gauge","series":[{"labels":{"kind":"nan"},"value":"NaN"},{"labels":{"kind":"pos"},"value":"+Inf"},{"labels":{"kind":"neg"},"value":"-Inf"}]},` +
	`{"name":"latency","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"2","sum":"1.5","buckets":[{"le":"0.5","count":"1"},{"le":"+Inf","count":"2"}]}}]}` +
	`]}`)

var semanticCorpus = []corpusCase{
	{name: "complete finite and special", content: validComplete, limits: snapshot.DefaultLimits(), outcome: ingestion.Accepted},
	{name: "zero series", content: []byte(`{"schema_version":1,"families":[]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Accepted},
	{name: "malformed", content: []byte(`{}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonMalformed},
	{name: "numeric invalid", content: []byte(`{"schema_version":1,"families":[{"name":"x","help":"","type":"gauge","series":[{"labels":{},"value":"1e999"}]}]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonNumericInvalid},
	{name: "schema version", content: []byte(`{"schema_version":2,"families":[]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonSchemaVersion},
	{name: "empty payload", content: nil, limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonEmptyPayload},
	{name: "payload limit", content: []byte(`{"schema_version":1,"families":[]}`), limits: changedLimits(func(value *snapshot.Limits) { value.DecodedBytes = 1 }), outcome: ingestion.Rejected, reason: snapshot.ReasonPayloadLimit},
	{name: "series limit", content: []byte(`{"schema_version":1,"families":[{"name":"x","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`), limits: changedLimits(func(value *snapshot.Limits) { value.Series = 0 }), outcome: ingestion.Rejected, reason: snapshot.ReasonSeriesLimit},
	{name: "label limit", content: []byte(`{"schema_version":1,"families":[{"name":"x","help":"","type":"gauge","series":[{"labels":{"a":"b"},"value":"1"}]}]}`), limits: changedLimits(func(value *snapshot.Limits) { value.LabelsPerSeries = 0 }), outcome: ingestion.Rejected, reason: snapshot.ReasonLabelLimit},
	{name: "name limit", content: []byte(`{"schema_version":1,"families":[{"name":"long","help":"","type":"gauge","series":[]}]}`), limits: changedLimits(func(value *snapshot.Limits) { value.MetricNameBytes = 1 }), outcome: ingestion.Rejected, reason: snapshot.ReasonNameLimit},
	{name: "policy", content: []byte(`{"schema_version":1,"families":[{"name":"bad-name","help":"","type":"gauge","series":[]}]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonPolicy},
	{name: "duplicate series", content: []byte(`{"schema_version":1,"families":[{"name":"x","help":"","type":"gauge","series":[{"labels":{},"value":"1"},{"labels":{},"value":"2"}]}]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonDuplicateSeries},
	{name: "type conflict", content: []byte(`{"schema_version":1,"families":[{"name":"x","help":"","type":"gauge","series":[]},{"name":"x","help":"","type":"counter","series":[]}]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonTypeConflict},
	{name: "metadata conflict", content: []byte(`{"schema_version":1,"families":[{"name":"x","help":"","type":"gauge","series":[]},{"name":"x","help":"again","type":"gauge","series":[]}]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonMetadataConflict},
	{name: "histogram invalid", content: []byte(`{"schema_version":1,"families":[{"name":"x","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"2","sum":"1","buckets":[{"le":"+Inf","count":"1"}]}}]}]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonHistogramInvalid},
	{name: "reserved name", content: []byte(`{"schema_version":1,"families":[{"name":"metricshell_x","help":"","type":"gauge","series":[]}]}`), limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonReservedName},
	{name: "frozen", content: validComplete, limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonFrozen, freeze: true},
	{name: "internal", content: validComplete, limits: snapshot.DefaultLimits(), outcome: ingestion.Rejected, reason: snapshot.ReasonInternal, internal: true},
}

type result struct {
	outcome    ingestion.Outcome
	reason     snapshot.Reason
	generation uint64
	canonical  string
	metrics    selfmetric.View
	logs       []map[string]any
}

func TestSharedSemanticCorpusAcrossFileUnixAndHTTP(t *testing.T) {
	wantReasons := make(map[snapshot.Reason]bool)
	for _, reason := range snapshot.RejectionReasons {
		wantReasons[reason] = false
	}
	for _, test := range semanticCorpus {
		t.Run(test.name, func(t *testing.T) {
			if test.reason != "" {
				wantReasons[test.reason] = true
			}
			results := []result{
				driveFile(t, test),
				driveUnix(t, test),
				driveHTTP(t, test),
			}
			for index, got := range results {
				if got.outcome != test.outcome || got.reason != test.reason {
					t.Fatalf("adapter %d result=%s/%s want=%s/%s", index, got.outcome, got.reason, test.outcome, test.reason)
				}
				if got.generation != results[0].generation || got.canonical != results[0].canonical {
					t.Fatalf("adapter %d state generation=%d canonical=%q; file=%d/%q", index, got.generation, got.canonical, results[0].generation, results[0].canonical)
				}
				assertObservability(t, got, ingestion.Transports[index], test)
			}
		})
	}
	for reason, covered := range wantReasons {
		if !covered {
			t.Errorf("shared corpus does not cover rejection reason %s", reason)
		}
	}
}

func TestCompleteReplacementAndRecoveryAcrossAdapters(t *testing.T) {
	first := []byte(`{"schema_version":1,"families":[{"name":"old","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`)
	second := []byte(`{"schema_version":1,"families":[{"name":"new","help":"","type":"gauge","series":[{"labels":{},"value":"2"}]}]}`)
	for _, transport := range ingestion.Transports {
		t.Run(string(transport), func(t *testing.T) {
			environment := newEnvironment(t, snapshot.DefaultLimits(), false)
			publishThrough(t, environment, transport, first)
			invalid := publishThrough(t, environment, transport, []byte(`{}`))
			if invalid.outcome != ingestion.Rejected || environment.holder.Active().Generation() != 1 {
				t.Fatal("malformed transport candidate changed last-valid state")
			}
			accepted := publishThrough(t, environment, transport, second)
			active := environment.holder.Active()
			if accepted.outcome != ingestion.Accepted || active.Generation() != 2 || strings.Contains(string(active.Validated().Canonical()), `"old"`) || !strings.Contains(string(active.Validated().Canonical()), `"new"`) {
				t.Fatalf("replacement result=%#v state=%s", accepted, active.Validated().Canonical())
			}
		})
	}
}

func TestTransportFailuresRemainSeparateAndAdaptersRecover(t *testing.T) {
	t.Run("file partial", func(t *testing.T) {
		environment := newEnvironment(t, snapshot.DefaultLimits(), false)
		directory := t.TempDir()
		path := filepath.Join(directory, "snapshot.json")
		if err := os.WriteFile(path, []byte(`{"schema_version":`), 0o600); err != nil {
			t.Fatal(err)
		}
		reconciler, err := fileingest.New(fileingest.Config{Path: path, ReconcileInterval: time.Second, DecodedBytes: snapshot.DefaultLimits().DecodedBytes}, environment.core, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := reconciler.Reconcile(context.Background(), fileingest.Event); got.Outcome != fileingest.Invalid || environment.holder.Active().Generation() != 0 {
			t.Fatalf("invalid reconcile = %#v", got)
		}
		temporary := filepath.Join(directory, ".snapshot.tmp")
		if err := os.WriteFile(temporary, validComplete, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(temporary, path); err != nil {
			t.Fatal(err)
		}
		if got := reconciler.Reconcile(context.Background(), fileingest.Event); got.Outcome != fileingest.Accepted || environment.holder.Active().Generation() != 1 {
			t.Fatalf("recovery reconcile = %#v", got)
		}
	})

	t.Run("socket protocol", func(t *testing.T) {
		environment := newEnvironment(t, snapshot.DefaultLimits(), false)
		handler, _ := socketingest.NewProtocol(socketingest.DefaultConfig(), environment.core, nil)
		server, client := net.Pipe()
		done := make(chan error, 1)
		go func() { done <- handler.ServeConnection(context.Background(), server) }()
		reader := bufio.NewReader(client)
		if _, err := client.Write([]byte("MSP/2 SNAPSHOT_BEGIN broken 1 1\n")); err != nil {
			t.Fatal(err)
		}
		response, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if response != "MSP/1 NACK broken protocol_version\n" || environment.holder.Active().Generation() != 0 {
			t.Fatalf("protocol response = %q", response)
		}
		frames := []string{
			fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN recovered 1 %d\n", len(validComplete)),
			fmt.Sprintf("MSP/1 SNAPSHOT_PART recovered 0 %s\n", base64.RawURLEncoding.EncodeToString(validComplete)),
			"MSP/1 SNAPSHOT_COMMIT recovered\n",
		}
		for _, frame := range frames {
			if _, err := client.Write([]byte(frame)); err != nil {
				t.Fatal(err)
			}
			var err error
			response, err = reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
		}
		client.Close()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if response != "MSP/1 ACK recovered 1\n" || environment.holder.Active().Generation() != 1 {
			t.Fatalf("recovery response = %q", response)
		}
	})

	t.Run("http encoding", func(t *testing.T) {
		environment := newEnvironment(t, snapshot.DefaultLimits(), false)
		handler, _ := httpingest.NewHandler(httpingest.DefaultConfig(), environment.core)
		invalidRequest := httptest.NewRequest(http.MethodPost, httpingest.Path, strings.NewReader("not-gzip"))
		invalidRequest.Header.Set("Content-Type", "application/json")
		invalidRequest.Header.Set("Content-Encoding", "gzip")
		invalidResponse := httptest.NewRecorder()
		handler.ServeHTTP(invalidResponse, invalidRequest)
		if invalidResponse.Code != http.StatusBadRequest || environment.holder.Active().Generation() != 0 {
			t.Fatalf("invalid response = %d/%s", invalidResponse.Code, invalidResponse.Body.String())
		}
		var encoded bytes.Buffer
		compressor := gzip.NewWriter(&encoded)
		if _, err := compressor.Write(validComplete); err != nil {
			t.Fatal(err)
		}
		if err := compressor.Close(); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, httpingest.Path, &encoded)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Content-Encoding", "gzip")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || environment.holder.Active().Generation() != 1 {
			t.Fatalf("recovery response = %d/%s", response.Code, response.Body.String())
		}
	})
}

func TestAdmissionOutcomesShareMetricsAndBoundedLogs(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	registry, _ := selfmetric.New(buildinfo.Info{Version: "test", Revision: "test"}, selfmetric.FinalWaitImmediate, func() time.Time { return now })
	metricsObserver, _ := ingestion.NewMetricsObserver(registry, func() time.Time { return now })
	logs := &bytes.Buffer{}
	diagnosticObserver, _ := ingestion.NewDiagnosticObserver(diagnostic.New(logs, func() time.Time { return now }), func() string { return "running" })
	entered := make(chan struct{})
	release := make(chan struct{})
	parser := func(content []byte, limits snapshot.Limits) (snapshot.ValidatedSnapshot, error) {
		close(entered)
		<-release
		return snapshot.Parse(content, limits)
	}
	core, err := ingestion.NewWithParser(snapshot.NewHolder(snapshot.Zero()), snapshot.DefaultLimits(), 1, 0,
		ingestion.MultiObserver{metricsObserver, diagnosticObserver}, parser, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan ingestion.Result, 1)
	go func() { done <- core.Publish(context.Background(), ingestion.HTTP, validComplete) }()
	<-entered
	if got := core.Publish(context.Background(), ingestion.Unix, validComplete); got.Outcome != ingestion.Busy {
		t.Fatalf("busy result = %#v", got)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := core.Publish(cancelled, ingestion.File, validComplete); got.Outcome != ingestion.Timeout {
		t.Fatalf("timeout result = %#v", got)
	}
	close(release)
	if got := <-done; got.Outcome != ingestion.Accepted {
		t.Fatalf("accepted result = %#v", got)
	}
	view := registry.View()
	if counter(view, selfmetric.SnapshotPublicationsTotal, map[string]string{"transport": "unix", "outcome": "busy"}) != 1 ||
		counter(view, selfmetric.SnapshotPublicationsTotal, map[string]string{"transport": "file", "outcome": "timeout"}) != 1 {
		t.Fatal("declined admission outcomes were not observed")
	}
	records := decodeLogs(t, logs.Bytes())
	if len(records) != 2 || records[0]["event"] != "ingestion.overloaded" || records[0]["outcome"] != "busy" || records[1]["event"] != "snapshot.accepted" {
		t.Fatalf("admission logs = %#v", records)
	}
}

type environment struct {
	holder  *snapshot.Holder
	core    *ingestion.Core
	metrics *selfmetric.Registry
	logs    *bytes.Buffer
}

func newEnvironment(t *testing.T, limits snapshot.Limits, frozen bool) *environment {
	return newEnvironmentWithParser(t, limits, frozen, nil)
}

func newEnvironmentWithParser(t *testing.T, limits snapshot.Limits, frozen bool, parser ingestion.Parser) *environment {
	t.Helper()
	holder := snapshot.NewHolder(snapshot.Zero())
	if frozen {
		holder.Freeze()
	}
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	registry, err := selfmetric.New(buildinfo.Info{Version: "test", Revision: "test"}, selfmetric.FinalWaitImmediate, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	metricsObserver, _ := ingestion.NewMetricsObserver(registry, func() time.Time { return now })
	logs := &bytes.Buffer{}
	logger := diagnostic.New(logs, func() time.Time { return now })
	diagnosticObserver, _ := ingestion.NewDiagnosticObserver(logger, func() string { return "running" })
	var core *ingestion.Core
	if parser == nil {
		core, err = ingestion.New(holder, limits, 4, 0, ingestion.MultiObserver{metricsObserver, diagnosticObserver})
	} else {
		core, err = ingestion.NewWithParser(holder, limits, 4, 0, ingestion.MultiObserver{metricsObserver, diagnosticObserver}, parser, func() time.Time { return now })
	}
	if err != nil {
		t.Fatal(err)
	}
	return &environment{holder: holder, core: core, metrics: registry, logs: logs}
}

func driveFile(t *testing.T, test corpusCase) result {
	t.Helper()
	environment := corpusEnvironment(t, test)
	return publishThrough(t, environment, ingestion.File, test.content)
}

func driveUnix(t *testing.T, test corpusCase) result {
	t.Helper()
	environment := corpusEnvironment(t, test)
	return publishThrough(t, environment, ingestion.Unix, test.content)
}

func driveHTTP(t *testing.T, test corpusCase) result {
	t.Helper()
	environment := corpusEnvironment(t, test)
	return publishThrough(t, environment, ingestion.HTTP, test.content)
}

func corpusEnvironment(t *testing.T, test corpusCase) *environment {
	t.Helper()
	if test.internal {
		return newEnvironmentWithParser(t, test.limits, test.freeze, func([]byte, snapshot.Limits) (snapshot.ValidatedSnapshot, error) {
			return snapshot.ValidatedSnapshot{}, snapshot.Reject(snapshot.ReasonInternal)
		})
	}
	return newEnvironment(t, test.limits, test.freeze)
}

func publishThrough(t *testing.T, environment *environment, transport ingestion.Transport, content []byte) result {
	t.Helper()
	var outcome ingestion.Outcome
	var reason snapshot.Reason
	switch transport {
	case ingestion.File:
		path := filepath.Join(t.TempDir(), "snapshot.json")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		reconciler, err := fileingest.New(fileingest.Config{Path: path, ReconcileInterval: time.Second, DecodedBytes: snapshot.DefaultLimits().DecodedBytes}, environment.core, nil)
		if err != nil {
			t.Fatal(err)
		}
		publication := reconciler.Reconcile(context.Background(), fileingest.Event).Publication
		outcome, reason = publication.Outcome, publication.Reason
	case ingestion.Unix:
		// MSP/1 cannot encode a zero-byte transaction; the common candidate boundary is exercised directly for
		// empty_payload while framing rejection remains covered by socket transport tests.
		if len(content) == 0 {
			publication := environment.core.Publish(context.Background(), ingestion.Unix, content)
			outcome, reason = publication.Outcome, publication.Reason
			break
		}
		configuration := socketingest.DefaultConfig()
		handler, err := socketingest.NewProtocol(configuration, environment.core, nil)
		if err != nil {
			t.Fatal(err)
		}
		server, client := net.Pipe()
		done := make(chan error, 1)
		go func() { done <- handler.ServeConnection(context.Background(), server) }()
		reader := bufio.NewReader(client)
		frames := []string{
			fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN corpus 1 %d\n", len(content)),
			fmt.Sprintf("MSP/1 SNAPSHOT_PART corpus 0 %s\n", base64.RawURLEncoding.EncodeToString(content)),
			"MSP/1 SNAPSHOT_COMMIT corpus\n",
		}
		var response string
		for _, frame := range frames {
			if _, err := client.Write([]byte(frame)); err != nil {
				t.Fatal(err)
			}
			response, err = reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
		}
		client.Close()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(response)
		if len(fields) != 4 {
			t.Fatalf("socket response = %q", response)
		}
		if fields[1] == "ACK" {
			outcome = ingestion.Accepted
		} else {
			outcome, reason = ingestion.Rejected, snapshot.Reason(fields[3])
		}
	case ingestion.HTTP:
		handler, err := httpingest.NewHandler(httpingest.DefaultConfig(), environment.core)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, httpingest.Path, bytes.NewReader(content))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		var body struct {
			Status string          `json:"status"`
			Code   snapshot.Reason `json:"code"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Status == "ack" {
			outcome = ingestion.Accepted
		} else {
			outcome, reason = ingestion.Rejected, body.Code
		}
	}
	active := environment.holder.Active()
	return result{outcome: outcome, reason: reason, generation: active.Generation(), canonical: string(active.Validated().Canonical()), metrics: environment.metrics.View(), logs: decodeLogs(t, environment.logs.Bytes())}
}

func assertObservability(t *testing.T, got result, transport ingestion.Transport, test corpusCase) {
	t.Helper()
	if counter(got.metrics, selfmetric.SnapshotPublicationsTotal, map[string]string{"transport": string(transport), "outcome": string(test.outcome)}) != 1 {
		t.Fatalf("%s publication metric not incremented for %s", transport, test.outcome)
	}
	event := "snapshot.accepted"
	if test.outcome == ingestion.Rejected {
		event = "snapshot.rejected"
		if counter(got.metrics, selfmetric.SnapshotRejectionsTotal, map[string]string{"transport": string(transport), "reason": string(test.reason)}) != 1 {
			t.Fatalf("%s rejection metric not incremented for %s", transport, test.reason)
		}
	}
	if len(got.logs) != 1 || got.logs[0]["event"] != event || got.logs[0]["transport"] != string(transport) {
		t.Fatalf("%s logs = %#v", transport, got.logs)
	}
	if test.outcome == ingestion.Accepted {
		if got.logs[0]["snapshot_generation"] != float64(got.generation) || got.logs[0]["snapshot_bytes"] == nil || got.logs[0]["series"] == nil {
			t.Fatalf("accepted log fields = %#v", got.logs[0])
		}
	} else if got.logs[0]["reason"] != string(test.reason) {
		t.Fatalf("rejected log fields = %#v", got.logs[0])
	}
}

func counter(view selfmetric.View, name string, labels map[string]string) uint64 {
	for _, family := range view.Families {
		if family.Name != name {
			continue
		}
		for _, sample := range family.Samples {
			if sameLabels(sample.Labels, labels) {
				return sample.Counter
			}
		}
	}
	return 0
}

func sameLabels(labels []selfmetric.Label, want map[string]string) bool {
	if len(labels) != len(want) {
		return false
	}
	for _, label := range labels {
		if want[label.Name] != label.Value {
			return false
		}
	}
	return true
}

func decodeLogs(t *testing.T, content []byte) []map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(content))
	var records []map[string]any
	for {
		var record map[string]any
		if err := decoder.Decode(&record); errors.Is(err, io.EOF) {
			return records
		} else if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
}

func changedLimits(change func(*snapshot.Limits)) snapshot.Limits {
	limits := snapshot.DefaultLimits()
	change(&limits)
	return limits
}
