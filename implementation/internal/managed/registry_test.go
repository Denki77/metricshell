package managed

import (
	"math"
	"reflect"
	"testing"
)

func TestRegistryStartsAsFreshEmptyEpoch(t *testing.T) {
	t.Parallel()

	first := NewRegistry()
	if got := first.Read(); got.Generation != 0 || len(got.Families) != 0 {
		t.Fatalf("initial snapshot = %+v", got)
	}
	if _, err := first.Declare(Descriptor{Name: "jobs", Type: Counter}); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Apply(Operation{Kind: CounterAdd, Name: "jobs", Value: 5}); err != nil {
		t.Fatal(err)
	}

	second := NewRegistry()
	if got := second.Read(); got.Generation != 0 || len(got.Families) != 0 {
		t.Fatalf("new execution reused state: %+v", got)
	}
}

func TestRegistryGenerationAccounting(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	assertGeneration(t, commitResult(registry.Declare(Descriptor{Name: "jobs", Type: Counter})), 1)
	assertGeneration(t, commitResult(registry.Declare(Descriptor{Name: "jobs", Type: Counter})), 1)
	assertGeneration(t, commitResult(registry.Declare(Descriptor{Name: "depth", Type: Gauge})), 2)
	assertGeneration(t, commitResult(registry.Apply(Operation{Kind: CounterAdd, Name: "jobs", Value: 0})), 3)
	assertGeneration(t, commitResult(registry.Apply(Operation{Kind: GaugeSet, Name: "depth", Value: 4})), 4)
	assertGeneration(t, commitResult(registry.ApplyBatch([]Operation{
		{Kind: CounterAdd, Name: "jobs", Value: 2},
		{Kind: GaugeSet, Name: "depth", Value: 3},
	})), 5)

	got := registry.Read()
	if got.Generation != 5 || onlySeriesFromSnapshot(t, got, "jobs").Value != 2 || onlySeriesFromSnapshot(t, got, "depth").Value != 3 {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestRegistryRejectionPreservesCompleteCommittedState(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	assertGeneration(t, commitResult(registry.Declare(Descriptor{Name: "latency", Type: Histogram, Buckets: []float64{1, math.Inf(1)}})), 1)
	assertGeneration(t, commitResult(registry.Apply(Operation{Kind: HistogramObserve, Name: "latency", Value: 0.5})), 2)
	before := registry.Read()

	if generation, err := registry.ApplyBatch([]Operation{
		{Kind: HistogramObserve, Name: "latency", Value: 0.75},
		{Kind: HistogramObserve, Name: "latency", Value: -1},
	}); err == nil || generation != before.Generation {
		t.Fatalf("rejected batch = generation %d, error %v", generation, err)
	}
	if after := registry.Read(); !reflect.DeepEqual(after, before) {
		t.Fatalf("rejection changed state:\nafter:  %+v\nbefore: %+v", after, before)
	}
	if generation, err := registry.Declare(Descriptor{Name: "latency", Type: Gauge}); err == nil || generation != before.Generation {
		t.Fatalf("conflicting declaration = generation %d, error %v", generation, err)
	}
	if after := registry.Read(); !reflect.DeepEqual(after, before) {
		t.Fatal("declaration rejection changed state")
	}
}

func TestRegistryReadIsCompleteAndDetached(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	assertGeneration(t, commitResult(registry.Declare(Descriptor{Name: "jobs", Type: Counter, Labels: []string{"worker"}})), 1)
	assertGeneration(t, commitResult(registry.Apply(Operation{Kind: CounterAdd, Name: "jobs", Labels: map[string]string{"worker": "one"}, Value: 1})), 2)
	assertGeneration(t, commitResult(registry.Apply(Operation{Kind: CounterAdd, Name: "jobs", Labels: map[string]string{"worker": "two"}, Value: 2})), 3)

	view := registry.Read()
	family := view.Families["jobs"]
	family.Descriptor.Labels[0] = "changed"
	for key, series := range family.Series {
		series.Value = 99
		series.Labels["worker"] = "changed"
		family.Series[key] = series
	}
	view.Families["jobs"] = family

	after := registry.Read()
	if after.Generation != 3 || len(after.Families["jobs"].Series) != 2 || after.Families["jobs"].Descriptor.Labels[0] != "worker" {
		t.Fatalf("detached read mutated registry: %+v", after)
	}
	values := map[float64]bool{}
	for _, series := range after.Families["jobs"].Series {
		values[series.Value] = true
	}
	if !values[1] || !values[2] {
		t.Fatalf("publisher state lost after detached/disconnected read: %+v", values)
	}
}

func TestRegistryRejectsGenerationOverflowBeforeMutation(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.generation = ^uint64(0)
	before := registry.Read()
	if generation, err := registry.Declare(Descriptor{Name: "jobs", Type: Counter}); err == nil || generation != before.Generation {
		t.Fatalf("overflow declaration = generation %d, error %v", generation, err)
	}
	if after := registry.Read(); !reflect.DeepEqual(after, before) {
		t.Fatal("generation overflow mutated registry")
	}
}

func TestRegistryFreezeIsSingleWinnerAndImmutable(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	assertGeneration(t, commitResult(registry.Declare(Descriptor{Name: "depth", Type: Gauge})), 1)

	first, firstWinner := registry.Freeze()
	second, secondWinner := registry.Freeze()
	if !firstWinner || secondWinner || first.Generation != 1 || !reflect.DeepEqual(second, first) || !registry.Frozen() {
		t.Fatalf("first=%+v winner=%v second=%+v winner=%v frozen=%v", first, firstWinner, second, secondWinner, registry.Frozen())
	}

	generation, err := registry.Apply(Operation{Kind: GaugeSet, Name: "depth", Value: 42})
	if reason, ok := RejectionReason(err); !ok || reason != ReasonLate || generation != first.Generation {
		t.Fatalf("late mutation generation=%d reason=%q error=%v", generation, reason, err)
	}
	if after := registry.Read(); !reflect.DeepEqual(after, first) {
		t.Fatalf("late mutation changed frozen registry: before=%+v after=%+v", first, after)
	}
}

type commit struct {
	generation uint64
	err        error
}

func commitResult(generation uint64, err error) commit {
	return commit{generation: generation, err: err}
}

func assertGeneration(t *testing.T, result commit, want uint64) {
	t.Helper()
	if result.err != nil || result.generation != want {
		t.Fatalf("generation = %d, error = %v; want %d, nil", result.generation, result.err, want)
	}
}

func onlySeriesFromSnapshot(t *testing.T, snapshot Snapshot, name string) Series {
	t.Helper()
	family := snapshot.Families[name]
	if len(family.Series) != 1 {
		t.Fatalf("%s series = %d, want 1", name, len(family.Series))
	}
	for _, series := range family.Series {
		return series
	}
	return Series{}
}
