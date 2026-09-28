package managedbridge

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedmaterialize"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

func TestBridgeUsesCoreValidationAndAtomicHolder(t *testing.T) {
	registry := managed.NewRegistry()
	if _, err := registry.Declare(managed.Descriptor{Name: "jobs", Help: "Jobs.", Type: managed.Counter}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Apply(managed.Operation{Kind: managed.CounterAdd, Name: "jobs", Labels: map[string]string{}, Value: 3}); err != nil {
		t.Fatal(err)
	}
	cache, _ := managedmaterialize.New(registry, nil)
	holder := snapshot.NewHolder(snapshot.Zero())
	core, err := ingestion.New(holder, snapshot.DefaultLimits(), 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	bridge, _ := New(cache, core)
	result, err := bridge.Install(context.Background())
	if err != nil || !result.Installed || result.Core.Outcome != ingestion.Accepted || result.Core.Transport != ingestion.Managed {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	materialized, _ := cache.Materialize()
	expected, err := snapshot.Parse(materialized.Representation.Bytes(), snapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(holder.Active().Validated().Canonical(), expected.Canonical()) {
		t.Fatal("Core active state differs from the complete managed candidate")
	}
	again, err := bridge.Install(context.Background())
	if err != nil || again.Installed || holder.Active().Generation() != 1 {
		t.Fatalf("unchanged generation reinstalled: result=%+v core_generation=%d", again, holder.Active().Generation())
	}
}

func TestBridgeFailuresPreservePreviousCoreState(t *testing.T) {
	baseline, err := snapshot.Parse([]byte(`{"schema_version":1,"families":[{"name":"baseline","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`), snapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	holder := snapshot.NewHolder(baseline)
	core, _ := ingestion.New(holder, snapshot.DefaultLimits(), 1, 0, nil)
	registry := managed.NewRegistry()
	if _, err := registry.Declare(managed.Descriptor{Name: "next", Type: managed.Gauge}); err != nil {
		t.Fatal(err)
	}

	t.Run("conversion", func(t *testing.T) {
		cache, _ := managedmaterialize.New(registry, func(managed.Snapshot) ([]byte, error) { return nil, errors.New("conversion") })
		bridge, _ := New(cache, core)
		if _, err := bridge.Install(context.Background()); err == nil {
			t.Fatal("conversion error accepted")
		}
	})
	t.Run("validation", func(t *testing.T) {
		cache, _ := managedmaterialize.New(registry, func(managed.Snapshot) ([]byte, error) { return []byte(`{"schema_version":2,"families":[]}`), nil })
		bridge, _ := New(cache, core)
		result, err := bridge.Install(context.Background())
		if err != nil || result.Core.Outcome != ingestion.Rejected || result.Core.Reason != snapshot.ReasonSchemaVersion {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	if holder.Active().Generation() != 0 || !bytes.Equal(holder.Active().Validated().Canonical(), baseline.Canonical()) {
		t.Fatal("failed managed candidate replaced prior Core state")
	}
}

func TestBridgeSerializesConcurrentInstallForOneRegistryGeneration(t *testing.T) {
	registry := managed.NewRegistry()
	if _, err := registry.Declare(managed.Descriptor{Name: "value", Type: managed.Gauge}); err != nil {
		t.Fatal(err)
	}
	cache, _ := managedmaterialize.New(registry, nil)
	holder := snapshot.NewHolder(snapshot.Zero())
	core, _ := ingestion.New(holder, snapshot.DefaultLimits(), 4, 4, nil)
	bridge, _ := New(cache, core)
	const callers = 32
	var wait sync.WaitGroup
	wait.Add(callers)
	for index := 0; index < callers; index++ {
		go func() {
			defer wait.Done()
			result, err := bridge.Install(context.Background())
			if err != nil || result.Core.Outcome != ingestion.Accepted {
				t.Errorf("result=%+v err=%v", result, err)
			}
		}()
	}
	wait.Wait()
	if holder.Active().Generation() != 1 {
		t.Fatalf("Core generation=%d, want one atomic install", holder.Active().Generation())
	}
}

func TestOversizedManagedCandidatePreservesPreviousCoreSnapshot(t *testing.T) {
	baseline, err := snapshot.Parse([]byte(`{"schema_version":1,"families":[{"name":"baseline","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`), snapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	holder := snapshot.NewHolder(baseline)
	limits := snapshot.DefaultLimits()
	limits.SnapshotBytes = len(baseline.Canonical())
	core, _ := ingestion.New(holder, limits, 1, 0, nil)
	registry := managed.NewRegistry()
	if _, err := registry.Declare(managed.Descriptor{Name: "large_metric", Help: string(bytes.Repeat([]byte("x"), 256)), Type: managed.Gauge}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Apply(managed.Operation{Kind: managed.GaugeSet, Name: "large_metric", Value: 2}); err != nil {
		t.Fatal(err)
	}
	cache, _ := managedmaterialize.New(registry, nil)
	bridge, _ := New(cache, core)
	result, err := bridge.Install(context.Background())
	if err != nil || result.Core.Outcome != ingestion.Rejected || result.Core.Reason != snapshot.ReasonPayloadLimit || result.Installed {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if holder.Active().Generation() != 0 || !bytes.Equal(holder.Active().Validated().Canonical(), baseline.Canonical()) {
		t.Fatal("oversized complete candidate partially replaced previous Core state")
	}
}

func TestBridgeCoalescesRegistryGenerations(t *testing.T) {
	registry := managed.NewRegistry()
	_, _ = registry.Declare(managed.Descriptor{Name: "depth", Type: managed.Gauge})
	_, _ = registry.Apply(managed.Operation{Kind: managed.GaugeSet, Name: "depth", Value: 1})
	_, _ = registry.Apply(managed.Operation{Kind: managed.GaugeSet, Name: "depth", Value: 2})
	cache, _ := managedmaterialize.New(registry, nil)
	holder := snapshot.NewHolder(snapshot.Zero())
	core, _ := ingestion.New(holder, snapshot.DefaultLimits(), 1, 0, nil)
	bridge, _ := New(cache, core)
	result, err := bridge.Install(context.Background())
	if err != nil || !result.Installed || result.RegistryGeneration != 3 || holder.Active().Generation() != 1 {
		t.Fatalf("coalesced install=%+v core_generation=%d err=%v", result, holder.Active().Generation(), err)
	}
}
