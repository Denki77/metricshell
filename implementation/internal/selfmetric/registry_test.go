package selfmetric

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/finalwait"
	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
	applicationsnapshot "github.com/Denki77/metricshell/implementation/internal/snapshot"
)

func TestRegistryContainsCompleteBoundedSpecification(t *testing.T) {
	clock := newClock()
	registry := mustRegistry(t, FinalWaitDuration, clock.Now)
	view := registry.View()
	wantNames := []string{
		BuildInfo, UptimeSeconds, RuntimeState, RuntimeFailuresTotal,
		WorkloadRunning, WorkloadProcessID, WorkloadStartsTotal, WorkloadExitCode, WorkloadSignalsTotal,
		WorkloadForcedTotal, ChildrenReapedTotal, SnapshotGeneration, SnapshotSeries, SnapshotBytes,
		SnapshotPublicationsTotal, SnapshotRejectionsTotal, IngestionInflight, IngestionConnections,
		IngestionLastSuccess, FileReconciliationsTotal, FileWatchEventsTotal, SocketTransactionsInflight,
		SocketFramesRejectedTotal, FilterRules, FilterFamilies, ExpositionRequestsTotal, ExpositionInflight,
		ExpositionResponseBytes, FinalWaitActive, FinalWaitModeInfo, FinalWaitRequiredScrapes,
		FinalWaitCompletedScrapes, FinalScrapeAttemptsTotal, FinalWaitCompletionsTotal, FinalWaitDeadline,
		ShutdownActive, ShutdownDeadline, ShutdownPhaseDuration,
	}
	if len(view.Families) != len(wantNames) {
		t.Fatalf("families = %d, want %d", len(view.Families), len(wantNames))
	}
	series := 0
	for index, family := range view.Families {
		if family.Name != wantNames[index] || family.Help == "" {
			t.Fatalf("family %d = %#v", index, family)
		}
		if family.Type == Counter && family.Name[len(family.Name)-len("_total"):] != "_total" {
			t.Fatalf("counter without suffix: %s", family.Name)
		}
		series += len(family.Samples)
	}
	if series != 187 {
		t.Fatalf("bounded cardinality = %d, want 187", series)
	}
	assertSeriesCount(t, view, RuntimeState, len(lifecycle.States))
	assertSeriesCount(t, view, WorkloadSignalsTotal, len(Signals)*len(SignalTargets))
	assertSeriesCount(t, view, SnapshotPublicationsTotal, len(Transports)*len(PublicationOutcomes))
	assertSeriesCount(t, view, SnapshotRejectionsTotal, len(Transports)*len(applicationsnapshot.RejectionReasons))
	assertSeriesCount(t, view, ShutdownPhaseDuration, len(ShutdownPhases))
}

func TestRuntimeStateAndFinalWaitModeAreExactOneHotVectors(t *testing.T) {
	registry := mustRegistry(t, FinalWaitImmediate, newClock().Now)
	for _, state := range lifecycle.States {
		if err := registry.SetRuntimeState(state); err != nil {
			t.Fatal(err)
		}
		assertOneHot(t, family(t, registry.View(), RuntimeState), "state", string(state))
	}
	if err := registry.SetRuntimeState(lifecycle.State("attacker")); !errors.Is(err, ErrValue) {
		t.Fatalf("invalid state error = %v", err)
	}
	assertOneHot(t, family(t, registry.View(), RuntimeState), "state", string(lifecycle.Terminated))

	for _, mode := range []FinalWaitMode{FinalWaitImmediate, FinalWaitDuration, FinalWaitScrapes} {
		if err := registry.SetFinalWaitMode(mode); err != nil {
			t.Fatal(err)
		}
		assertOneHot(t, family(t, registry.View(), FinalWaitModeInfo), "mode", string(mode))
	}
	if err := registry.SetFinalWaitMode("attacker"); !errors.Is(err, ErrValue) {
		t.Fatalf("invalid mode error = %v", err)
	}
}

func TestFinalWaitReasonRegistryMatchesStateMachine(t *testing.T) {
	want := []string{
		string(finalwait.ReasonImmediate),
		string(finalwait.ReasonDurationElapsed),
		string(finalwait.ReasonRequiredScrapes),
		string(finalwait.ReasonTimeout),
		string(finalwait.ReasonExternalTermination),
		string(finalwait.ReasonRuntimeFailure),
	}
	if !reflect.DeepEqual(FinalWaitReasons[:], want) {
		t.Fatalf("final-wait reasons = %#v, want %#v", FinalWaitReasons, want)
	}
}

func TestRegistryRejectsUnboundedLabelsAndWrongMetricOperations(t *testing.T) {
	registry := mustRegistry(t, FinalWaitImmediate, newClock().Now)
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"unknown metric", registry.AddCounter("metricshell_attacker_total", nil, 1), ErrUnknownMetric},
		{"unknown label value", registry.AddCounter(RuntimeFailuresTotal, map[string]string{"reason": "raw error"}, 1), ErrLabels},
		{"unknown label name", registry.AddCounter(RuntimeFailuresTotal, map[string]string{"raw": "internal"}, 1), ErrLabels},
		{"extra label", registry.AddCounter(RuntimeFailuresTotal, map[string]string{"reason": "internal", "raw": "x"}, 1), ErrLabels},
		{"type mismatch", registry.AddCounter(WorkloadRunning, nil, 1), ErrMetricType},
		{"managed gauge", registry.SetGauge(RuntimeState, map[string]string{"state": "running"}, 1), ErrManaged},
		{"non-finite gauge", registry.SetGauge(ShutdownDeadline, nil, math.Inf(1)), ErrValue},
		{"negative gauge", registry.SetGauge(ShutdownDeadline, nil, -1), ErrValue},
		{"non-binary gauge", registry.SetGauge(ShutdownActive, nil, 2), ErrValue},
		{"fractional count gauge", registry.SetGauge(IngestionInflight, map[string]string{"transport": "http"}, 0.5), ErrValue},
	}
	for _, test := range tests {
		if !errors.Is(test.err, test.want) {
			t.Errorf("%s error = %v, want %v", test.name, test.err, test.want)
		}
	}
	if got := len(family(t, registry.View(), RuntimeFailuresTotal).Samples); got != len(RuntimeFailureReasons) {
		t.Fatalf("rejected labels changed cardinality: %d", got)
	}
}

func TestSnapshotProjectionIsAtomicAndRemainsMutableAfterApplicationFreeze(t *testing.T) {
	registry := mustRegistry(t, FinalWaitImmediate, newClock().Now)
	holder := applicationsnapshot.NewInitialHolder()
	assertSnapshotProjection(t, registry.View(), holder.Active())

	validated := mustApplicationSnapshot(t, `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`)
	active, err := holder.Install(validated)
	if err != nil {
		t.Fatal(err)
	}
	registry.SetActiveSnapshot(active)
	assertSnapshotProjection(t, registry.View(), active)
	holder.Freeze()
	if err := registry.AddCounter(SnapshotPublicationsTotal, map[string]string{"transport": "file", "outcome": "accepted"}, 1); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetGauge(FinalWaitActive, nil, 1); err != nil {
		t.Fatal(err)
	}
	if counterValue(t, registry.View(), SnapshotPublicationsTotal, map[string]string{"transport": "file", "outcome": "accepted"}) != 1 || gaugeValue(t, registry.View(), FinalWaitActive, nil) != 1 {
		t.Fatal("self-metrics stopped after application freeze")
	}
}

func TestConcurrentSnapshotProjectionAndUpdatesAreRaceSafe(t *testing.T) {
	registry := mustRegistry(t, FinalWaitImmediate, newClock().Now)
	zeroActive := applicationsnapshot.NewActive(applicationsnapshot.Zero(), 10)
	full := mustApplicationSnapshot(t, `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`)
	fullActive := applicationsnapshot.NewActive(full, 11)
	allowed := map[string]struct{}{projectionKey(zeroActive): {}, projectionKey(fullActive): {}}

	var wait sync.WaitGroup
	start := make(chan struct{})
	for worker := 0; worker < 24; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for iteration := 0; iteration < 500; iteration++ {
				view := registry.View()
				key := fmt.Sprintf("%.0f:%.0f:%.0f", gaugeValue(t, view, SnapshotGeneration, nil), gaugeValue(t, view, SnapshotSeries, nil), gaugeValue(t, view, SnapshotBytes, nil))
				if _, exists := allowed[key]; !exists {
					t.Errorf("partial snapshot projection %s", key)
					return
				}
			}
		}()
	}
	registry.SetActiveSnapshot(zeroActive)
	close(start)
	for iteration := 0; iteration < 500; iteration++ {
		registry.SetActiveSnapshot(fullActive)
		registry.SetActiveSnapshot(zeroActive)
	}
	wait.Wait()
}

func TestCounterHistogramUptimeAndRestartRules(t *testing.T) {
	clock := newClock()
	registry := mustRegistry(t, FinalWaitDuration, clock.Now)
	labels := map[string]string{"reason": "internal"}
	if err := registry.AddCounter(RuntimeFailuresTotal, labels, 2); err != nil {
		t.Fatal(err)
	}
	if err := registry.Observe(ShutdownPhaseDuration, map[string]string{"phase": "finalization"}, 0.1); err != nil {
		t.Fatal(err)
	}
	if err := registry.Observe(ShutdownPhaseDuration, map[string]string{"phase": "finalization"}, 3); err != nil {
		t.Fatal(err)
	}
	clock.Advance(5 * time.Second)
	view := registry.View()
	if counterValue(t, view, RuntimeFailuresTotal, labels) != 2 || gaugeValue(t, view, UptimeSeconds, nil) != 5 {
		t.Fatal("counter or uptime update failed")
	}
	histogram := histogramValue(t, view, ShutdownPhaseDuration, map[string]string{"phase": "finalization"})
	if histogram.Count != 2 || histogram.Sum != 3.1 || histogram.Buckets[4].Count != 1 || histogram.Buckets[9].Count != 2 {
		t.Fatalf("histogram = %#v", histogram)
	}

	restarted := mustRegistry(t, FinalWaitDuration, clock.Now)
	if counterValue(t, restarted.View(), RuntimeFailuresTotal, labels) != 0 || gaugeValue(t, restarted.View(), WorkloadExitCode, nil) != -1 {
		t.Fatal("process-local values did not reset")
	}
}

func TestHistogramSumOverflowRejectsWithoutMutation(t *testing.T) {
	registry := mustRegistry(t, FinalWaitImmediate, newClock().Now)
	labels := map[string]string{"phase": "total"}
	if err := registry.Observe(ShutdownPhaseDuration, labels, math.MaxFloat64); err != nil {
		t.Fatal(err)
	}
	before := histogramValue(t, registry.View(), ShutdownPhaseDuration, labels)
	if err := registry.Observe(ShutdownPhaseDuration, labels, math.MaxFloat64); !errors.Is(err, ErrValue) {
		t.Fatalf("overflow error = %v", err)
	}
	after := histogramValue(t, registry.View(), ShutdownPhaseDuration, labels)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("overflow mutated histogram: before=%#v after=%#v", before, after)
	}
}

func TestBuildWorkloadAndReturnedViewOwnership(t *testing.T) {
	registry, err := New(buildinfo.Info{Version: "1.2.3", Revision: "abc"}, FinalWaitImmediate, newClock().Now)
	if err != nil {
		t.Fatal(err)
	}
	build := family(t, registry.View(), BuildInfo)
	if len(build.Samples) != 1 || build.Samples[0].Gauge != 1 || !reflect.DeepEqual(build.Samples[0].Labels, []Label{{Name: "version", Value: "1.2.3"}, {Name: "revision", Value: "abc"}}) {
		t.Fatalf("build info = %#v", build)
	}
	if err := registry.SetWorkload(123, true); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetWorkloadExitCode(17); err != nil {
		t.Fatal(err)
	}
	if gaugeValue(t, registry.View(), WorkloadRunning, nil) != 1 || gaugeValue(t, registry.View(), WorkloadProcessID, nil) != 123 || gaugeValue(t, registry.View(), WorkloadExitCode, nil) != 17 {
		t.Fatal("workload projection failed")
	}
	if err := registry.SetWorkload(123, false); !errors.Is(err, ErrValue) {
		t.Fatalf("inconsistent workload update error = %v", err)
	}

	view := registry.View()
	view.Families[0].Name = "mutated"
	view.Families[0].Samples[0].Labels[0].Value = "mutated"
	histogramValue(t, view, ShutdownPhaseDuration, map[string]string{"phase": "total"}).Buckets[0].Count = 99
	if got := family(t, registry.View(), BuildInfo); got.Name != BuildInfo || got.Samples[0].Labels[0].Value != "1.2.3" {
		t.Fatal("registry retained caller mutation")
	}
}

func TestRegistryConstructionRejectsDefinitionConflicts(t *testing.T) {
	duplicate := definitions()
	duplicate = append(duplicate, duplicate[0])
	if _, err := newRegistry(duplicate, newClock().Now); err == nil {
		t.Fatal("duplicate definition accepted")
	}
	invalid := definitions()
	invalid[0].help = ""
	if _, err := newRegistry(invalid, newClock().Now); err == nil {
		t.Fatal("definition without HELP accepted")
	}
	invalid = definitions()
	invalid[0].labelNames = []string{"revision", "version"}
	if _, err := newRegistry(invalid, newClock().Now); err == nil {
		t.Fatal("invalid build-info labels accepted")
	}
	invalid = definitions()
	invalid[1].typeID = MetricType("summary")
	if _, err := newRegistry(invalid, newClock().Now); err == nil {
		t.Fatal("invalid metric type accepted")
	}
}

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock {
	return &clock{now: time.Unix(1_700_000_000, 0)}
}

func (clock *clock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *clock) Advance(duration time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(duration)
}

func mustRegistry(t *testing.T, mode FinalWaitMode, now func() time.Time) *Registry {
	t.Helper()
	registry, err := New(buildinfo.Info{Version: "test", Revision: "test"}, mode, now)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func mustApplicationSnapshot(t *testing.T, document string) applicationsnapshot.ValidatedSnapshot {
	t.Helper()
	validated, err := applicationsnapshot.Parse([]byte(document), applicationsnapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	return validated
}

func family(t *testing.T, view View, name string) Family {
	t.Helper()
	for _, family := range view.Families {
		if family.Name == name {
			return family
		}
	}
	t.Fatalf("family %s not found", name)
	return Family{}
}

func assertSeriesCount(t *testing.T, view View, name string, want int) {
	t.Helper()
	if got := len(family(t, view, name).Samples); got != want {
		t.Fatalf("%s series = %d, want %d", name, got, want)
	}
}

func assertOneHot(t *testing.T, family Family, labelName, selected string) {
	t.Helper()
	ones := 0
	for _, sample := range family.Samples {
		if sample.Gauge == 1 {
			ones++
			if labelValue(sample.Labels, labelName) != selected {
				t.Fatalf("selected %s = %s, want %s", labelName, labelValue(sample.Labels, labelName), selected)
			}
		} else if sample.Gauge != 0 {
			t.Fatalf("one-hot value = %v", sample.Gauge)
		}
	}
	if ones != 1 {
		t.Fatalf("one-hot ones = %d", ones)
	}
}

func gaugeValue(t *testing.T, view View, name string, labels map[string]string) float64 {
	t.Helper()
	return matchingSample(t, family(t, view, name), labels).Gauge
}

func counterValue(t *testing.T, view View, name string, labels map[string]string) uint64 {
	t.Helper()
	return matchingSample(t, family(t, view, name), labels).Counter
}

func histogramValue(t *testing.T, view View, name string, labels map[string]string) *HistogramValue {
	t.Helper()
	value := matchingSample(t, family(t, view, name), labels).Histogram
	if value == nil {
		t.Fatalf("%s is not a histogram", name)
	}
	return value
}

func matchingSample(t *testing.T, family Family, labels map[string]string) Sample {
	t.Helper()
	for _, sample := range family.Samples {
		if labelsMatch(sample.Labels, labels) {
			return sample
		}
	}
	t.Fatalf("sample %s%v not found", family.Name, labels)
	return Sample{}
}

func labelsMatch(actual []Label, wanted map[string]string) bool {
	if len(actual) != len(wanted) {
		return false
	}
	for _, label := range actual {
		if wanted[label.Name] != label.Value {
			return false
		}
	}
	return true
}

func labelValue(labels []Label, name string) string {
	for _, label := range labels {
		if label.Name == name {
			return label.Value
		}
	}
	return ""
}

func assertSnapshotProjection(t *testing.T, view View, active applicationsnapshot.ActiveSnapshot) {
	t.Helper()
	validated := active.Validated()
	if got, want := gaugeValue(t, view, SnapshotGeneration, nil), float64(active.Generation()); got != want {
		t.Fatalf("generation = %v, want %v", got, want)
	}
	if got, want := gaugeValue(t, view, SnapshotSeries, nil), float64(validated.SeriesCount()); got != want {
		t.Fatalf("series = %v, want %v", got, want)
	}
	if got, want := gaugeValue(t, view, SnapshotBytes, nil), float64(validated.CanonicalBytes()); got != want {
		t.Fatalf("bytes = %v, want %v", got, want)
	}
}

func projectionKey(active applicationsnapshot.ActiveSnapshot) string {
	validated := active.Validated()
	return fmt.Sprintf("%d:%d:%d", active.Generation(), validated.SeriesCount(), validated.CanonicalBytes())
}
