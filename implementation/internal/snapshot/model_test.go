package snapshot

import (
	"bytes"
	"sync"
	"testing"
)

func TestCanonicalSnapshotIsOrderedAndImmutable(t *testing.T) {
	input := []InputFamily{
		{Name: "temperature", Help: "Gauge.", Type: Gauge, Series: []InputSeries{
			{Labels: []InputLabel{{Name: "zone", Value: "b"}, {Name: "rack", Value: "2"}}, Value: "1.2300"},
			{Labels: []InputLabel{{Name: "rack", Value: "1"}, {Name: "zone", Value: "a"}}, Value: "2e0"},
		}},
		{Name: "empty", Help: "removed", Type: Counter},
		{Name: "requests", Help: "Counter.", Type: Counter, Series: []InputSeries{{Value: "42.0"}}},
	}
	candidate := NewCandidate(SchemaVersion, input, 100)
	input[0].Name = "mutated"
	input[0].Series[0].Labels[0].Value = "mutated"

	validated, err := Validate(candidate, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"schema_version\":1,\"families\":[{\"name\":\"requests\",\"help\":\"Counter.\",\"type\":\"counter\",\"series\":[{\"labels\":{},\"value\":\"42\"}]},{\"name\":\"temperature\",\"help\":\"Gauge.\",\"type\":\"gauge\",\"series\":[{\"labels\":{\"rack\":\"1\",\"zone\":\"a\"},\"value\":\"2\"},{\"labels\":{\"rack\":\"2\",\"zone\":\"b\"},\"value\":\"1.23\"}]}]}\n"
	if got := string(validated.Canonical()); got != want {
		t.Fatalf("canonical = %s, want %s", got, want)
	}
	if validated.SeriesCount() != 3 || validated.IsZeroSeries() {
		t.Fatalf("series=%d zero=%t", validated.SeriesCount(), validated.IsZeroSeries())
	}

	canonical := validated.Canonical()
	canonical[0] = 'X'
	families := validated.Families()
	families[0].name = "mutated"
	if bytes.Equal(canonical, validated.Canonical()) || validated.Families()[0].Name() != "requests" {
		t.Fatal("validated snapshot retained caller mutation")
	}
}

func TestZeroSeriesCanonicalization(t *testing.T) {
	for _, families := range [][]InputFamily{nil, {{Name: "valid_empty", Type: Gauge}}} {
		validated, err := Validate(NewCandidate(SchemaVersion, families, 1), DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(validated.Canonical()), "{\"schema_version\":1,\"families\":[]}\n"; got != want {
			t.Fatalf("canonical = %q, want %q", got, want)
		}
		if !validated.IsZeroSeries() {
			t.Fatal("zero-series snapshot not detected")
		}
	}
}

func TestHistogramCanonicalization(t *testing.T) {
	validated, err := Validate(NewCandidate(SchemaVersion, []InputFamily{{
		Name: "latency_seconds", Type: Histogram, Series: []InputSeries{{
			Labels: []InputLabel{{Name: "route", Value: "/"}},
			Histogram: &InputHistogram{Count: "7", Sum: "1.250", Buckets: []InputBucket{
				{UpperBound: "0.10", Count: "2"}, {UpperBound: "0.5", Count: "6"}, {UpperBound: "+Inf", Count: "7"},
			}},
		}},
	}}, 100), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	family := validated.Families()[0]
	histogram, ok := family.Series()[0].Histogram()
	if !ok || histogram.Sum() != "1.25" || histogram.Count() != 7 || histogram.Buckets()[0].UpperBound() != "0.1" {
		t.Fatalf("histogram = %#v", histogram)
	}
}

func TestDeterministicRejections(t *testing.T) {
	validHistogram := &InputHistogram{Count: "1", Sum: "1", Buckets: []InputBucket{{UpperBound: "+Inf", Count: "1"}}}
	tests := []struct {
		name      string
		candidate CandidateSnapshot
		limits    Limits
		reason    Reason
	}{
		{"schema", NewCandidate(2, nil, 1), DefaultLimits(), ReasonSchemaVersion},
		{"decoded limit", NewCandidate(1, nil, 3<<20), DefaultLimits(), ReasonPayloadLimit},
		{"reserved", NewCandidate(1, []InputFamily{{Name: "metricshell_attack", Type: Gauge}}, 1), DefaultLimits(), ReasonReservedName},
		{"counter suffix", NewCandidate(1, []InputFamily{{Name: "jobs_total", Type: Counter}}, 1), DefaultLimits(), ReasonMetadataConflict},
		{"histogram suffix", NewCandidate(1, []InputFamily{{Name: "jobs_bucket", Type: Histogram}}, 1), DefaultLimits(), ReasonMetadataConflict},
		{"component collision", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Counter}, {Name: "jobs_total", Type: Gauge}}, 1), DefaultLimits(), ReasonMetadataConflict},
		{"duplicate family", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Gauge}, {Name: "jobs", Type: Gauge}}, 1), DefaultLimits(), ReasonMetadataConflict},
		{"type conflict", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Gauge}, {Name: "jobs", Type: Counter}}, 1), DefaultLimits(), ReasonTypeConflict},
		{"duplicate series", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Gauge, Series: []InputSeries{{Value: "1"}, {Value: "2"}}}}, 1), DefaultLimits(), ReasonDuplicateSeries},
		{"duplicate label", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Gauge, Series: []InputSeries{{Value: "1", Labels: []InputLabel{{Name: "x"}, {Name: "x"}}}}}}, 1), DefaultLimits(), ReasonDuplicateSeries},
		{"label policy", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Gauge, Series: []InputSeries{{Value: "1", Labels: []InputLabel{{Name: "__name__"}}}}}}, 1), DefaultLimits(), ReasonPolicy},
		{"counter negative", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Counter, Series: []InputSeries{{Value: "-1"}}}}, 1), DefaultLimits(), ReasonNumericInvalid},
		{"counter negative zero", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Counter, Series: []InputSeries{{Value: "-0"}}}}, 1), DefaultLimits(), ReasonNumericInvalid},
		{"numeric overflow", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Gauge, Series: []InputSeries{{Value: "1e999"}}}}, 1), DefaultLimits(), ReasonNumericInvalid},
		{"numeric underflow", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Gauge, Series: []InputSeries{{Value: "1e-999"}}}}, 1), DefaultLimits(), ReasonNumericInvalid},
		{"histogram le label", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Histogram, Series: []InputSeries{{Labels: []InputLabel{{Name: "le"}}, Histogram: validHistogram}}}}, 1), DefaultLimits(), ReasonHistogramInvalid},
		{"histogram negative sum", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Histogram, Series: []InputSeries{{Histogram: &InputHistogram{Count: "1", Sum: "-1", Buckets: validHistogram.Buckets}}}}}, 1), DefaultLimits(), ReasonHistogramInvalid},
		{"histogram boundary collision", histogramCandidate(InputHistogram{
			Count: "1", Sum: "1", Buckets: []InputBucket{
				{UpperBound: "1", Count: "0"},
				{UpperBound: "1.0", Count: "0"},
				{UpperBound: "+Inf", Count: "1"},
			},
		}), DefaultLimits(), ReasonHistogramInvalid},
		{"histogram count", NewCandidate(1, []InputFamily{{Name: "jobs", Type: Histogram, Series: []InputSeries{{Histogram: &InputHistogram{Count: "2", Sum: "1", Buckets: validHistogram.Buckets}}}}}, 1), DefaultLimits(), ReasonHistogramInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Validate(test.candidate, test.limits)
			if reason, ok := RejectionReason(err); !ok || reason != test.reason || err.Error() != string(test.reason) {
				t.Fatalf("error = %v (%s), want %s", err, reason, test.reason)
			}
		})
	}
}

func TestLimitsAtBoundaryAndBeyond(t *testing.T) {
	base := InputFamily{Name: "m", Type: Gauge, Series: []InputSeries{{Labels: []InputLabel{{Name: "l", Value: "v"}}, Value: "1"}}}
	validated, err := Validate(NewCandidate(1, []InputFamily{base}, 1), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		limits Limits
		reason Reason
	}{
		{"series", withLimit(func(value *Limits) { value.Series = 0 }), ReasonSeriesLimit},
		{"labels", withLimit(func(value *Limits) { value.LabelsPerSeries = 0 }), ReasonLabelLimit},
		{"metric name", withLimit(func(value *Limits) { value.MetricNameBytes = 0 }), ReasonNameLimit},
		{"label name", withLimit(func(value *Limits) { value.LabelNameBytes = 0 }), ReasonNameLimit},
		{"label value", withLimit(func(value *Limits) { value.LabelValueBytes = 0 }), ReasonNameLimit},
		{"canonical", withLimit(func(value *Limits) { value.SnapshotBytes = validated.CanonicalBytes() - 1 }), ReasonPayloadLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Validate(NewCandidate(1, []InputFamily{base}, 1), test.limits)
			if reason, _ := RejectionReason(err); reason != test.reason {
				t.Fatalf("reason = %s, want %s", reason, test.reason)
			}
		})
	}
	limits := DefaultLimits()
	limits.SnapshotBytes = validated.CanonicalBytes()
	if _, err := Validate(NewCandidate(1, []InputFamily{base}, 1), limits); err != nil {
		t.Fatalf("exact canonical limit rejected: %v", err)
	}
}

func TestConcurrentReadersReceiveIndependentValues(t *testing.T) {
	validated, err := Validate(NewCandidate(1, []InputFamily{{Name: "value", Type: Gauge, Series: []InputSeries{{Value: "1"}}}}, 1), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	active := NewActive(validated, 7)
	var wait sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 100; iteration++ {
				copy := active.Validated()
				copy.canonical[0] = 'X'
				if active.Generation() != 7 || active.Validated().Canonical()[0] != '{' {
					t.Error("immutable active snapshot changed")
				}
			}
		}()
	}
	wait.Wait()
}

func withLimit(change func(*Limits)) Limits {
	limits := DefaultLimits()
	change(&limits)
	return limits
}

func histogramCandidate(histogram InputHistogram) CandidateSnapshot {
	return NewCandidate(SchemaVersion, []InputFamily{{
		Name: "jobs", Type: Histogram, Series: []InputSeries{{Histogram: &histogram}},
	}}, 1)
}
