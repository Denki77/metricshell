package shutdown

import (
	"context"
	"testing"
	"time"
)

func TestValidateBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	valid := []Config{
		{TotalGrace: time.Second, WorkloadTimeout: 750 * time.Millisecond, Reserve: 250 * time.Millisecond},
		{TotalGrace: time.Hour, WorkloadTimeout: 59 * time.Minute, Reserve: time.Minute},
		{TotalGrace: time.Second, WorkloadTimeout: 0, Reserve: time.Second},
		{TotalGrace: 30 * time.Second, WorkloadTimeout: 28 * time.Second, Reserve: 2 * time.Second, Deadline: now.Add(time.Second)},
	}
	for _, configuration := range valid {
		if err := configuration.Validate(now); err != nil {
			t.Errorf("Validate(%+v) = %v", configuration, err)
		}
	}

	invalid := []Config{
		{TotalGrace: time.Second - 1, WorkloadTimeout: 0, Reserve: 250 * time.Millisecond},
		{TotalGrace: time.Hour + 1, WorkloadTimeout: 0, Reserve: 250 * time.Millisecond},
		{TotalGrace: time.Second, WorkloadTimeout: -1, Reserve: 250 * time.Millisecond},
		{TotalGrace: time.Second, WorkloadTimeout: 750 * time.Millisecond, Reserve: 250*time.Millisecond + 1},
		{TotalGrace: time.Second, WorkloadTimeout: 0, Reserve: 250*time.Millisecond - 1},
		{TotalGrace: time.Second, WorkloadTimeout: 0, Reserve: time.Minute + 1},
		{TotalGrace: time.Second, WorkloadTimeout: 0, Reserve: time.Second, Deadline: now},
	}
	for _, configuration := range invalid {
		if err := configuration.Validate(now); err == nil {
			t.Errorf("Validate(%+v) succeeded", configuration)
		}
	}
}

func TestResolveAbsoluteDeadline(t *testing.T) {
	now := time.Now()
	configuration := Defaults()
	external := now.Add(5 * time.Second)
	configuration.Deadline = external

	for _, test := range []struct {
		name           string
		at             time.Time
		workloadBudget time.Duration
		remaining      time.Duration
	}{
		{name: "full", at: now, workloadBudget: 3 * time.Second, remaining: 5 * time.Second},
		{name: "consumed", at: now.Add(2 * time.Second), workloadBudget: time.Second, remaining: 3 * time.Second},
		{name: "reserve exceeds remaining", at: now.Add(4 * time.Second), workloadBudget: 0, remaining: time.Second},
		{name: "expired", at: now.Add(6 * time.Second), workloadBudget: 0, remaining: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := configuration.Resolve(test.at)
			if plan.WorkloadBudget != test.workloadBudget || plan.Remaining(test.at) != test.remaining {
				t.Fatalf("plan = %+v, remaining=%s", plan, plan.Remaining(test.at))
			}
		})
	}
}

func TestPhaseContextsAreCappedAndCancellable(t *testing.T) {
	now := time.Now()
	plan := Config{TotalGrace: time.Second, WorkloadTimeout: 750 * time.Millisecond, Reserve: 250 * time.Millisecond}.Resolve(now)
	for _, phase := range Phases {
		parent, cancelParent := context.WithCancel(context.Background())
		phaseContext, cancelPhase, err := plan.PhaseContext(parent, phase, time.Hour, now)
		if err != nil {
			t.Fatal(err)
		}
		deadline, ok := phaseContext.Deadline()
		if !ok || deadline.After(plan.Deadline) {
			t.Fatalf("%s deadline = %v, plan deadline = %v", phase, deadline, plan.Deadline)
		}
		if (phase == SignalForwarding || phase == WorkloadGrace) && deadline.After(plan.WorkloadDeadline) {
			t.Fatalf("%s exceeds workload deadline", phase)
		}
		cancelParent()
		<-phaseContext.Done()
		if phaseContext.Err() != context.Canceled {
			t.Fatalf("%s error = %v", phase, phaseContext.Err())
		}
		cancelPhase()
	}
}

func TestCompletionReasons(t *testing.T) {
	now := time.Now()
	plan := Defaults().Resolve(now)
	if got := plan.CompletionReason(now, nil); got != Completed {
		t.Fatal(got)
	}
	if got := plan.CompletionReason(now, context.Canceled); got != Cancelled {
		t.Fatal(got)
	}
	if got := plan.CompletionReason(plan.Deadline, nil); got != DeadlineExhausted {
		t.Fatal(got)
	}
}
