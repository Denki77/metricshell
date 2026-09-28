package managedbridge

import (
	"context"
	"errors"
	"sync"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/managedmaterialize"
)

type Materializer interface {
	Materialize() (managedmaterialize.Result, error)
}

type Publisher interface {
	Publish(context.Context, ingestion.Transport, []byte) ingestion.Result
}

type Result struct {
	RegistryGeneration uint64
	CacheHit           bool
	Installed          bool
	Core               ingestion.Result
}

type Bridge struct {
	materializer Materializer
	core         Publisher
	mu           sync.Mutex
	hasInstalled bool
	lastRegistry uint64
	lastCore     ingestion.Result
}

func New(materializer Materializer, core Publisher) (*Bridge, error) {
	if materializer == nil || core == nil {
		return nil, errors.New("managed bridge dependencies are required")
	}
	return &Bridge{materializer: materializer, core: core}, nil
}

func (bridge *Bridge) Install(ctx context.Context) (Result, error) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	materialized, err := bridge.materializer.Materialize()
	if err != nil {
		return Result{}, err
	}
	result := Result{RegistryGeneration: materialized.Representation.Generation, CacheHit: materialized.Hit}
	if bridge.hasInstalled && bridge.lastRegistry == result.RegistryGeneration {
		result.Core = bridge.lastCore
		return result, nil
	}
	result.Core = bridge.core.Publish(ctx, ingestion.Managed, materialized.Representation.Bytes())
	if result.Core.Outcome == ingestion.Accepted {
		result.Installed = true
		bridge.hasInstalled = true
		bridge.lastRegistry = result.RegistryGeneration
		bridge.lastCore = result.Core
	}
	return result, nil
}
