package snapshot

import (
	"sync"
	"sync/atomic"
)

// Holder owns the linear installation order and the last valid complete snapshot.
// Readers load one immutable ActiveSnapshot without participating in writer locking.
type Holder struct {
	writeMu  sync.Mutex
	active   atomic.Pointer[ActiveSnapshot]
	bindings map[string]MetricType
	frozen   bool
}

// NewHolder starts at generation zero with the supplied validated snapshot.
func NewHolder(initial ValidatedSnapshot) *Holder {
	holder := &Holder{bindings: make(map[string]MetricType)}
	for _, family := range initial.families {
		holder.bindings[family.name] = family.typeID
	}
	active := NewActive(initial, 0)
	holder.active.Store(&active)
	return holder
}

// Active returns a caller-owned view of exactly one installed generation.
func (holder *Holder) Active() ActiveSnapshot {
	active := holder.active.Load()
	return NewActive(active.validated, active.generation)
}

// Install atomically replaces the complete state and assigns its generation.
func (holder *Holder) Install(validated ValidatedSnapshot) (ActiveSnapshot, error) {
	holder.writeMu.Lock()
	defer holder.writeMu.Unlock()

	current := holder.active.Load()
	if holder.frozen {
		return NewActive(current.validated, current.generation), Reject(ReasonFrozen)
	}
	for _, family := range validated.families {
		if bound, exists := holder.bindings[family.name]; exists && bound != family.typeID {
			return NewActive(current.validated, current.generation), Reject(ReasonTypeConflict)
		}
	}
	if current.generation == ^uint64(0) {
		return NewActive(current.validated, current.generation), Reject(ReasonInternal)
	}

	for _, family := range validated.families {
		holder.bindings[family.name] = family.typeID
	}
	next := NewActive(validated, current.generation+1)
	holder.active.Store(&next)
	return NewActive(next.validated, next.generation), nil
}

// Freeze closes installation and returns the immutable final generation.
func (holder *Holder) Freeze() ActiveSnapshot {
	holder.writeMu.Lock()
	defer holder.writeMu.Unlock()
	holder.frozen = true
	current := holder.active.Load()
	return NewActive(current.validated, current.generation)
}

func (holder *Holder) Frozen() bool {
	holder.writeMu.Lock()
	defer holder.writeMu.Unlock()
	return holder.frozen
}
