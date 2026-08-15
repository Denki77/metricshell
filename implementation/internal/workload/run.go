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
	PrimaryExited func(exitCode int) error
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
}

type reapEvent struct {
	pid    int
	status syscall.WaitStatus
	done   bool
	err    error
}

// Run executes and reaps one primary workload and all descendants adopted by MetricShell.
func Run(argv []string, stdin io.Reader, stdout, stderr io.Writer, observers Observers) Result {
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
	if observers.Started != nil {
		if err := observers.Started(primaryPID, processGroupID); err != nil {
			cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
			return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
		}
	}

	for _, received := range pending {
		if result, done := handleSignal(command, primaryPID, processGroupID, received, reaped, observers.Signal, dependencies); done {
			return result
		}
	}

	var primaryResult *Result
	for {
		select {
		case received := <-dependencies.signals:
			if result, done := handleSignal(command, primaryPID, processGroupID, received, reaped, observers.Signal, dependencies); done {
				return result
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

			if err := processReapEvent(event, primaryPID, &primaryResult, func() {
				_ = command.Process.Release()
			}, observers); err != nil {
				cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
				return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
			}
		}
	}
}

func processReapEvent(event reapEvent, primaryPID int, primaryResult **Result, releasePrimary func(), observers Observers) error {
	kind := "adopted"
	if event.pid == primaryPID && *primaryResult == nil {
		kind = "direct"
		resolved := resolveWaitStatus(event.status)
		*primaryResult = &resolved
		releasePrimary()
		if observers.PrimaryExited != nil {
			if err := observers.PrimaryExited(resolved.ExitCode); err != nil {
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

func handleSignal(command *exec.Cmd, primaryPID, processGroupID int, received os.Signal, reaped <-chan reapEvent, observer func(SignalEvent) error, dependencies runDependencies) (Result, bool) {
	event := dependencies.forward(processGroupID, received)
	if observer != nil {
		if err := observer(event); err != nil {
			cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
			return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}, true
		}
	}
	if event.Outcome == SignalFailed {
		cleanupAndReap(command, primaryPID, processGroupID, reaped, dependencies)
		return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}, true
	}
	return Result{}, false
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
