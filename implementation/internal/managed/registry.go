package managed

import "sync"

type Snapshot struct {
	Generation uint64
	Families   map[string]Family
}

type Registry struct {
	mu         sync.RWMutex
	generation uint64
	model      *Model
}

func NewRegistry() *Registry {
	registry, _ := NewRegistryWithLimits(DefaultLimits())
	return registry
}

func NewRegistryWithLimits(limits Limits) (*Registry, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	return &Registry{model: NewModelWithLimits(limits)}, nil
}

func (registry *Registry) Declare(descriptor Descriptor) (uint64, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	_, existed := registry.model.families[descriptor.Name]
	if !existed && registry.generation == ^uint64(0) {
		return registry.generation, reject(ReasonOverflow)
	}
	if err := registry.model.Declare(descriptor); err != nil {
		return registry.generation, err
	}
	if !existed {
		registry.generation++
	}
	return registry.generation, nil
}

func (registry *Registry) Apply(operation Operation) (uint64, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if registry.generation == ^uint64(0) {
		return registry.generation, reject(ReasonOverflow)
	}
	if err := registry.model.Apply(operation); err != nil {
		return registry.generation, err
	}
	registry.generation++
	return registry.generation, nil
}

func (registry *Registry) ApplyBatch(operations []Operation) (uint64, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if registry.generation == ^uint64(0) {
		return registry.generation, reject(ReasonOverflow)
	}
	if err := registry.model.ApplyBatch(operations); err != nil {
		return registry.generation, err
	}
	registry.generation++
	return registry.generation, nil
}

func (registry *Registry) Read() Snapshot {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	return Snapshot{Generation: registry.generation, Families: registry.model.Families()}
}
