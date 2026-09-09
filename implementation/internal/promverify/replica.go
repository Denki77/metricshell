package promverify

import (
	"fmt"
	"sort"
	"time"
)

type Sample struct {
	Replica   string
	Target    string
	Timestamp time.Time
	Value     float64
	Stale     bool
}

type Result struct {
	Replica   string
	Target    string
	Timestamp time.Time
	Value     float64
}

func VerifyFinalSamples(expectedReplicas []string, samples []Sample, wantValue float64, windowStart, windowEnd time.Time) ([]Result, error) {
	if len(expectedReplicas) == 0 {
		return nil, fmt.Errorf("no expected replicas configured")
	}
	results := make([]Result, 0, len(expectedReplicas))
	seen := map[string]Result{}
	expected := map[string]struct{}{}
	for _, replica := range expectedReplicas {
		if replica == "" {
			return nil, fmt.Errorf("empty replica name")
		}
		if _, duplicate := expected[replica]; duplicate {
			return nil, fmt.Errorf("duplicate expected replica %q", replica)
		}
		expected[replica] = struct{}{}
	}
	for _, sample := range samples {
		if _, ok := expected[sample.Replica]; !ok {
			continue
		}
		if sample.Stale || sample.Value != wantValue || sample.Timestamp.Before(windowStart) || !sample.Timestamp.Before(windowEnd) {
			continue
		}
		if previous, duplicate := seen[sample.Replica]; duplicate {
			return nil, fmt.Errorf("duplicate final sample for replica %q targets %q and %q", sample.Replica, previous.Target, sample.Target)
		}
		seen[sample.Replica] = Result{Replica: sample.Replica, Target: sample.Target, Timestamp: sample.Timestamp, Value: sample.Value}
	}
	for replica := range expected {
		result, ok := seen[replica]
		if !ok {
			return nil, fmt.Errorf("missing final sample for replica %q", replica)
		}
		results = append(results, result)
	}
	sort.Slice(results, func(left, right int) bool { return results[left].Replica < results[right].Replica })
	return results, nil
}
