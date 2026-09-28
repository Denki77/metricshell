package managed

import (
	"math"
	"reflect"
	"testing"
)

func TestDescriptorValidationAndConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		descriptor Descriptor
		reason     Reason
	}{
		{name: "invalid name", descriptor: Descriptor{Name: "bad-name", Type: Gauge}, reason: ReasonInvalidName},
		{name: "reserved", descriptor: Descriptor{Name: "metricshell_private", Type: Gauge}, reason: ReasonReservedName},
		{name: "unknown type", descriptor: Descriptor{Name: "metric", Type: "summary"}, reason: ReasonInvalidType},
		{name: "counter derived suffix", descriptor: Descriptor{Name: "requests_total", Type: Counter}, reason: ReasonDerivedName},
		{name: "histogram derived suffix", descriptor: Descriptor{Name: "latency_sum", Type: Histogram, Buckets: []float64{math.Inf(1)}}, reason: ReasonDerivedName},
		{name: "invalid label", descriptor: Descriptor{Name: "metric", Type: Gauge, Labels: []string{"bad-label"}}, reason: ReasonInvalidLabels},
		{name: "colon label", descriptor: Descriptor{Name: "metric", Type: Gauge, Labels: []string{"bad:label"}}, reason: ReasonInvalidLabels},
		{name: "reserved label", descriptor: Descriptor{Name: "metric", Type: Gauge, Labels: []string{"__name__"}}, reason: ReasonInvalidLabels},
		{name: "histogram le label", descriptor: Descriptor{Name: "metric", Type: Histogram, Labels: []string{"le"}, Buckets: []float64{math.Inf(1)}}, reason: ReasonInvalidLabels},
		{name: "duplicate label", descriptor: Descriptor{Name: "metric", Type: Gauge, Labels: []string{"zone", "zone"}}, reason: ReasonDuplicateLabel},
		{name: "buckets on gauge", descriptor: Descriptor{Name: "metric", Type: Gauge, Buckets: []float64{1}}, reason: ReasonInvalidBuckets},
		{name: "missing infinity", descriptor: Descriptor{Name: "metric", Type: Histogram, Buckets: []float64{1}}, reason: ReasonInvalidBuckets},
		{name: "negative bucket", descriptor: Descriptor{Name: "metric", Type: Histogram, Buckets: []float64{-1, math.Inf(1)}}, reason: ReasonInvalidBuckets},
		{name: "negative zero bucket", descriptor: Descriptor{Name: "metric", Type: Histogram, Buckets: []float64{math.Copysign(0, -1), math.Inf(1)}}, reason: ReasonInvalidBuckets},
		{name: "unordered buckets", descriptor: Descriptor{Name: "metric", Type: Histogram, Buckets: []float64{1, 1, math.Inf(1)}}, reason: ReasonInvalidBuckets},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := NewModel()
			assertReason(t, model.Declare(test.descriptor), test.reason)
			if len(model.Families()) != 0 {
				t.Fatal("rejected declaration mutated model")
			}
		})
	}

	model := NewModel()
	descriptor := Descriptor{Name: "requests", Help: "Requests.", Type: Counter, Labels: []string{"status", "method"}}
	if err := model.Declare(descriptor); err != nil {
		t.Fatal(err)
	}
	if err := model.Declare(Descriptor{Name: "requests", Help: "Requests.", Type: Counter, Labels: []string{"method", "status"}}); err != nil {
		t.Fatalf("equivalent declaration rejected: %v", err)
	}
	for _, conflict := range []Descriptor{
		{Name: "requests", Help: "Changed.", Type: Counter, Labels: []string{"method", "status"}},
		{Name: "requests", Help: "Requests.", Type: Gauge, Labels: []string{"method", "status"}},
		{Name: "requests_total", Type: Gauge},
	} {
		assertReason(t, model.Declare(conflict), map[bool]Reason{true: ReasonDerivedName, false: ReasonDescriptorConflict}[conflict.Name == "requests_total"])
	}
}

func TestCounterSemanticsAndCanonicalLabels(t *testing.T) {
	t.Parallel()

	model := NewModel()
	mustDeclare(t, model, Descriptor{Name: "requests", Type: Counter, Labels: []string{"method", "status"}})
	labels := map[string]string{"status": "200", "method": "GET"}
	if err := model.Apply(Operation{Kind: CounterInitialize, Name: "requests", Labels: labels, Value: 2}); err != nil {
		t.Fatal(err)
	}
	labels["status"] = "mutated"
	if err := model.Apply(Operation{Kind: CounterAdd, Name: "requests", Labels: map[string]string{"method": "GET", "status": "200"}, Value: 3}); err != nil {
		t.Fatal(err)
	}
	series := onlySeries(t, model, "requests")
	if series.Value != 5 || series.Labels["status"] != "200" {
		t.Fatalf("series = %+v", series)
	}

	before := model.Families()
	for _, operation := range []Operation{
		{Kind: CounterInitialize, Name: "requests", Labels: map[string]string{"method": "GET", "status": "200"}, Value: 1},
		{Kind: CounterAdd, Name: "requests", Labels: map[string]string{"method": "GET"}, Value: 1},
		{Kind: CounterAdd, Name: "requests", Labels: map[string]string{"method": "GET", "status": "200"}, Value: -1},
		{Kind: CounterAdd, Name: "requests", Labels: map[string]string{"method": "GET", "status": "200"}, Value: math.NaN()},
		{Kind: CounterAdd, Name: "requests", Labels: map[string]string{"method": "GET", "status": "200"}, Value: math.Inf(1)},
	} {
		if err := model.Apply(operation); err == nil {
			t.Fatalf("Apply(%+v) succeeded", operation)
		}
		if !reflect.DeepEqual(model.Families(), before) {
			t.Fatal("rejected counter operation mutated model")
		}
	}
}

func TestGaugeAndHistogramSemantics(t *testing.T) {
	t.Parallel()

	model := NewModel()
	mustDeclare(t, model, Descriptor{Name: "temperature", Type: Gauge})
	mustDeclare(t, model, Descriptor{Name: "latency", Type: Histogram, Labels: []string{"route"}, Buckets: []float64{0, 0.5, 1, math.Inf(1)}})
	for _, value := range []float64{-3, 0, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := model.Apply(Operation{Kind: GaugeSet, Name: "temperature", Value: value}); err != nil {
			t.Fatalf("GaugeSet(%v): %v", value, err)
		}
		if got := onlySeries(t, model, "temperature").Value; math.IsNaN(value) != math.IsNaN(got) || !math.IsNaN(value) && got != value {
			t.Fatalf("gauge value = %v, want %v", got, value)
		}
	}
	labels := map[string]string{"route": "/jobs"}
	for _, value := range []float64{0, 0.5, 2, math.Inf(1)} {
		if err := model.Apply(Operation{Kind: HistogramObserve, Name: "latency", Labels: labels, Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	series := onlySeries(t, model, "latency")
	if series.Count != 4 || !math.IsInf(series.Sum, 1) || !reflect.DeepEqual(series.BucketCounts, []uint64{1, 2, 2, 4}) {
		t.Fatalf("histogram = %+v", series)
	}
	before := model.Families()
	for _, value := range []float64{-1, math.Copysign(0, -1), math.NaN(), math.Inf(-1)} {
		assertReason(t, model.Apply(Operation{Kind: HistogramObserve, Name: "latency", Labels: labels, Value: value}), ReasonInvalidNumber)
		if !reflect.DeepEqual(model.Families(), before) {
			t.Fatal("rejected observation mutated histogram")
		}
	}
}

func TestBatchIsAllOrNothing(t *testing.T) {
	t.Parallel()

	model := NewModel()
	mustDeclare(t, model, Descriptor{Name: "jobs", Type: Counter})
	mustDeclare(t, model, Descriptor{Name: "depth", Type: Gauge})
	if err := model.ApplyBatch([]Operation{
		{Kind: CounterAdd, Name: "jobs", Value: 2},
		{Kind: GaugeSet, Name: "depth", Value: 7},
	}); err != nil {
		t.Fatal(err)
	}
	before := model.Families()
	assertReason(t, model.ApplyBatch([]Operation{
		{Kind: CounterAdd, Name: "jobs", Value: 3},
		{Kind: GaugeSet, Name: "missing", Value: 1},
	}), ReasonUndeclared)
	if !reflect.DeepEqual(model.Families(), before) {
		t.Fatal("rejected batch committed a prefix")
	}
	assertReason(t, model.ApplyBatch(nil), ReasonUnsupported)
}

func TestReturnedStateCannotMutateModel(t *testing.T) {
	t.Parallel()

	model := NewModel()
	mustDeclare(t, model, Descriptor{Name: "depth", Type: Gauge, Labels: []string{"queue"}})
	if err := model.Apply(Operation{Kind: GaugeSet, Name: "depth", Labels: map[string]string{"queue": "main"}, Value: 4}); err != nil {
		t.Fatal(err)
	}
	view := model.Families()
	view["depth"].Descriptor.Labels[0] = "changed"
	for key, series := range view["depth"].Series {
		series.Labels["queue"] = "changed"
		view["depth"].Series[key] = series
	}
	if got := onlySeries(t, model, "depth"); got.Labels["queue"] != "main" {
		t.Fatalf("model aliases returned state: %+v", got)
	}
}

func mustDeclare(t *testing.T, model *Model, descriptor Descriptor) {
	t.Helper()
	if err := model.Declare(descriptor); err != nil {
		t.Fatal(err)
	}
}

func onlySeries(t *testing.T, model *Model, familyName string) Series {
	t.Helper()
	family := model.Families()[familyName]
	if len(family.Series) != 1 {
		t.Fatalf("series count = %d, want 1", len(family.Series))
	}
	for _, series := range family.Series {
		return series
	}
	return Series{}
}

func assertReason(t *testing.T, err error, want Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %q", want)
	}
	got, ok := RejectionReason(err)
	if !ok || got != want {
		t.Fatalf("reason = %q, %v; want %q", got, ok, want)
	}
}

func TestPrometheusSpecialNumericValues(t *testing.T) {
	tests := []struct {
		name       string
		descriptor Descriptor
		operation  Operation
		accepted   bool
	}{
		{"gauge NaN", Descriptor{Name: "metric", Type: Gauge}, Operation{Kind: GaugeSet, Name: "metric", Value: math.NaN()}, true},
		{"gauge positive infinity", Descriptor{Name: "metric", Type: Gauge}, Operation{Kind: GaugeSet, Name: "metric", Value: math.Inf(1)}, true},
		{"gauge negative infinity", Descriptor{Name: "metric", Type: Gauge}, Operation{Kind: GaugeSet, Name: "metric", Value: math.Inf(-1)}, true},
		{"histogram positive infinity", Descriptor{Name: "metric", Type: Histogram, Buckets: []float64{1, math.Inf(1)}}, Operation{Kind: HistogramObserve, Name: "metric", Value: math.Inf(1)}, true},
		{"histogram NaN", Descriptor{Name: "metric", Type: Histogram, Buckets: []float64{1, math.Inf(1)}}, Operation{Kind: HistogramObserve, Name: "metric", Value: math.NaN()}, false},
		{"histogram negative infinity", Descriptor{Name: "metric", Type: Histogram, Buckets: []float64{1, math.Inf(1)}}, Operation{Kind: HistogramObserve, Name: "metric", Value: math.Inf(-1)}, false},
		{"counter positive infinity", Descriptor{Name: "metric", Type: Counter}, Operation{Kind: CounterAdd, Name: "metric", Value: math.Inf(1)}, false},
		{"counter NaN", Descriptor{Name: "metric", Type: Counter}, Operation{Kind: CounterAdd, Name: "metric", Value: math.NaN()}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := NewRegistry()
			if _, err := registry.Declare(test.descriptor); err != nil {
				t.Fatal(err)
			}
			before := registry.Read().Generation
			generation, err := registry.Apply(test.operation)
			if test.accepted && err != nil {
				t.Fatalf("special value rejected: %v", err)
			}
			if !test.accepted {
				if reason, ok := RejectionReason(err); !ok || reason != ReasonInvalidNumber || generation != before {
					t.Fatalf("generation=%d reason=%q err=%v", generation, reason, err)
				}
			}
		})
	}
}
