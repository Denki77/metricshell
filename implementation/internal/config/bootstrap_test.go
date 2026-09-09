package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/finalwait"
)

var testNow = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

func TestParseWorkloadAndShutdownDefaults(t *testing.T) {
	t.Parallel()

	want := []string{"program", "space value", "", "Привет", "--version", "--"}
	configuration, err := Parse(append([]string{"--"}, want...), testNow, nil)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !reflect.DeepEqual(configuration.Workload, want) {
		t.Fatalf("Workload = %#v, want %#v", configuration.Workload, want)
	}
	if configuration.Shutdown.TotalGrace != 30*time.Second || configuration.Shutdown.WorkloadTimeout != 28*time.Second || configuration.Shutdown.Reserve != 2*time.Second {
		t.Fatalf("shutdown defaults = %+v", configuration.Shutdown)
	}
	if configuration.FinalWait != finalwait.Defaults() {
		t.Fatalf("final-wait defaults = %+v", configuration.FinalWait)
	}
	if configuration.IngestionTransport != "unix" || configuration.UnixSocketPath != "/run/metricshell/ingest.sock" {
		t.Fatalf("ingestion defaults = %+v", configuration)
	}
}

func TestParseShutdownPrecedence(t *testing.T) {
	t.Parallel()

	environment := map[string]string{
		"METRICSHELL_SHUTDOWN_TOTAL_GRACE":      "10s",
		"METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT": "8s",
		"METRICSHELL_SHUTDOWN_RESERVE":          "2s",
		"METRICSHELL_SHUTDOWN_DEADLINE":         "2026-08-24T12:01:00Z",
	}
	lookup := func(name string) (string, bool) { value, ok := environment[name]; return value, ok }
	configuration, err := Parse([]string{"--shutdown-total-grace=5s", "--workload-shutdown-timeout", "4s", "--shutdown-reserve=1s", "--", "program"}, testNow, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Shutdown.TotalGrace != 5*time.Second || configuration.Shutdown.WorkloadTimeout != 4*time.Second || configuration.Shutdown.Reserve != time.Second {
		t.Fatalf("shutdown = %+v", configuration.Shutdown)
	}
	if configuration.Shutdown.Deadline.Format(time.RFC3339) != environment["METRICSHELL_SHUTDOWN_DEADLINE"] {
		t.Fatalf("deadline = %s", configuration.Shutdown.Deadline)
	}
}

func TestParseRejectsInvalidCommandLine(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{nil, {"--"}, {"program"}, {"--unknown", "--", "program"}, {"--shutdown-reserve", "--", "program"}} {
		if _, err := Parse(args, testNow, nil); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", args)
		}
	}
}

func TestParseRejectsSharedMemoryConfigurationSurface(t *testing.T) {
	t.Parallel()

	for _, option := range []string{"--mmap", "--mmap-path=/run/metrics", "--shared-memory", "--shm-path=/dev/shm/metrics", "--ingestion-transport=mmap"} {
		if _, err := Parse([]string{option, "--", "program"}, testNow, nil); err == nil {
			t.Errorf("Parse(%q) accepted unsupported shared-memory option", option)
		}
	}
	for _, name := range append(unsupportedSharedMemoryEnvironment[:], "METRICSHELL_INGESTION_TRANSPORT") {
		lookup := func(candidate string) (string, bool) {
			if candidate != name {
				return "", false
			}
			if name == "METRICSHELL_INGESTION_TRANSPORT" {
				return "mmap", true
			}
			return "configured", true
		}
		if _, err := Parse([]string{"--", "program"}, testNow, lookup); err == nil {
			t.Errorf("Parse accepted unsupported environment %s", name)
		}
	}
}

func TestDurationGrammarAndBudgetValidation(t *testing.T) {
	t.Parallel()

	valid := []string{"0", "1ns", "2us", "3ms", "4s", "5m", "1h"}
	for _, value := range valid {
		if _, err := parseDuration(value); err != nil {
			t.Errorf("parseDuration(%q) = %v", value, err)
		}
	}
	invalid := []string{"", "01s", "1.5s", "1s500ms", "-1s", "+1s", "1S", "1", "18446744073709551615h"}
	for _, value := range invalid {
		if _, err := parseDuration(value); err == nil {
			t.Errorf("parseDuration(%q) succeeded", value)
		}
	}

	for _, args := range [][]string{
		{"--shutdown-total-grace=1s", "--workload-shutdown-timeout=751ms", "--shutdown-reserve=250ms", "--", "program"},
		{"--shutdown-deadline=2026-08-24T11:59:59Z", "--", "program"},
		{"--shutdown-reserve=", "--", "program"},
	} {
		if _, err := Parse(args, testNow, nil); err == nil {
			t.Errorf("Parse(%q) succeeded", args)
		}
	}
}

func TestParseExpositionDefaultsPrecedenceAndSelectors(t *testing.T) {
	t.Parallel()

	environment := map[string]string{
		"METRICSHELL_EXPOSITION_LISTEN":        "127.0.0.1:19090",
		"METRICSHELL_MAX_RESPONSE_BYTES":       "1MiB",
		"METRICSHELL_MAX_CONCURRENT_SCRAPES":   "4",
		"METRICSHELL_EXPOSITION_WRITE_TIMEOUT": "5s",
		"METRICSHELL_METRICS_INCLUDE":          "prefix:environment_,name:ignored",
		"METRICSHELL_METRICS_EXCLUDE":          "prefix:debug_",
	}
	lookup := func(name string) (string, bool) { value, ok := environment[name]; return value, ok }
	configuration, err := Parse([]string{
		"--exposition-listen=127.0.0.1:0", "--max-response-bytes", "2MiB", "--max-concurrent-scrapes=8",
		"--exposition-write-timeout=10s", "--metrics-include=name:application_jobs", "--metrics-include=prefix:worker_",
		"--", "program",
	}, testNow, lookup)
	if err != nil {
		t.Fatal(err)
	}
	got := configuration.Exposition
	if got.Listen != "127.0.0.1:0" || got.ResponseBytes != 2<<20 || got.Concurrent != 8 || got.WriteTimeout != 10*time.Second {
		t.Fatalf("exposition = %+v", got)
	}
	wantInclude := []string{"name:application_jobs", "prefix:worker_"}
	if !reflect.DeepEqual(got.Include, wantInclude) || !reflect.DeepEqual(got.Exclude, []string{"prefix:debug_"}) {
		t.Fatalf("selectors = %v/%v", got.Include, got.Exclude)
	}
}

func TestParseRejectsInvalidExpositionConfiguration(t *testing.T) {
	t.Parallel()

	for _, option := range []string{
		"--exposition-listen=:9090", "--max-response-bytes=63KiB", "--max-response-bytes=65MiB",
		"--max-concurrent-scrapes=0", "--max-concurrent-scrapes=129", "--exposition-write-timeout=999ms",
		"--metrics-include=regex:jobs", "--metrics-exclude=prefix:*",
	} {
		if _, err := Parse([]string{option, "--", "program"}, testNow, nil); err == nil {
			t.Errorf("Parse accepted %s", option)
		}
	}
}

func TestParseFinalWaitPrecedenceAndValidation(t *testing.T) {
	t.Parallel()

	environment := map[string]string{
		"METRICSHELL_FINAL_WAIT_MODE":             "duration",
		"METRICSHELL_FINAL_WAIT_DURATION":         "45s",
		"METRICSHELL_FINAL_WAIT_TIMEOUT":          "50s",
		"METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES": "2",
		"METRICSHELL_FINAL_WAIT_COMPLETION_GRACE": "1s",
	}
	lookup := func(name string) (string, bool) { value, ok := environment[name]; return value, ok }
	configuration, err := Parse([]string{
		"--final-wait-mode=scrapes", "--final-wait-timeout=10s", "--final-wait-required-scrapes", "3",
		"--final-wait-completion-grace=250ms", "--", "program",
	}, testNow, lookup)
	if err != nil {
		t.Fatal(err)
	}
	want := finalwait.Config{Mode: finalwait.Scrapes, Duration: 45 * time.Second, Timeout: 10 * time.Second, RequiredScrapes: 3, CompletionGrace: 250 * time.Millisecond}
	if configuration.FinalWait != want {
		t.Fatalf("final wait = %+v, want %+v", configuration.FinalWait, want)
	}

	for _, option := range []string{
		"--final-wait-mode=auto", "--final-wait-duration=1h1s", "--final-wait-timeout=0",
		"--final-wait-required-scrapes=0", "--final-wait-required-scrapes=17", "--final-wait-completion-grace=5001ms",
	} {
		if _, err := Parse([]string{option, "--", "program"}, testNow, nil); err == nil {
			t.Errorf("Parse accepted %s", option)
		}
	}
}

func TestParseIngestionDefaultsPrecedenceAndValidation(t *testing.T) {
	t.Parallel()

	environment := map[string]string{
		"METRICSHELL_INGESTION_TRANSPORT":    "unix",
		"METRICSHELL_UNIX_SOCKET_PATH":       "/tmp/env.sock",
		"METRICSHELL_MAX_PENDING_INGESTIONS": "0",
		"METRICSHELL_MAX_LABELS_PER_SERIES":  "0",
		"METRICSHELL_MAX_HELP_BYTES":         "0",
	}
	lookup := func(name string) (string, bool) { value, ok := environment[name]; return value, ok }
	configuration, err := Parse([]string{
		"--unix-socket-path=/tmp/cli.sock",
		"--max-concurrent-ingestions=2",
		"--max-snapshot-bytes=64KiB",
		"--max-decoded-input-bytes=128KiB",
		"--max-series=1",
		"--socket-max-frame-bytes=1KiB",
		"--socket-max-parts=1024",
		"--socket-max-transactions=1",
		"--socket-max-connections=1",
		"--socket-transaction-timeout=100ms",
		"--socket-read-timeout=100ms",
		"--socket-write-timeout=100ms",
		"--", "program",
	}, testNow, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.IngestionTransport != "unix" || configuration.UnixSocketPath != "/tmp/cli.sock" {
		t.Fatalf("transport endpoints = %+v", configuration)
	}
	if configuration.PendingIngestion != 0 || configuration.Limits.LabelsPerSeries != 0 || configuration.Limits.HelpBytes != 0 {
		t.Fatalf("zero-valued bounds = %+v", configuration)
	}
	if configuration.Limits.SnapshotBytes != 64<<10 || configuration.Limits.DecodedBytes != 128<<10 ||
		configuration.HTTPIngestion.DecodedBytes != 128<<10 || configuration.Socket.SnapshotBytes != 64<<10 {
		t.Fatalf("limits propagation = %+v", configuration)
	}

	for _, option := range []string{
		"--ingestion-transport=shm", "--file-reconcile-interval=99ms", "--max-snapshot-bytes=63KiB",
		"--max-decoded-input-bytes=63KiB", "--max-concurrent-ingestions=0", "--max-pending-ingestions=65",
		"--socket-max-frame-bytes=512B", "--socket-max-connections=65", "--http-ingestion-max-wire-bytes=63KiB",
		"--http-idle-timeout=999ms",
	} {
		if _, err := Parse([]string{option, "--", "program"}, testNow, nil); err == nil {
			t.Errorf("Parse accepted %s", option)
		}
	}
}

func TestParseRejectsInactiveTransportOptions(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"--ingestion-transport=http", "--unix-socket-path=/tmp/cli.sock", "--", "program"},
		{"--ingestion-transport=unix", "--snapshot-file-path=/tmp/cli.json", "--", "program"},
		{"--ingestion-transport=file", "--http-ingestion-listen=127.0.0.1:9091", "--", "program"},
	} {
		if _, err := Parse(args, testNow, nil); err == nil {
			t.Errorf("Parse accepted inactive transport options %q", args)
		}
	}
}

func TestParseLogPrecedenceAndRequiredNoFile(t *testing.T) {
	t.Parallel()

	environment := map[string]string{
		"METRICSHELL_LOG_LEVEL":           "info",
		"METRICSHELL_LOG_SELECTOR_VALUES": "false",
	}
	lookup := func(name string) (string, bool) { value, ok := environment[name]; return value, ok }
	configuration, err := Parse([]string{
		"--log-level=debug",
		"--log-selector-values=true",
		"--max-concurrent-scrapes=8",
		"--max-concurrent-ingestions=2",
		"--socket-max-connections=4",
		"--", "program",
	}, testNow, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Log.Level != "debug" || !configuration.Log.SelectorValues {
		t.Fatalf("log config = %+v", configuration.Log)
	}
	if got, want := RequiredNoFile(configuration), 16+8+2+4; got != want {
		t.Fatalf("RequiredNoFile = %d, want %d", got, want)
	}

	for _, option := range []string{"--log-level=trace", "--log-selector-values=yes"} {
		if _, err := Parse([]string{option, "--", "program"}, testNow, nil); err == nil {
			t.Errorf("Parse accepted %s", option)
		}
	}
}
