package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
	"github.com/Denki77/metricshell/implementation/internal/exposition"
	"github.com/Denki77/metricshell/implementation/internal/finalwait"
	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
)

func TestRunVersion(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	identity := buildinfo.Info{Version: "1.2.3", Revision: "0123456"}
	code := Run([]string{"--version"}, nil, &stdout, &stderr, identity, time.Now)

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if got, want := stdout.String(), identity.String()+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunFailsBeforeWorkloadWhenExpositionBindFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--exposition-listen=" + listener.Addr().String(), "--", "/must-not-start"}, nil, &stdout, &stderr, buildinfo.Info{}, time.Now)
	if code != 72 {
		t.Fatalf("Run() code = %d, want 72", code)
	}
	if stdout.Len() != 0 || bytes.Contains(stderr.Bytes(), []byte(`"event":"workload.`)) {
		t.Fatalf("workload was observed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte(`"event":"endpoint.bind_failed"`)) {
		t.Fatalf("endpoint.bind_failed missing: %s", stderr.String())
	}
}

func TestRunTraversesConfiguredFinalWait(t *testing.T) {
	var stdout bytes.Buffer
	var stderr synchronizedBuffer
	code := Run([]string{
		"--exposition-listen=127.0.0.1:0", "--final-wait-mode=duration", "--final-wait-duration=0",
		"--", "/bin/true",
	}, nil, &stdout, &stderr, buildinfo.Info{}, time.Now)
	if code != 0 {
		t.Fatalf("Run() code = %d, stderr=%s", code, stderr.String())
	}
	content := stderr.Bytes()
	finalizing := bytes.Index(content, []byte(`"state":"finalizing"`))
	finalWait := bytes.Index(content, []byte(`"state":"final_wait"`))
	terminated := bytes.LastIndex(content, []byte(`"state":"terminated"`))
	if finalizing < 0 || finalWait <= finalizing || terminated <= finalWait {
		t.Fatalf("final-wait state flow missing or unordered: %s", stderr.String())
	}
	records := parseDiagnosticRecords(t, content)
	started := findRecord(records, "final_wait.started")
	completed := findRecord(records, "final_wait.completed")
	if started == nil || started["mode"] != "duration" || started["deadline"] == "" {
		t.Fatalf("final_wait.started missing or incomplete: %s", stderr.String())
	}
	if completed == nil || completed["reason"] != "duration_elapsed" {
		t.Fatalf("final_wait.completed missing or incomplete: %s", stderr.String())
	}
}

func TestFinalResponseObservabilityUpdatesMetricsAndLogs(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC) }
	metrics, err := selfmetric.New(buildinfo.Info{Version: "test", Revision: "test"}, selfmetric.FinalWaitScrapes, clock)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	logger := diagnostic.New(&output, clock)

	observeFinalResponse(metrics, logger, exposition.FinalResponse{
		RequestID: 7, Generation: 42, Outcome: exposition.FinalCompleted, Completed: 1, Counted: true,
	})
	observeFinalResponse(metrics, logger, exposition.FinalResponse{
		RequestID: 8, Generation: 42, Outcome: exposition.FinalCancelled, Completed: 1, Counted: false,
	})

	view := metrics.View()
	if got := cliCounterValue(view, selfmetric.FinalScrapeAttemptsTotal, map[string]string{"outcome": "completed"}); got != 1 {
		t.Fatalf("completed attempts = %d, want 1", got)
	}
	if got := cliCounterValue(view, selfmetric.FinalScrapeAttemptsTotal, map[string]string{"outcome": "cancelled"}); got != 1 {
		t.Fatalf("cancelled attempts = %d, want 1", got)
	}
	if got := cliGaugeValue(view, selfmetric.FinalWaitCompletedScrapes, nil); got != 1 {
		t.Fatalf("completed gauge = %v, want 1", got)
	}

	records := parseDiagnosticRecords(t, output.Bytes())
	counted := findRecord(records, "final_scrape.counted")
	notCounted := findRecord(records, "final_scrape.not_counted")
	if counted == nil || counted["request_id"] != float64(7) || counted["snapshot_generation"] != float64(42) {
		t.Fatalf("counted event = %#v", counted)
	}
	if notCounted == nil || notCounted["request_id"] != float64(8) || notCounted["outcome"] != "cancelled" {
		t.Fatalf("not-counted event = %#v", notCounted)
	}
}

func TestFinalWaitObservabilityLifecycleMetrics(t *testing.T) {
	startedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	metrics, err := selfmetric.New(buildinfo.Info{}, selfmetric.FinalWaitScrapes, func() time.Time { return startedAt })
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	logger := diagnostic.New(&output, func() time.Time { return startedAt })
	configuration := finalwait.Defaults()
	configuration.Timeout = 5 * time.Second

	if err := startFinalWaitObservability(metrics, logger, configuration, lifecycle.FinalWait, startedAt); err != nil {
		t.Fatal(err)
	}
	result := finalwait.Result{Reason: finalwait.ReasonRequiredScrapes, Generation: 3, Completed: 1}
	if err := completeFinalWaitObservability(metrics, logger, result, lifecycle.FinalWait, 25*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	view := metrics.View()
	if cliGaugeValue(view, selfmetric.FinalWaitActive, nil) != 0 || cliGaugeValue(view, selfmetric.FinalWaitDeadline, nil) != 0 {
		t.Fatal("final wait terminal gauges were not reset")
	}
	if cliCounterValue(view, selfmetric.FinalWaitCompletionsTotal, map[string]string{"reason": "required_scrapes"}) != 1 {
		t.Fatal("required_scrapes completion counter missing")
	}
	records := parseDiagnosticRecords(t, output.Bytes())
	if findRecord(records, "final_wait.started") == nil || findRecord(records, "final_wait.completed") == nil {
		t.Fatalf("final-wait events missing: %s", output.String())
	}
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *synchronizedBuffer) Write(content []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(content)
}

func (buffer *synchronizedBuffer) Bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return bytes.Clone(buffer.buffer.Bytes())
}

func (buffer *synchronizedBuffer) String() string { return string(buffer.Bytes()) }

func parseDiagnosticRecords(t *testing.T, content []byte) []map[string]any {
	t.Helper()
	var records []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("stderr is not JSON Lines: %v", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}

func findRecord(records []map[string]any, event string) map[string]any {
	for _, record := range records {
		if record["event"] == event {
			return record
		}
	}
	return nil
}

func cliCounterValue(view selfmetric.View, name string, labels map[string]string) uint64 {
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

func cliGaugeValue(view selfmetric.View, name string, labels map[string]string) float64 {
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
				return sample.Gauge
			}
		}
	}
	return 0
}

func TestRunHelp(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, nil, &stdout, &stderr, buildinfo.Info{}, time.Now)
	if code != 0 || stdout.String() != usage || stderr.Len() != 0 {
		t.Fatalf("Run(--help) = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestRunRejectsInvalidStartupConfiguration(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	now := time.Date(2026, 8, 11, 10, 30, 0, 123, time.FixedZone("test", 3*60*60))
	code := Run(nil, nil, &stdout, &stderr, buildinfo.Info{}, func() time.Time { return now })

	if code != 64 {
		t.Fatalf("Run() code = %d, want 64", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}

	var records []map[string]any
	scanner := bufio.NewScanner(&stderr)
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("stderr is not JSON Lines: %v", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(records) != 5 {
		t.Fatalf("records = %d, want 5: %s", len(records), stderr.String())
	}
	var record map[string]any
	for _, candidate := range records {
		if candidate["event"] == "configuration.rejected" {
			record = candidate
			break
		}
	}
	if record == nil {
		t.Fatalf("configuration.rejected missing: %s", stderr.String())
	}
	want := map[string]any{
		"timestamp":      "2026-08-11T07:30:00.000000123Z",
		"schema_version": "1",
		"level":          "error",
		"event":          "configuration.rejected",
		"component":      "runtime",
		"state":          "initializing",
		"reason":         "configuration",
		"error_code":     "CONFIG_INVALID",
	}
	for field, expected := range want {
		if got := record[field]; got != expected {
			t.Errorf("field %s = %#v, want %#v", field, got, expected)
		}
	}
	if record["sequence"] != float64(3) {
		t.Errorf("sequence = %#v, want 3", record["sequence"])
	}
	if record["runtime_id"] == "" {
		t.Error("runtime_id is empty")
	}
}

func TestRunRejectsOvercommittedShutdownBeforeWorkloadStart(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	code := Run([]string{
		"--shutdown-total-grace=1s",
		"--workload-shutdown-timeout=751ms",
		"--shutdown-reserve=250ms",
		"--", "/must-not-start",
	}, nil, &stdout, &stderr, buildinfo.Info{}, func() time.Time { return now })

	if code != 64 {
		t.Fatalf("Run() code = %d, want 64", code)
	}
	if stdout.Len() != 0 || bytes.Contains(stderr.Bytes(), []byte(`"event":"workload.`)) {
		t.Fatalf("workload was observed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte(`"event":"configuration.rejected"`)) {
		t.Fatalf("configuration rejection missing: %s", stderr.String())
	}
}

func TestNoFileAvailabilityBoundary(t *testing.T) {
	t.Parallel()

	if nofileAvailable(63, 64) {
		t.Fatal("nofileAvailable accepted required-1")
	}
	if !nofileAvailable(64, 64) {
		t.Fatal("nofileAvailable rejected exact required limit")
	}
}
