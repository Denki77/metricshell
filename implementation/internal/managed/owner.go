package managed

import (
	"context"
	"sync"
)

type Mutation struct {
	Descriptor *Descriptor
	Operations []Operation
}

type Outcome string

const (
	OutcomeCommitted  Outcome = "committed"
	OutcomeRejected   Outcome = "rejected"
	OutcomeOverloaded Outcome = "overloaded"
	OutcomeCancelled  Outcome = "cancelled"
	OutcomeUnknown    Outcome = "unknown"
	OutcomeClosed     Outcome = "closed"
)

type Result struct {
	Outcome    Outcome
	Generation uint64
	Commit     uint64
	Reason     Reason
}

type OwnerState struct {
	Depth    int
	Capacity int
	Closed   bool
}

type ownerRequest struct {
	mutation Mutation
	result   chan Result
}

type Owner struct {
	registry *Registry
	queue    chan ownerRequest
	done     chan struct{}
	start    <-chan struct{}

	mu     sync.RWMutex
	closed bool
}

func NewOwner(registry *Registry, capacity int) (*Owner, error) {
	start := make(chan struct{})
	close(start)
	return newOwner(registry, capacity, start)
}

func newOwner(registry *Registry, capacity int, start <-chan struct{}) (*Owner, error) {
	if registry == nil || capacity < 1 || capacity > 1024 {
		return nil, reject(ReasonInvalidNumber)
	}
	owner := &Owner{
		registry: registry,
		queue:    make(chan ownerRequest, capacity),
		done:     make(chan struct{}),
		start:    start,
	}
	go owner.run()
	return owner, nil
}

func (owner *Owner) Submit(ctx context.Context, mutation Mutation) Result {
	if err := ctx.Err(); err != nil {
		return Result{Outcome: OutcomeCancelled, Generation: owner.registry.Read().Generation}
	}
	request := ownerRequest{mutation: mutation, result: make(chan Result, 1)}
	owner.mu.RLock()
	if owner.closed {
		owner.mu.RUnlock()
		return Result{Outcome: OutcomeClosed, Generation: owner.registry.Read().Generation, Reason: ReasonLate}
	}
	select {
	case owner.queue <- request:
		owner.mu.RUnlock()
		select {
		case result := <-request.result:
			return result
		case <-ctx.Done():
			return Result{Outcome: OutcomeUnknown, Generation: owner.registry.Read().Generation}
		}
	default:
		owner.mu.RUnlock()
		return Result{Outcome: OutcomeOverloaded, Generation: owner.registry.Read().Generation}
	}
}

func (owner *Owner) State() OwnerState {
	owner.mu.RLock()
	defer owner.mu.RUnlock()
	return OwnerState{Depth: len(owner.queue), Capacity: cap(owner.queue), Closed: owner.closed}
}

func (owner *Owner) Close() {
	owner.CloseAdmission()
	<-owner.done
}

func (owner *Owner) CloseAdmission() {
	owner.mu.Lock()
	if !owner.closed {
		owner.closed = true
		close(owner.queue)
	}
	owner.mu.Unlock()
}

func (owner *Owner) Drain(ctx context.Context) bool {
	select {
	case <-owner.done:
		return true
	case <-ctx.Done():
		return false
	}
}

func (owner *Owner) run() {
	defer close(owner.done)
	<-owner.start
	var commit uint64
	for request := range owner.queue {
		generation, err := owner.apply(request.mutation)
		result := Result{Generation: generation}
		if err != nil {
			result.Outcome = OutcomeRejected
			result.Reason, _ = RejectionReason(err)
		} else {
			commit++
			result.Outcome = OutcomeCommitted
			result.Commit = commit
		}
		request.result <- result
	}
}

func (owner *Owner) apply(mutation Mutation) (uint64, error) {
	if mutation.Descriptor != nil {
		if len(mutation.Operations) != 0 {
			return owner.registry.Read().Generation, reject(ReasonUnsupported)
		}
		return owner.registry.Declare(*mutation.Descriptor)
	}
	if len(mutation.Operations) == 1 {
		return owner.registry.Apply(mutation.Operations[0])
	}
	return owner.registry.ApplyBatch(mutation.Operations)
}
