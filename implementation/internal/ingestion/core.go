package ingestion

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

type Transport string

const (
	File Transport = "file"
	Unix Transport = "unix"
	HTTP Transport = "http"
)

var Transports = [...]Transport{File, Unix, HTTP}

type Outcome string

const (
	Accepted      Outcome = "accepted"
	Rejected      Outcome = "rejected"
	Busy          Outcome = "busy"
	Timeout       Outcome = "timeout"
	InternalError Outcome = "internal_error"
)

var Outcomes = [...]Outcome{Accepted, Rejected, Busy, Timeout, InternalError}

// TransportFailure describes failures before a complete candidate reaches Core.
// Candidate rejection reasons remain owned by snapshot.Reason.
type TransportFailure string

const (
	FailureNone               TransportFailure = ""
	FailureIO                 TransportFailure = "io"
	FailureMalformed          TransportFailure = "malformed"
	FailureProtocolVersion    TransportFailure = "protocol_version"
	FailureFrameLimit         TransportFailure = "frame_limit"
	FailurePartLimit          TransportFailure = "part_limit"
	FailureDuplicatePart      TransportFailure = "duplicate_part"
	FailureMissingPart        TransportFailure = "missing_part"
	FailureTransactionInvalid TransportFailure = "transaction_invalid"
	FailureTransactionExpired TransportFailure = "transaction_expired"
	FailureWireLimit          TransportFailure = "wire_limit"
	FailureEncoding           TransportFailure = "encoding"
	FailureMediaType          TransportFailure = "media_type"
	FailureMethod             TransportFailure = "method"
)

var TransportFailures = [...]TransportFailure{
	FailureIO, FailureMalformed, FailureProtocolVersion, FailureFrameLimit, FailurePartLimit,
	FailureDuplicatePart, FailureMissingPart, FailureTransactionInvalid, FailureTransactionExpired,
	FailureWireLimit, FailureEncoding, FailureMediaType, FailureMethod,
}

type Result struct {
	Transport  Transport
	Outcome    Outcome
	Generation uint64
	Reason     snapshot.Reason
}

type Publisher interface {
	Publish(context.Context, Transport, []byte) Result
}

type Observer interface {
	Started(Transport)
	Completed(Result, snapshot.ActiveSnapshot, time.Duration)
}

type noopObserver struct{}

func (noopObserver) Started(Transport)                                        {}
func (noopObserver) Completed(Result, snapshot.ActiveSnapshot, time.Duration) {}

type Parser func([]byte, snapshot.Limits) (snapshot.ValidatedSnapshot, error)

type Core struct {
	holder     *snapshot.Holder
	limits     snapshot.Limits
	parser     Parser
	observer   Observer
	now        func() time.Time
	executing  chan struct{}
	admissions chan struct{}
}

func New(holder *snapshot.Holder, limits snapshot.Limits, concurrent, pending int, observer Observer) (*Core, error) {
	return NewWithParser(holder, limits, concurrent, pending, observer, snapshot.Parse, time.Now)
}

func NewWithParser(holder *snapshot.Holder, limits snapshot.Limits, concurrent, pending int, observer Observer, parser Parser, now func() time.Time) (*Core, error) {
	if holder == nil || parser == nil || now == nil || concurrent < 1 || pending < 0 {
		return nil, errors.New("invalid ingestion configuration")
	}
	if observer == nil {
		observer = noopObserver{}
	}
	return &Core{
		holder: holder, limits: limits, parser: parser, observer: observer, now: now,
		executing: make(chan struct{}, concurrent), admissions: make(chan struct{}, concurrent+pending),
	}, nil
}

func (core *Core) Publish(ctx context.Context, transport Transport, candidate []byte) Result {
	result := Result{Transport: transport}
	if !validTransport(transport) {
		result.Outcome, result.Reason = InternalError, snapshot.ReasonInternal
		return result
	}
	if contextExpired(ctx) {
		result.Outcome = Timeout
		return result
	}
	select {
	case core.admissions <- struct{}{}:
		defer func() { <-core.admissions }()
	default:
		result.Outcome = Busy
		return result
	}
	select {
	case core.executing <- struct{}{}:
		defer func() { <-core.executing }()
	case <-ctx.Done():
		result.Outcome = Timeout
		return result
	}

	started := core.now()
	core.observer.Started(transport)
	active := core.holder.Active()
	defer func() { core.observer.Completed(result, active, core.now().Sub(started)) }()

	validated, err := core.parser(candidate, core.limits)
	if err != nil {
		if reason, ok := snapshot.RejectionReason(err); ok {
			result.Outcome, result.Reason = Rejected, reason
			return result
		}
		result.Outcome, result.Reason = InternalError, snapshot.ReasonInternal
		return result
	}
	if contextExpired(ctx) {
		result.Outcome = Timeout
		return result
	}
	active, err = core.holder.Install(validated)
	if err != nil {
		if reason, ok := snapshot.RejectionReason(err); ok {
			result.Outcome, result.Reason = Rejected, reason
			return result
		}
		result.Outcome, result.Reason = InternalError, snapshot.ReasonInternal
		return result
	}
	result.Outcome, result.Generation = Accepted, active.Generation()
	return result
}

func validTransport(transport Transport) bool {
	for _, candidate := range Transports {
		if candidate == transport {
			return true
		}
	}
	return false
}

func contextExpired(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// MetricsObserver projects shared Core outcomes into the bounded self-metrics registry.
type MetricsObserver struct {
	mu       sync.Mutex
	registry *selfmetric.Registry
	now      func() time.Time
	inflight map[Transport]int
}

func NewMetricsObserver(registry *selfmetric.Registry, now func() time.Time) (*MetricsObserver, error) {
	if registry == nil || now == nil {
		return nil, errors.New("invalid metrics observer")
	}
	return &MetricsObserver{registry: registry, now: now, inflight: make(map[Transport]int)}, nil
}

func (observer *MetricsObserver) Started(transport Transport) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.inflight[transport]++
	_ = observer.registry.SetGauge(selfmetric.IngestionInflight, map[string]string{"transport": string(transport)}, float64(observer.inflight[transport]))
}

func (observer *MetricsObserver) Completed(result Result, active snapshot.ActiveSnapshot, _ time.Duration) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.inflight[result.Transport]--
	labels := map[string]string{"transport": string(result.Transport)}
	_ = observer.registry.SetGauge(selfmetric.IngestionInflight, labels, float64(observer.inflight[result.Transport]))
	_ = observer.registry.AddCounter(selfmetric.SnapshotPublicationsTotal, map[string]string{"transport": string(result.Transport), "outcome": string(result.Outcome)}, 1)
	if result.Outcome == Rejected {
		_ = observer.registry.AddCounter(selfmetric.SnapshotRejectionsTotal, map[string]string{"transport": string(result.Transport), "reason": string(result.Reason)}, 1)
	}
	if result.Outcome == Accepted {
		observer.registry.SetActiveSnapshot(active)
		_ = observer.registry.SetGauge(selfmetric.IngestionLastSuccess, labels, float64(observer.now().UnixNano())/float64(time.Second))
	}
}
