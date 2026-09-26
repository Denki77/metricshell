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
}

func New(registry Registry, bridge Installer) (*Finalizer, error) {
	if registry == nil || bridge == nil {
		return nil, errors.New("managed finalizer dependencies are required")
	}
	return &Finalizer{registry: registry, bridge: bridge}, nil
}

func (finalizer *Finalizer) FreezeAndInstall(ctx context.Context) (Result, error) {
	winner := false
	finalizer.once.Do(func() {
		winner = true
		frozen, _ := finalizer.registry.Freeze()
		finalizer.result.RegistryGeneration = frozen.Generation
		finalizer.result.Install, finalizer.err = finalizer.bridge.Install(ctx)
	})
	result := finalizer.result
	result.Winner = winner
	return result, finalizer.err
}
