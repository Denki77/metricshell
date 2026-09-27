package managedvalidation

import (
	"context"
	"sync"
	"testing"

	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedmaterialize"
)

func TestProductionPolicyAndConcurrencyMatrix(t *testing.T) {
	limits := managed.Limits{
		Families: 2, Series: 2, Labels: 1, Buckets: 2, Batch: 2,
		MetricNameBytes: 32, LabelNameBytes: 32, LabelValueBytes: 32, HelpBytes: 64,
	}
	registry, err := managed.NewRegistryWithLimits(limits)
	if err != nil {
		t.Fatal(err)
	}
	owner, _ := managed.NewOwner(registry, 64)
	defer owner.Close()
	if result := owner.Submit(context.Background(), managed.Mutation{Descriptor: &managed.Descriptor{Name: "jobs", Type: managed.Counter}}); result.Outcome != managed.OutcomeCommitted {
		t.Fatalf("declare=%+v", result)
	}

	const publishers = 32
	var wait sync.WaitGroup
	wait.Add(publishers)
	for index := 0; index < publishers; index++ {
		go func() {
			defer wait.Done()
			result := owner.Submit(context.Background(), managed.Mutation{Operations: []managed.Operation{{Kind: managed.CounterAdd, Name: "jobs", Value: 1}}})
			if result.Outcome != managed.OutcomeCommitted {
				t.Errorf("publish=%+v", result)
			}
		}()
	}
	wait.Wait()
	committed := registry.Read()
	if committed.Generation != publishers+1 {
		t.Fatalf("generation=%d", committed.Generation)
	}

	assertRejectedWithoutMutation(t, registry, managed.Descriptor{Name: "latency", Type: managed.Histogram, Buckets: []float64{1, 2, 3}}, managed.ReasonBucketLimit)
	cache, _ := managedmaterialize.New(registry, nil)
	first, err := cache.Materialize()
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 32; index++ {
		current, materializeErr := cache.Materialize()
		if materializeErr != nil || !current.Hit || string(current.Representation.Bytes()) != string(first.Representation.Bytes()) {
			t.Fatalf("immutable cache result=%+v err=%v", current, materializeErr)
		}
	}
	registry.Freeze()
	if generation, lateErr := registry.Apply(managed.Operation{Kind: managed.CounterAdd, Name: "jobs", Value: 1}); generation != committed.Generation || !isReason(lateErr, managed.ReasonLate) {
		t.Fatalf("late generation=%d error=%v", generation, lateErr)
	}
	if fresh := managed.NewRegistry().Read(); fresh.Generation != 0 || len(fresh.Families) != 0 {
		t.Fatalf("restart epoch=%+v", fresh)
	}
}

func BenchmarkProductionOwnerAndMaterialization(b *testing.B) {
	registry := managed.NewRegistry()
	owner, _ := managed.NewOwner(registry, 1024)
	defer owner.Close()
	_ = owner.Submit(context.Background(), managed.Mutation{Descriptor: &managed.Descriptor{Name: "jobs", Type: managed.Counter}})
	cache, _ := managedmaterialize.New(registry, nil)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result := owner.Submit(context.Background(), managed.Mutation{Operations: []managed.Operation{{Kind: managed.CounterAdd, Name: "jobs", Value: 1}}})
		if result.Outcome != managed.OutcomeCommitted {
			b.Fatal(result)
		}
		if _, err := cache.Materialize(); err != nil {
			b.Fatal(err)
		}
	}
}

func assertRejectedWithoutMutation(t *testing.T, registry *managed.Registry, descriptor managed.Descriptor, reason managed.Reason) {
	t.Helper()
	before := registry.Read()
	generation, err := registry.Declare(descriptor)
	if generation != before.Generation || !isReason(err, reason) || registry.Read().Generation != before.Generation {
		t.Fatalf("generation=%d error=%v", generation, err)
	}
}

func isReason(err error, want managed.Reason) bool {
	reason, ok := managed.RejectionReason(err)
	return ok && reason == want
}
