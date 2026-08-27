package lifecycle

import (
	"fmt"
	"sync"
)

type State string

const (
	Initializing     State = "initializing"
	StartingWorkload State = "starting_workload"
	Running          State = "running"
	Stopping         State = "stopping"
	Finalizing       State = "finalizing"
	FinalWait        State = "final_wait"
	Failed           State = "failed"
	Terminated       State = "terminated"
)

var States = [...]State{
	Initializing,
	StartingWorkload,
	Running,
	Stopping,
	Finalizing,
	FinalWait,
	Failed,
	Terminated,
}

type Event string

const (
	ConfigurationValidated Event = "configuration_validated"
	InitializationFailed   Event = "initialization_failed"
	WorkloadStarted        Event = "workload_started"
	WorkloadStartFailed    Event = "workload_start_failed"
	WorkloadExited         Event = "workload_exited"
	TerminationBeforeSpawn Event = "termination_before_spawn"
	TerminationAfterSpawn  Event = "termination_after_spawn"
	RuntimeFailed          Event = "runtime_failed"
	FinalizationImmediate  Event = "finalization_completed_immediate"
	FinalizationWait       Event = "finalization_completed_wait"
	FinalWaitCompleted     Event = "final_wait_completed"
	CleanupCompleted       Event = "cleanup_completed"
)

var Events = [...]Event{
	ConfigurationValidated,
	InitializationFailed,
	WorkloadStarted,
	WorkloadStartFailed,
	WorkloadExited,
	TerminationBeforeSpawn,
	TerminationAfterSpawn,
	RuntimeFailed,
	FinalizationImmediate,
	FinalizationWait,
	FinalWaitCompleted,
	CleanupCompleted,
}

type Change struct {
	Previous State
	Current  State
}

type Observer func(Change) error

type transition struct {
	from  State
	event Event
	to    State
}

var transitions = [...]transition{
	{Initializing, ConfigurationValidated, StartingWorkload},
	{Initializing, InitializationFailed, Failed},
	{Initializing, TerminationBeforeSpawn, Terminated},
	{Initializing, RuntimeFailed, Failed},
	{StartingWorkload, WorkloadStarted, Running},
	{StartingWorkload, WorkloadStartFailed, Failed},
	{StartingWorkload, TerminationBeforeSpawn, Terminated},
	{StartingWorkload, TerminationAfterSpawn, Stopping},
	{StartingWorkload, RuntimeFailed, Failed},
	{Running, WorkloadExited, Finalizing},
	{Running, TerminationAfterSpawn, Stopping},
	{Running, RuntimeFailed, Failed},
	{Stopping, WorkloadExited, Finalizing},
	{Stopping, RuntimeFailed, Failed},
	{Finalizing, FinalizationWait, FinalWait},
	{Finalizing, FinalizationImmediate, Terminated},
	{Finalizing, TerminationAfterSpawn, Terminated},
	{Finalizing, RuntimeFailed, Failed},
	{FinalWait, FinalWaitCompleted, Terminated},
	{FinalWait, TerminationAfterSpawn, Terminated},
	{FinalWait, RuntimeFailed, Failed},
	{Failed, CleanupCompleted, Terminated},
}

type Machine struct {
	mu       sync.RWMutex
	state    State
	observer Observer
}

func New(observer Observer) (*Machine, error) {
	machine := &Machine{state: Initializing, observer: observer}
	if observer != nil {
		if err := observer(Change{Current: Initializing}); err != nil {
			return nil, err
		}
	}
	return machine, nil
}

func (machine *Machine) State() State {
	machine.mu.RLock()
	defer machine.mu.RUnlock()
	return machine.state
}

func (machine *Machine) TransitionEvent(event Event) error {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	targets := make([]State, 0, 2)
	for _, candidate := range transitions {
		if candidate.from == machine.state && candidate.event == event {
			targets = append(targets, candidate.to)
		}
	}
	if len(targets) != 1 {
		return fmt.Errorf("lifecycle event %s has %d targets from %s", event, len(targets), machine.state)
	}
	return machine.transitionLocked(targets[0])
}

func (machine *Machine) transitionLocked(target State) error {
	previous := machine.state
	machine.state = target
	if machine.observer != nil {
		if err := machine.observer(Change{Previous: previous, Current: target}); err != nil {
			return err
		}
	}
	return nil
}

func (machine *Machine) OneHot() map[State]float64 {
	machine.mu.RLock()
	defer machine.mu.RUnlock()

	values := make(map[State]float64, len(States))
	for _, state := range States {
		values[state] = 0
	}
	values[machine.state] = 1
	return values
}

func ProbeStatuses(state State) (health, readiness int) {
	switch state {
	case Initializing, StartingWorkload, Stopping, Finalizing, FinalWait:
		return 200, 503
	case Running:
		return 200, 200
	case Failed:
		return 500, 503
	default:
		return 0, 0
	}
}
