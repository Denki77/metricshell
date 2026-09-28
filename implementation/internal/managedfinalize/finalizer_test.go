package managedfinalize

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedbridge"
	"github.com/Denki77/metricshell/implementation/internal/managedmaterialize"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

type installerStub struct {
	calls  atomic.Int64
	result managedbridge.Result
	err    error
}

func (installer *installerStub) Install(context.Context) (managedbridge.Result, error) {
	installer.calls.Add(1)
	return installer.result, installer.err
}

func TestFreezeAndInstallHasOneWinnerAndRejectsLateMutations(t *testing.T) {
	registry := managed.NewRegistry()
	if _, err := registry.Declare(managed.Descriptor{Name: "jobs", Type: managed.Counter}); err != nil {
		t.Fatal(err)
	}
	installer := &installerStub{result: managedbridge.Result{
		RegistryGeneration: 1, Core: ingestion.Result{Transport: ingestion.Managed, Outcome: ingestion.Accepted, Generation: 1}, Installed: true,
	}}
	finalizer, err := New(registry, installer)
	if err != nil {
		t.Fatal(err)
	}
	const contenders = 64
	var wait sync.WaitGroup
	var winners atomic.Int64
	wait.Add(contenders)
	for index := 0; index < contenders; index++ {
		go func() {
			defer wait.Done()
			result, err := finalizer.FreezeAndInstall(context.Background())
			if err != nil || result.RegistryGeneration != 1 || result.Install.Core.Outcome != ingestion.Accepted {
				t.Errorf("result=%+v err=%v", result, err)
			}
			if result.Winner {
				winners.Add(1)
			}
		}()
	}
	wait.Wait()
	if winners.Load() != 1 || installer.calls.Load() != 1 || !registry.Frozen() {
		t.Fatalf("winners=%d installs=%d frozen=%v", winners.Load(), installer.calls.Load(), registry.Frozen())
	}
	for index := 0; index < 100; index++ {
		generation, err := registry.Apply(managed.Operation{Kind: managed.CounterAdd, Name: "jobs", Labels: map[string]string{}, Value: 1})
		if reason, ok := managed.RejectionReason(err); !ok || reason != managed.ReasonLate || generation != 1 {
			t.Fatalf("late generation=%d reason=%q err=%v", generation, reason, err)
		}
	}
}

func TestFinalInstallFailureIsStableAndAttemptedOnce(t *testing.T) {
	registry := managed.NewRegistry()
	installer := &installerStub{result: managedbridge.Result{Core: ingestion.Result{
		Transport: ingestion.Managed, Outcome: ingestion.Rejected, Reason: snapshot.ReasonPayloadLimit,
	}}, err: errors.New("injected final conversion failure")}
	finalizer, _ := New(registry, installer)
	first, firstErr := finalizer.FreezeAndInstall(context.Background())
	second, secondErr := finalizer.FreezeAndInstall(context.Background())
	if firstErr == nil || secondErr == nil || !first.Winner || second.Winner || installer.calls.Load() != 1 {
		t.Fatalf("first=%+v err=%v second=%+v err=%v calls=%d", first, firstErr, second, secondErr, installer.calls.Load())
	}
	if !registry.Frozen() {
		t.Fatal("failed final install did not retain the frozen registry")
	}
}

func TestFrozenGenerationBecomesCoreFinalSnapshot(t *testing.T) {
	registry := managed.NewRegistry()
	if _, err := registry.Declare(managed.Descriptor{Name: "jobs", Type: managed.Counter}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Apply(managed.Operation{Kind: managed.CounterAdd, Name: "jobs", Value: 7}); err != nil {
		t.Fatal(err)
	}
	cache, _ := managedmaterialize.New(registry, nil)
	holder := snapshot.NewHolder(snapshot.Zero())
	core, _ := ingestion.New(holder, snapshot.DefaultLimits(), 1, 0, nil)
	bridge, _ := managedbridge.New(cache, core)
	finalizer, _ := New(registry, bridge)

	result, err := finalizer.FreezeAndInstall(context.Background())
	if err != nil || result.RegistryGeneration != 2 || !result.Install.Installed || result.Install.Core.Outcome != ingestion.Accepted {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	final := core.CloseAndFreeze(context.Background())
	materialized, err := cache.Materialize()
	if err != nil {
		t.Fatal(err)
	}
	want, err := snapshot.Parse(materialized.Representation.Bytes(), snapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if final.Generation() != result.Install.Core.Generation || !bytes.Equal(final.Validated().Canonical(), want.Canonical()) {
		t.Fatalf("final Core generation=%d install=%d", final.Generation(), result.Install.Core.Generation)
	}
	late := core.Publish(context.Background(), ingestion.Managed, materialized.Representation.Bytes())
	if late.Outcome != ingestion.Rejected || late.Reason != snapshot.ReasonFrozen || final.Generation() != holder.Active().Generation() {
		t.Fatalf("late publish=%+v active_generation=%d", late, holder.Active().Generation())
	}
}

func TestFinalFreezeCannotBeOverwrittenByPreFreezePublication(t *testing.T) {
	registry := managed.NewRegistry()
	_, _ = registry.Declare(managed.Descriptor{Name: "depth", Type: managed.Gauge})
	_, _ = registry.Apply(managed.Operation{Kind: managed.GaugeSet, Name: "depth", Value: 1})
	encoded := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls atomic.Int64
	cache, _ := managedmaterialize.New(registry, func(value managed.Snapshot) ([]byte, error) {
		if calls.Add(1) == 1 {
			encoded <- struct{}{}
			<-release
		}
		return managedmaterialize.Encode(value)
	})
	holder := snapshot.NewHolder(snapshot.Zero())
	core, _ := ingestion.New(holder, snapshot.DefaultLimits(), 2, 0, nil)
	bridge, _ := managedbridge.New(cache, core)
	periodic := make(chan error, 1)
	go func() {
		_, err := bridge.Install(context.Background())
		periodic <- err
	}()
	<-encoded
	_, _ = registry.Apply(managed.Operation{Kind: managed.GaugeSet, Name: "depth", Value: 2})
	finalizer, _ := New(registry, bridge)
	final := make(chan error, 1)
	go func() {
		_, err := finalizer.FreezeAndInstall(context.Background())
		final <- err
	}()
	close(release)
	if err := <-periodic; err != nil {
		t.Fatal(err)
	}
	if err := <-final; err != nil {
		t.Fatal(err)
	}
	canonical := string(holder.Active().Validated().Canonical())
	if !registry.Frozen() || !strings.Contains(canonical, `"value":"2"`) || holder.Active().Generation() != 2 {
		t.Fatalf("pre-freeze publication won: generation=%d canonical=%s", holder.Active().Generation(), canonical)
	}
}
