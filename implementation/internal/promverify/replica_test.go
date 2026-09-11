package promverify

import (
	"strings"
	"testing"
	"time"
)

func TestVerifyFinalSamplesRequiresEachReplicaIndependently(t *testing.T) {
	t.Parallel()

	start := time.Unix(100, 0)
	end := time.Unix(200, 0)
	results, err := VerifyFinalSamples([]string{"prometheus-0", "prometheus-1"}, []Sample{
		{Replica: "prometheus-0", Target: "job-a", Timestamp: time.Unix(150, 0), Value: 42},
		{Replica: "prometheus-1", Target: "job-a", Timestamp: time.Unix(151, 0), Value: 42},
	}, 42, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Replica != "prometheus-0" || results[1].Replica != "prometheus-1" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestVerifyFinalSamplesRejectsAggregateOnlyEvidence(t *testing.T) {
	t.Parallel()

	start := time.Unix(100, 0)
	end := time.Unix(200, 0)
	_, err := VerifyFinalSamples([]string{"prometheus-0", "prometheus-1"}, []Sample{
		{Replica: "prometheus-0", Target: "job-a", Timestamp: time.Unix(150, 0), Value: 84},
	}, 42, start, end)
	requireErrorContains(t, err, "missing final sample for replica")
}

func TestVerifyFinalSamplesRejectsDuplicateAndStaleOnlySamples(t *testing.T) {
	t.Parallel()

	start := time.Unix(100, 0)
	end := time.Unix(200, 0)
	_, duplicateErr := VerifyFinalSamples([]string{"prometheus-0"}, []Sample{
		{Replica: "prometheus-0", Target: "job-a", Timestamp: time.Unix(150, 0), Value: 42},
		{Replica: "prometheus-0", Target: "job-b", Timestamp: time.Unix(151, 0), Value: 42},
	}, 42, start, end)
	requireErrorContains(t, duplicateErr, "duplicate final sample")

	_, staleErr := VerifyFinalSamples([]string{"prometheus-0"}, []Sample{
		{Replica: "prometheus-0", Target: "job-a", Timestamp: time.Unix(150, 0), Value: 42, Stale: true},
		{Replica: "prometheus-0", Target: "job-a", Timestamp: time.Unix(201, 0), Value: 42},
	}, 42, start, end)
	requireErrorContains(t, staleErr, `missing final sample for replica "prometheus-0"`)
}

func requireErrorContains(t *testing.T, err error, needle string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), needle) {
		t.Fatalf("error = %v, want containing %q", err, needle)
	}
}
