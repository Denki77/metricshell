package managed

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
)

func TestOwnerConcurrentMutationMatrix(t *testing.T) {
	for _, publishers := range []int{1, 2, 8, 32, 128} {
		t.Run(fmt.Sprintf("publishers_%d", publishers), func(t *testing.T) {
			registry := NewRegistry()
			owner, err := NewOwner(registry, publishers*2)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			descriptor := Descriptor{Name: "jobs", Type: Counter}
			if result := owner.Submit(context.Background(), Mutation{Descriptor: &descriptor}); result.Outcome != OutcomeCommitted {
				t.Fatalf("declaration = %+v", result)
			}
			var wait sync.WaitGroup
			results := make(chan Result, publishers)
			for index := 0; index < publishers; index++ {
				wait.Add(1)
				go func() {
					defer wait.Done()
					results <- owner.Submit(context.Background(), Mutation{Operations: []Operation{{Kind: CounterAdd, Name: "jobs", Value: 1}}})
				}()
			}
			wait.Wait()
			close(results)
			for result := range results {
				if result.Outcome != OutcomeCommitted {
					t.Fatalf("result = %+v", result)
				}
			}
			if got := onlySeriesFromSnapshot(t, registry.Read(), "jobs").Value; got != float64(publishers) {
				t.Fatalf("counter = %v, want %d", got, publishers)
			}
		})
	}
}

func TestOwnerCommitOrderForGaugeHistogramAndDescriptors(t *testing.T) {
	registry := NewRegistry()
	owner, err := NewOwner(registry, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	for _, descriptor := range []Descriptor{
		{Name: "depth", Type: Gauge},
		{Name: "latency", Type: Histogram, Buckets: []float64{1, math.Inf(1)}},
	} {
		copy := descriptor
		if got := owner.Submit(context.Background(), Mutation{Descriptor: &copy}); got.Outcome != OutcomeCommitted {
			t.Fatal(got)
		}
	}
	first := owner.Submit(context.Background(), Mutation{Operations: []Operation{{Kind: GaugeSet, Name: "depth", Value: 1}}})
	second := owner.Submit(context.Background(), Mutation{Operations: []Operation{{Kind: GaugeSet, Name: "depth", Value: 2}}})
	histogram := owner.Submit(context.Background(), Mutation{Operations: []Operation{{Kind: HistogramObserve, Name: "latency", Value: 0.5}}})
	if !(first.Commit < second.Commit && second.Commit < histogram.Commit) {
		t.Fatalf("commit order = %d, %d, %d", first.Commit, second.Commit, histogram.Commit)
	}
	if got := onlySeriesFromSnapshot(t, registry.Read(), "depth").Value; got != 2 {
		t.Fatalf("gauge = %v", got)
	}
	h := onlySeriesFromSnapshot(t, registry.Read(), "latency")
	if h.Count != 1 || h.Sum != 0.5 || h.BucketCounts[0] != 1 || h.BucketCounts[1] != 1 {
		t.Fatalf("histogram = %+v", h)
	}

	compatible := Descriptor{Name: "depth", Type: Gauge}
	conflict := Descriptor{Name: "depth", Type: Counter}
	if got := owner.Submit(context.Background(), Mutation{Descriptor: &compatible}); got.Outcome != OutcomeCommitted {
		t.Fatalf("compatible declaration = %+v", got)
	}
	if got := owner.Submit(context.Background(), Mutation{Descriptor: &conflict}); got.Outcome != OutcomeRejected || got.Reason != ReasonDescriptorConflict {
		t.Fatalf("conflicting declaration = %+v", got)
	}
}

func TestOwnerOverloadCancellationAndClose(t *testing.T) {
	registry := NewRegistry()
	owner, err := NewOwner(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := owner.Submit(cancelled, Mutation{}); got.Outcome != OutcomeCancelled {
		t.Fatalf("cancelled = %+v", got)
	}
	owner.Close()
	if got := owner.Submit(context.Background(), Mutation{}); got.Outcome != OutcomeClosed {
		t.Fatalf("closed = %+v", got)
	}
	if got := registry.Read(); got.Generation != 0 || len(got.Families) != 0 {
		t.Fatalf("non-admitted requests mutated registry: %+v", got)
	}
	for _, capacity := range []int{-1, 0, 1025} {
		if _, err := NewOwner(NewRegistry(), capacity); err == nil {
			t.Fatalf("capacity %d accepted", capacity)
		}
	}
}

func TestOwnerQueueCapacityIsExact(t *testing.T) {
	registry := NewRegistry()
	start := make(chan struct{})
	owner, err := newOwner(registry, 1, start)
	if err != nil {
		t.Fatal(err)
	}
	first := make(chan Result, 1)
	go func() {
		first <- owner.Submit(context.Background(), Mutation{Operations: []Operation{{Kind: GaugeSet, Name: "missing", Value: 1}}})
	}()
	for owner.State().Depth != 1 {
	}
	if got := owner.Submit(context.Background(), Mutation{}); got.Outcome != OutcomeOverloaded || got.Generation != 0 {
		t.Fatalf("overload = %+v", got)
	}
	close(start)
	if got := <-first; got.Outcome != OutcomeRejected {
		t.Fatalf("admitted result = %+v", got)
	}
	owner.Close()
	if got := registry.Read(); got.Generation != 0 {
		t.Fatalf("overload changed generation: %+v", got)
	}
}
