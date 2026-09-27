package managedobserve

import (
	"context"
	"sync/atomic"

	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedprotocol"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
)

type Submitter interface {
	Submit(context.Context, managed.Mutation) managed.Result
	State() managed.OwnerState
}

type Observer struct {
	metrics     *selfmetric.Registry
	logger      *diagnostic.Logger
	registry    *managed.Registry
	owner       Submitter
	state       func() string
	connections atomic.Int64
}

func New(metrics *selfmetric.Registry, logger *diagnostic.Logger, registry *managed.Registry, owner Submitter, state func() string) *Observer {
	return &Observer{metrics: metrics, logger: logger, registry: registry, owner: owner, state: state}
}

func (observer *Observer) Initialize() {
	owner := observer.owner.State()
	_ = observer.metrics.SetGauge(selfmetric.ManagedQueueCapacity, nil, float64(owner.Capacity))
	observer.project()
}

func (observer *Observer) Submit(ctx context.Context, mutation managed.Mutation) managed.Result {
	before := observer.owner.State()
	_ = observer.metrics.SetGauge(selfmetric.ManagedQueueDepth, nil, float64(before.Depth))
	result := observer.owner.Submit(ctx, mutation)
	_ = observer.metrics.AddCounter(selfmetric.ManagedOperationsTotal, map[string]string{"outcome": string(result.Outcome)}, 1)
	switch result.Outcome {
	case managed.OutcomeRejected:
		class := rejectionClass(result.Reason)
		_ = observer.metrics.AddCounter(selfmetric.ManagedRejectionsTotal, map[string]string{"class": class}, 1)
	case managed.OutcomeOverloaded:
		_ = observer.metrics.AddCounter(selfmetric.ManagedRejectionsTotal, map[string]string{"class": "overload"}, 1)
	case managed.OutcomeClosed:
		_ = observer.metrics.AddCounter(selfmetric.ManagedRejectionsTotal, map[string]string{"class": "late"}, 1)
	}
	after := observer.owner.State()
	_ = observer.metrics.SetGauge(selfmetric.ManagedQueueDepth, nil, float64(after.Depth))
	observer.project()
	_ = observer.logger.WriteManagedOperation(string(result.Outcome), rejectionClass(result.Reason), result.Generation, observer.state())
	return result
}

func (observer *Observer) ProtocolRejected(code managedprotocol.Code) {
	_ = observer.metrics.AddCounter(selfmetric.ManagedProtocolTotal, map[string]string{"code": string(code)}, 1)
	_ = observer.metrics.AddCounter(selfmetric.ManagedRejectionsTotal, map[string]string{"class": "protocol"}, 1)
	_ = observer.logger.WriteManagedProtocolRejected(string(code), observer.state())
}

func (observer *Observer) Connection(delta int) {
	active := observer.connections.Add(int64(delta))
	_ = observer.metrics.SetGauge(selfmetric.IngestionConnections, map[string]string{"transport": "managed"}, float64(active))
}

func (observer *Observer) Materialized(cacheHit bool, err error) {
	outcome := "built"
	if err != nil {
		outcome = "error"
	} else if cacheHit {
		outcome = "cache_hit"
	}
	_ = observer.metrics.AddCounter(selfmetric.ManagedMaterializations, map[string]string{"outcome": outcome}, 1)
	_ = observer.logger.WriteManagedMaterialized(outcome, observer.state())
}

func (observer *Observer) Frozen(winner bool, generation uint64) {
	outcome := "duplicate"
	if winner {
		outcome = "winner"
	}
	_ = observer.metrics.AddCounter(selfmetric.ManagedFreezes, map[string]string{"outcome": outcome}, 1)
	_ = observer.logger.WriteManagedFrozen(outcome, generation, observer.state())
}

func (observer *Observer) FinalInstalled(outcome string, generation uint64) {
	_ = observer.metrics.AddCounter(selfmetric.ManagedFinalInstalls, map[string]string{"outcome": outcome}, 1)
	_ = observer.logger.WriteManagedFinalInstalled(outcome, generation, observer.state())
}

func (observer *Observer) project() {
	state := observer.registry.State()
	_ = observer.metrics.SetGauge(selfmetric.ManagedGeneration, nil, float64(state.Generation))
	_ = observer.metrics.SetGauge(selfmetric.ManagedFamilies, nil, float64(state.Families))
	_ = observer.metrics.SetGauge(selfmetric.ManagedSeries, nil, float64(state.Series))
}

func rejectionClass(reason managed.Reason) string {
	switch reason {
	case managed.ReasonFamilyLimit, managed.ReasonSeriesLimit, managed.ReasonLabelLimit, managed.ReasonBucketLimit,
		managed.ReasonBatchLimit, managed.ReasonNameLimit, managed.ReasonValueLimit, managed.ReasonHelpLimit:
		return "resource"
	case managed.ReasonLate:
		return "late"
	default:
		return "semantic"
	}
}
