package workload

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	exitCodes "github.com/Denki77/metricshell/implementation/internal/constants"
	"github.com/Denki77/metricshell/implementation/internal/shutdown"
)

const (
	signalQueueCapacity    = 16
	reapQueueCapacity      = 16
	groupKillFallbackDelay = 100 * time.Millisecond
)

var supportedSignals = []os.Signal{
	syscall.SIGTERM,
	syscall.SIGINT,
	syscall.SIGHUP,
	syscall.SIGQUIT,
}

type Result struct {
	ExitCode    int
	Started     bool
	StartFailed bool
	Forced      bool
}

type SignalOutcome uint8

const (
	SignalForwarded SignalOutcome = iota
	SignalIgnored
	SignalFailed
)

type SignalEvent struct {
	Name           string
	ProcessGroupID int
	Outcome        SignalOutcome
	Reason         string
}

type Observers struct {
	Started       func(pid, processGroupID int) error
	Signal        func(event SignalEvent) error
	Shutdown      func(signal string, plan shutdown.Plan) error
	Forced        func(signal string, processGroupID int) error
	PrimaryExited func(exitCode int, forced bool) error
	ChildReaped   func(kind string) error
	Failed        func() error
}

type wait4Func func(pid int, status *syscall.WaitStatus, options int, usage *syscall.Rusage) (int, error)

type runDependencies struct {
	signals         <-chan os.Signal
	stopSignals     func()
	enableSubreaper func() error
	forward         func(processGroupID int, received os.Signal) SignalEvent
	killGroup       func(processGroupID int) error
	wait4           wait4Func
	fallbackDelay   time.Duration
	shutdown        shutdown.Config
	now             func() time.Time
	after           func(time.Duration) <-chan time.Time
}

type reapEvent struct {
	pid    int
	status syscall.WaitStatus
	done   bool
	err    error
}

// Run executes and reaps one primary workload and all descendants adopted by MetricShell.
func Run(argv []string, stdin io.Reader, stdout, stderr io.Writer, shutdownConfiguration shutdown.Config, observers Observers) Result {
	signals := make(chan os.Signal, signalQueueCapacity)
	signal.Notify(signals, supportedSignals...)
	defer signal.Stop(signals)
	return run(argv, stdin, stdout, stderr, observers, runDependencies{
		signals:         signals,
		stopSignals:     func() { signal.Stop(signals) },
		enableSubreaper: enableSubreaper,
		forward:         forwardSignal,
		killGroup:       func(processGroupID int) error { return syscall.Kill(-processGroupID, syscall.SIGKILL) },
		wait4:           syscall.Wait4,
		fallbackDelay:   groupKillFallbackDelay,
		shutdown:        shutdownConfiguration,
		now:             time.Now,
		after:           time.After,
	})
}

func run(argv []string, stdin io.Reader, stdout, stderr io.Writer, observers Observers, dependencies runDependencies) Result {
	if err := dependencies.enableSubreaper(); err != nil {
		notifyFailure(observers.Failed)
		return Result{ExitCode: exitCodes.ExitInternalFailure}
	}

	command := exec.Command(argv[0], argv[1:]...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	pending, termination := drainPreStartSignals(dependencies.signals)
	if termination != 0 {
		return Result{ExitCode: 128 + int(termination)}
	}
	if err := command.Start(); err != nil {
		return Result{ExitCode: exitCodes.ExitWorkloadStartFailed, StartFailed: true}
	}

	primaryPID := command.Process.Pid
	processGroupID := primaryPID
	reaped := startReaper(dependencies.wait4)
	escalation := newEscalation(processGroupID, observers, dependencies)
	if observers.Started != nil {
		if err := observers.Started(primaryPID, processGroupID); err != nil {
			cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
			return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
		}
	}

	for _, received := range pending {
		if err := escalation.handle(received); err != nil {
			cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
			return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
		}
	}

	var primaryResult *Result
	for {
		select {
		case received := <-dependencies.signals:
			if err := escalation.handle(received); err != nil {
				cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
				return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
			}
		case <-escalation.timeout():
			if err := escalation.force(); err != nil {
				notifyFailure(observers.Failed)
				cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
				return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
			}
		case event := <-reaped:
			if event.err != nil {
				notifyFailure(observers.Failed)
				cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
				return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
			}
			if event.done {
				dependencies.stopSignals()
				if notifyQueuedSignals(dependencies.signals, processGroupID, observers.Signal) != nil {
					return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
				}
				if primaryResult == nil {
					notifyFailure(observers.Failed)
					return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
				}
				return *primaryResult
			}

			if err := processReapEvent(event, primaryPID, &primaryResult, escalation.forced, func() {
				_ = command.Process.Release()
			}, observers); err != nil {
				cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
				return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
			}
		}
	}
}

func processReapEvent(event reapEvent, primaryPID int, primaryResult **Result, forced bool, releasePrimary func(), observers Observers) error {
	kind := "adopted"
	if event.pid == primaryPID && *primaryResult == nil {
		kind = "direct"
		resolved := resolveWaitStatus(event.status)
		resolved.Forced = forced
		*primaryResult = &resolved
		releasePrimary()
		if observers.PrimaryExited != nil {
			if err := observers.PrimaryExited(resolved.ExitCode, resolved.Forced); err != nil {
				return err
			}
		}
	}
	if observers.ChildReaped != nil {
		return observers.ChildReaped(kind)
	}
	return nil
}

func startReaper(wait4 wait4Func) <-chan reapEvent {
	events := make(chan reapEvent, reapQueueCapacity)
	go func() {
		for {
			var status syscall.WaitStatus
			pid, err := wait4(-1, &status, 0, nil)
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			if errors.Is(err, syscall.ECHILD) {
				events <- reapEvent{done: true}
				return
			}
			if err != nil {
				events <- reapEvent{err: err}
				continue
			}
			events <- reapEvent{pid: pid, status: status}
		}
	}()
	return events
}

func drainPreStartSignals(signals <-chan os.Signal) ([]os.Signal, syscall.Signal) {
	pending := make([]os.Signal, 0, signalQueueCapacity)
	for {
		select {
		case received := <-signals:
			systemSignal, ok := received.(syscall.Signal)
			if ok && (systemSignal == syscall.SIGTERM || systemSignal == syscall.SIGINT) {
				return nil, systemSignal
			}
			pending = append(pending, received)
		default:
			return pending, 0
		}
	}
}

type escalation struct {
	processGroupID int
	observers      Observers
	dependencies   runDependencies
	plan           *shutdown.Plan
	timer          <-chan time.Time
	forced         bool
	forceAttempted bool
	signal         string
}

func newEscalation(processGroupID int, observers Observers, dependencies runDependencies) *escalation {
	return &escalation{processGroupID: processGroupID, observers: observers, dependencies: dependencies}
}

func (value *escalation) handle(received os.Signal) error {
	systemSignal, isSystemSignal := received.(syscall.Signal)
	name, supported := signalName(systemSignal)
	termination := isSystemSignal && supported && (systemSignal == syscall.SIGTERM || systemSignal == syscall.SIGINT)
	repeated := termination && value.plan != nil
	if termination && !repeated {
		plan := value.dependencies.shutdown.Resolve(value.dependencies.now())
		value.plan = &plan
		value.signal = name
		if value.observers.Shutdown != nil {
			if err := value.observers.Shutdown(name, plan); err != nil {
				return err
			}
		}
	}

	event := value.dependencies.forward(value.processGroupID, received)
	if value.observers.Signal != nil {
		if err := value.observers.Signal(event); err != nil {
			return err
		}
	}
	if event.Outcome == SignalFailed {
		return errors.New("signal forwarding failed")
	}
	if !termination || value.forceAttempted {
		return nil
	}
	if repeated || value.plan.WorkloadBudget == 0 {
		return value.force()
	}
	value.timer = value.dependencies.after(value.plan.WorkloadBudget)
	return nil
}

func (value *escalation) timeout() <-chan time.Time {
	return value.timer
}

func (value *escalation) force() error {
	if value.forceAttempted {
		return nil
	}
	value.forceAttempted = true
	value.timer = nil
	err := value.dependencies.killGroup(value.processGroupID)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if err != nil {
		return err
	}
	value.forced = true
	if value.observers.Forced != nil {
		return value.observers.Forced(value.signal, value.processGroupID)
	}
	return nil
}

func cleanupAndReap(command *exec.Cmd, primaryPID, processGroupID int, reaped <-chan reapEvent, dependencies runDependencies) {
	_ = dependencies.killGroup(processGroupID)
	timer := time.NewTimer(dependencies.fallbackDelay)
	defer timer.Stop()
	fallback := timer.C
	primaryReaped := false
	for {
		select {
		case event := <-reaped:
			if event.done {
				return
			}
			if event.err != nil {
				continue
			}
			if event.pid == primaryPID {
				primaryReaped = true
				_ = command.Process.Release()
			}
		case <-fallback:
			if !primaryReaped {
				_ = command.Process.Kill()
			}
			fallback = nil
		}
	}
}

func notifyFailure(observer func() error) {
	if observer != nil {
		_ = observer()
	}
}

func forwardSignal(processGroupID int, received os.Signal) SignalEvent {
	systemSignal, ok := received.(syscall.Signal)
	if !ok {
		return ignoredSignal("UNSUPPORTED", processGroupID, "unsupported_signal")
	}
	name, supported := signalName(systemSignal)
	if !supported {
		return ignoredSignal(name, processGroupID, "unsupported_signal")
	}
	event := SignalEvent{Name: name, ProcessGroupID: processGroupID, Outcome: SignalForwarded}
	if err := syscall.Kill(-processGroupID, systemSignal); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return ignoredSignal(name, processGroupID, "target_exited")
		}
		event.Outcome = SignalFailed
		event.Reason = "signal_forward"
	}
	return event
}

func notifyQueuedSignals(signals <-chan os.Signal, processGroupID int, observer func(SignalEvent) error) error {
	for {
		select {
		case received := <-signals:
			if observer != nil {
				name := "UNSUPPORTED"
				if systemSignal, ok := received.(syscall.Signal); ok {
					name, _ = signalName(systemSignal)
				}
				if err := observer(ignoredSignal(name, processGroupID, "target_exited")); err != nil {
					return err
				}
			}
		default:
			return nil
		}
	}
}

func ignoredSignal(name string, processGroupID int, reason string) SignalEvent {
	return SignalEvent{Name: name, ProcessGroupID: processGroupID, Outcome: SignalIgnored, Reason: reason}
}

func signalName(value syscall.Signal) (string, bool) {
	switch value {
	case syscall.SIGTERM:
		return "TERM", true
	case syscall.SIGINT:
		return "INT", true
	case syscall.SIGHUP:
		return "HUP", true
	case syscall.SIGQUIT:
		return "QUIT", true
	default:
		return "UNSUPPORTED", false
	}
}

func resolveWaitStatus(status syscall.WaitStatus) Result {
	if status.Signaled() {
		return Result{ExitCode: 128 + int(status.Signal()), Started: true}
	}
	return Result{ExitCode: status.ExitStatus(), Started: true}
}
