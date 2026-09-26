package managed

import (
	"math"
	"testing"
)

func TestResourceLimitsRejectBeforeMutation(t *testing.T) {
	limits := DefaultLimits()
	limits.Families, limits.Series, limits.Labels, limits.Buckets, limits.Batch = 1, 1, 1, 2, 1
	limits.MetricNameBytes, limits.LabelNameBytes, limits.LabelValueBytes, limits.HelpBytes = 8, 4, 4, 4
	registry, err := NewRegistryWithLimits(limits)
	if err != nil {
		t.Fatal(err)
	}
	if generation, err := registry.Declare(Descriptor{Name: "latency", Help: "help", Type: Histogram, Labels: []string{"zone"}, Buckets: []float64{1, math.Inf(1)}}); err != nil || generation != 1 {
		t.Fatalf("declare generation=%d err=%v", generation, err)
	}
	if generation, err := registry.Apply(Operation{Kind: HistogramObserve, Name: "latency", Labels: map[string]string{"zone": "west"}, Value: 1}); err != nil || generation != 2 {
		t.Fatalf("observe generation=%d err=%v", generation, err)
	}
	checks := []struct {
		name   string
		apply  func() (uint64, error)
		reason Reason
	}{
		{"family", func() (uint64, error) { return registry.Declare(Descriptor{Name: "other", Type: Gauge}) }, ReasonFamilyLimit},
		{"series", func() (uint64, error) {
			return registry.Apply(Operation{Kind: HistogramObserve, Name: "latency", Labels: map[string]string{"zone": "east"}, Value: 1})
		}, ReasonSeriesLimit},
		{"value", func() (uint64, error) {
			return registry.Apply(Operation{Kind: HistogramObserve, Name: "latency", Labels: map[string]string{"zone": "12345"}, Value: 1})
		}, ReasonValueLimit},
		{"batch", func() (uint64, error) {
			return registry.ApplyBatch([]Operation{{Kind: HistogramObserve}, {Kind: HistogramObserve}})
		}, ReasonBatchLimit},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			generation, err := check.apply()
			if reason, ok := RejectionReason(err); !ok || reason != check.reason || generation != 2 {
				t.Fatalf("generation=%d reason=%q err=%v", generation, reason, err)
			}
			snapshot := registry.Read()
			if snapshot.Generation != 2 || len(snapshot.Families) != 1 || len(snapshot.Families["latency"].Series) != 1 {
				t.Fatalf("state changed after rejection: %+v", snapshot)
			}
		})
	}
}

func TestDescriptorResourceLimitMatrix(t *testing.T) {
	base := DefaultLimits()
	tests := []struct {
		name       string
		configure  func(*Limits)
		descriptor Descriptor
		reason     Reason
	}{
		{"labels", func(l *Limits) { l.Labels = 1 }, Descriptor{Name: "metric", Type: Gauge, Labels: []string{"a", "b"}}, ReasonLabelLimit},
		{"buckets", func(l *Limits) { l.Buckets = 1 }, Descriptor{Name: "metric", Type: Histogram, Buckets: []float64{1, math.Inf(1)}}, ReasonBucketLimit},
		{"metric name", func(l *Limits) { l.MetricNameBytes = 5 }, Descriptor{Name: "metric", Type: Gauge}, ReasonNameLimit},
		{"label name", func(l *Limits) { l.LabelNameBytes = 3 }, Descriptor{Name: "metric", Type: Gauge, Labels: []string{"zone"}}, ReasonNameLimit},
		{"help", func(l *Limits) { l.HelpBytes = 3 }, Descriptor{Name: "metric", Help: "help", Type: Gauge}, ReasonHelpLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limits := base
			test.configure(&limits)
			registry, err := NewRegistryWithLimits(limits)
			if err != nil {
				t.Fatal(err)
			}
			generation, err := registry.Declare(test.descriptor)
			if reason, ok := RejectionReason(err); !ok || reason != test.reason || generation != 0 || len(registry.Read().Families) != 0 {
				t.Fatalf("generation=%d reason=%q err=%v", generation, reason, err)
			}
		})
	}
	invalid := base
	invalid.MetricNameBytes = 1025
	if _, err := NewRegistryWithLimits(invalid); err == nil {
		t.Fatal("invalid startup limit accepted")
	}
}
