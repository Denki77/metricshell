package lifecycle

import (
	"sync"
	"testing"
)

func TestNormativeTransitions(t *testing.T) {
	for _, candidate := range transitions {
		candidate := candidate
		t.Run(string(candidate.from)+"/"+string(candidate.event)+"/"+string(candidate.to), func(t *testing.T) {
			machine := &Machine{state: candidate.from}
			if err := machine.TransitionEvent(candidate.event); err != nil {
				t.Fatal(err)
			}
			if got := machine.State(); got != candidate.to {
				t.Fatalf("state = %q, want %q", got, candidate.to)
			}
		})
	}
}

func TestTransitionTableIsDeterministic(t *testing.T) {
	targets := make(map[struct {
		state State
		event Event
	}]State, len(transitions))
	for _, candidate := range transitions {
		key := struct {
			state State
			event Event
		}{candidate.from, candidate.event}
		if previous, exists := targets[key]; exists {
			t.Fatalf("ambiguous transition for %s --%s--> %s or %s", key.state, key.event, previous, candidate.to)
		}
		targets[key] = candidate.to
	}
}

func TestInvalidTransitionsDoNotMutateState(t *testing.T) {
	for _, from := range States {
		for _, event := range Events {
			valid := false
			for _, candidate := range transitions {
				valid = valid || candidate.from == from && candidate.event == event
			}
			if valid {
				continue
			}
			machine := &Machine{state: from}
			if err := machine.TransitionEvent(event); err == nil {
				t.Fatalf("accepted invalid event %s from %s", event, from)
			}
			if got := machine.State(); got != from {
				t.Fatalf("invalid transition changed state to %s", got)
			}
		}
	}
}

func TestOneHotState(t *testing.T) {
	machine, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		event Event
		state State
	}{{ConfigurationValidated, StartingWorkload}, {WorkloadStarted, Running}, {WorkloadExited, Finalizing}, {FinalizationImmediate, Terminated}} {
		if err := machine.TransitionEvent(step.event); err != nil {
			t.Fatal(err)
		}
		active := 0
		for _, value := range machine.OneHot() {
			if value == 1 {
				active++
			}
		}
		if active != 1 || machine.OneHot()[step.state] != 1 {
			t.Fatalf("one-hot state invalid for %s: %#v", step.state, machine.OneHot())
		}
	}
}

func TestConcurrentExitAndTerminationRemainValid(t *testing.T) {
	machine := &Machine{state: Running}
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, event := range []Event{TerminationAfterSpawn, WorkloadExited} {
		wait.Add(1)
		go func(event Event) {
			defer wait.Done()
			results <- machine.TransitionEvent(event)
		}(event)
	}
	wait.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 2 {
		t.Fatalf("successful transitions = %d, want 2", succeeded)
	}
	if state := machine.State(); state != Finalizing && state != Terminated {
		t.Fatalf("final state = %s, want finalizing or terminated", state)
	}
	active := 0
	for _, value := range machine.OneHot() {
		if value == 1 {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("one-hot state invalid: %#v", machine.OneHot())
	}
}

func TestProbeStatuses(t *testing.T) {
	want := map[State][2]int{
		Initializing: {200, 503}, StartingWorkload: {200, 503}, Running: {200, 200}, Stopping: {200, 503},
		Finalizing: {200, 503}, FinalWait: {200, 503}, Failed: {500, 503}, Terminated: {0, 0},
	}
	for state, statuses := range want {
		health, readiness := ProbeStatuses(state)
		if health != statuses[0] || readiness != statuses[1] {
			t.Errorf("%s = (%d,%d), want %v", state, health, readiness, statuses)
		}
	}
}
