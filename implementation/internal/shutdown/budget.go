package shutdown

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	DefaultTotalGrace      = 30 * time.Second
	DefaultWorkloadTimeout = 28 * time.Second
	DefaultReserve         = 2 * time.Second
)

type Phase string

const (
	SignalForwarding Phase = "signal_forwarding"
	WorkloadGrace    Phase = "workload_grace"
	Finalization     Phase = "finalization"
	HTTPDrain        Phase = "http_drain"
	ForcedCleanup    Phase = "forced_cleanup"
)

var Phases = [...]Phase{SignalForwarding, WorkloadGrace, Finalization, HTTPDrain, ForcedCleanup}

type CompletionReason string

const (
	Completed         CompletionReason = "completed"
	DeadlineExhausted CompletionReason = "deadline_exhausted"
	Cancelled         CompletionReason = "cancelled"
)

type Config struct {
	TotalGrace      time.Duration
	WorkloadTimeout time.Duration
	Reserve         time.Duration
	Deadline        time.Time
}

func Defaults() Config {
	return Config{TotalGrace: DefaultTotalGrace, WorkloadTimeout: DefaultWorkloadTimeout, Reserve: DefaultReserve}
}

func (configuration Config) Validate(now time.Time) error {
	if configuration.TotalGrace < time.Second || configuration.TotalGrace > time.Hour {
		return fmt.Errorf("shutdown total grace must be between 1s and 1h")
	}
	if configuration.WorkloadTimeout < 0 || configuration.WorkloadTimeout > time.Hour {
		return fmt.Errorf("workload shutdown timeout must be between 0 and 1h")
	}
	if configuration.Reserve < 250*time.Millisecond || configuration.Reserve > time.Minute {
		return fmt.Errorf("shutdown reserve must be between 250ms and 1m")
	}
	if configuration.WorkloadTimeout > configuration.TotalGrace-configuration.Reserve {
		return fmt.Errorf("workload shutdown timeout plus reserve exceeds total grace")
	}
	if !configuration.Deadline.IsZero() && !configuration.Deadline.After(now) {
		return fmt.Errorf("shutdown deadline must be in the future")
	}
	return nil
}

type Plan struct {
	StartedAt        time.Time
	Deadline         time.Time
	WorkloadDeadline time.Time
	WorkloadBudget   time.Duration
	Reserve          time.Duration
}

func (configuration Config) Resolve(now time.Time) Plan {
	deadline := configuration.Deadline
	if deadline.IsZero() {
		deadline = now.Add(configuration.TotalGrace)
	} else {
		deadline = now.Add(deadline.Sub(now))
	}

	remaining := max(deadline.Sub(now), 0)
	workloadBudget := time.Duration(0)
	if remaining > configuration.Reserve {
		workloadBudget = min(configuration.WorkloadTimeout, remaining-configuration.Reserve)
	}
	return Plan{
		StartedAt:        now,
		Deadline:         deadline,
		WorkloadDeadline: now.Add(workloadBudget),
		WorkloadBudget:   workloadBudget,
		Reserve:          min(configuration.Reserve, remaining),
	}
}

func (plan Plan) Remaining(now time.Time) time.Duration {
	return max(plan.Deadline.Sub(now), 0)
}

func (plan Plan) PhaseContext(parent context.Context, phase Phase, requested time.Duration, now time.Time) (context.Context, context.CancelFunc, error) {
	if !knownPhase(phase) {
		return nil, nil, fmt.Errorf("unknown shutdown phase %q", phase)
	}
	if requested < 0 {
		return nil, nil, errors.New("phase duration must not be negative")
	}
	deadline := plan.Deadline
	if phase == SignalForwarding || phase == WorkloadGrace {
		deadline = minTime(deadline, plan.WorkloadDeadline)
	}
	deadline = minTime(deadline, now.Add(requested))
	context, cancel := context.WithDeadline(parent, deadline)
	return context, cancel, nil
}

func (plan Plan) CompletionReason(now time.Time, err error) CompletionReason {
	if errors.Is(err, context.Canceled) {
		return Cancelled
	}
	if !now.Before(plan.Deadline) || errors.Is(err, context.DeadlineExceeded) {
		return DeadlineExhausted
	}
	return Completed
}

func knownPhase(value Phase) bool {
	for _, phase := range Phases {
		if phase == value {
			return true
		}
	}
	return false
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
