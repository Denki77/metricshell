package config

import (
	"reflect"
	"testing"
	"time"
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
