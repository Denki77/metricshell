package managedfinalize

import (
	"context"
	"errors"
	"sync"

	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedbridge"
)

type Registry interface {
	Freeze() (managed.Snapshot, bool)
}

type Installer interface {
	Install(context.Context) (managedbridge.Result, error)
}

type Observer interface {
	Materialized(bool, error)
	Frozen(bool, uint64)
	FinalInstalled(string, uint64)
}

type noopObserver struct{}

func (noopObserver) Materialized(bool, error)      {}
func (noopObserver) Frozen(bool, uint64)           {}
func (noopObserver) FinalInstalled(string, uint64) {}

type Result struct {
	RegistryGeneration uint64
	Install            managedbridge.Result
	Winner             bool
}

type Finalizer struct {
	registry Registry
	bridge   Installer
	once     sync.Once
	result   Result
	err      error
	observer Observer
}

func New(registry Registry, bridge Installer) (*Finalizer, error) {
	return NewObserved(registry, bridge, nil)
}

func NewObserved(registry Registry, bridge Installer, observer Observer) (*Finalizer, error) {
	if registry == nil || bridge == nil {
		return nil, errors.New("managed finalizer dependencies are required")
	}
	if observer == nil {
		observer = noopObserver{}
	}
	return &Finalizer{registry: registry, bridge: bridge, observer: observer}, nil
}

func (finalizer *Finalizer) FreezeAndInstall(ctx context.Context) (Result, error) {
	winner := false
	finalizer.once.Do(func() {
		winner = true
		frozen, freezeWinner := finalizer.registry.Freeze()
		finalizer.result.RegistryGeneration = frozen.Generation
		finalizer.observer.Frozen(freezeWinner, frozen.Generation)
		finalizer.result.Install, finalizer.err = finalizer.bridge.Install(ctx)
		finalizer.observer.Materialized(finalizer.result.Install.CacheHit, finalizer.err)
		outcome := "error"
		if finalizer.err == nil && finalizer.result.Install.Core.Outcome == "accepted" {
			outcome = "accepted"
		} else if finalizer.err == nil {
			outcome = "rejected"
		}
		finalizer.observer.FinalInstalled(outcome, frozen.Generation)
	})
	result := finalizer.result
	result.Winner = winner
	return result, finalizer.err
}
