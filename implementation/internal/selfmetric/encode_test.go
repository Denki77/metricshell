package selfmetric

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
)

func TestRuntimeStateGoldenForEveryLifecycleState(t *testing.T) {
	registry := mustRegistry(t, FinalWaitImmediate, newClock().Now)
	for _, selected := range lifecycle.States {
		if err := registry.SetRuntimeState(selected); err != nil {
			t.Fatal(err)
		}
		stateFamily := family(t, registry.View(), RuntimeState)
		encoded, err := Encode(View{Families: []Family{stateFamily}}, PrometheusText)
		if err != nil {
			t.Fatal(err)
		}
		var want strings.Builder
		want.WriteString("# HELP metricshell_runtime_state Current MetricShell runtime state as a one-hot vector.\n")
		want.WriteString("# TYPE metricshell_runtime_state gauge\n")
		for _, state := range lifecycle.States {
			value := 0
			if state == selected {
				value = 1
			}
			fmt.Fprintf(&want, "metricshell_runtime_state{state=\"%s\"} %d\n", state, value)
		}
		if string(encoded) != want.String() {
			t.Fatalf("state %s:\n%s\nwant:\n%s", selected, encoded, want.String())
		}
	}
}

func TestPrometheusAndOpenMetricsMetadataAndCounterNaming(t *testing.T) {
	registry, err := New(buildinfo.Info{Version: "1\n2", Revision: `a\"b`}, FinalWaitImmediate, newClock().Now)
	if err != nil {
		t.Fatal(err)
	}
	prometheus, err := registry.Encode(PrometheusText)
	if err != nil {
		t.Fatal(err)
	}
	openMetrics, err := registry.Encode(OpenMetricsText)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(prometheus), "# EOF") || !strings.HasSuffix(string(openMetrics), "# EOF\n") {
		t.Fatal("format terminator mismatch")
	}
	if !strings.Contains(string(prometheus), "# TYPE metricshell_runtime_failures_total counter\n") {
		t.Fatal("Prometheus counter metadata name mismatch")
	}
	if !strings.Contains(string(openMetrics), "# TYPE metricshell_runtime_failures counter\n") || !strings.Contains(string(openMetrics), "metricshell_runtime_failures_total{reason=\"internal\"} 0\n") {
		t.Fatal("OpenMetrics counter family/sample naming mismatch")
	}
	if !strings.Contains(string(prometheus), `metricshell_build_info{version="1\n2",revision="a\\\"b"} 1`) {
		t.Fatalf("build labels were not escaped: %s", prometheus[:min(len(prometheus), 300)])
	}
	for _, family := range registry.View().Families {
		metadataName := family.Name
		if family.Type == Counter {
			metadataName = strings.TrimSuffix(metadataName, "_total")
		}
		if !strings.Contains(string(openMetrics), "# HELP "+metadataName+" ") || !strings.Contains(string(openMetrics), "# TYPE "+metadataName+" "+string(family.Type)+"\n") {
			t.Fatalf("missing OpenMetrics metadata for %s", family.Name)
		}
	}
}

func TestHistogramEncodingIncludesCumulativeBucketsAndInfinity(t *testing.T) {
	registry := mustRegistry(t, FinalWaitImmediate, newClock().Now)
	labels := map[string]string{"phase": "finalization"}
	if err := registry.Observe(ShutdownPhaseDuration, labels, 0.1); err != nil {
		t.Fatal(err)
	}
	if err := registry.Observe(ShutdownPhaseDuration, labels, 3); err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(View{Families: []Family{family(t, registry.View(), ShutdownPhaseDuration)}}, PrometheusText)
	if err != nil {
		t.Fatal(err)
	}
	checks := []string{
		`metricshell_shutdown_phase_duration_seconds_bucket{phase="finalization",le="0.1"} 1`,
		`metricshell_shutdown_phase_duration_seconds_bucket{phase="finalization",le="5"} 2`,
		`metricshell_shutdown_phase_duration_seconds_bucket{phase="finalization",le="+Inf"} 2`,
		`metricshell_shutdown_phase_duration_seconds_sum{phase="finalization"} 3.1`,
		`metricshell_shutdown_phase_duration_seconds_count{phase="finalization"} 2`,
	}
	for _, check := range checks {
		if !strings.Contains(string(encoded), check+"\n") {
			t.Fatalf("missing %q in:\n%s", check, encoded)
		}
	}
}

func TestEncodeRejectsUnknownFormatAndBrokenHistogram(t *testing.T) {
	if _, err := Encode(View{}, "unknown"); err == nil {
		t.Fatal("unknown format accepted")
	}
	if _, err := Encode(View{Families: []Family{{Name: "metricshell_broken", Help: "Broken.", Type: Histogram, Samples: []Sample{{}}}}}, PrometheusText); err == nil {
		t.Fatal("histogram without value accepted")
	}
}
