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

const signalQueueCapacity = 16

const groupKillFallbackDelay = 100 * time.Millisecond

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
	Started func(pid, processGroupID int) error
	Signal  func(event SignalEvent) error
}

type runDependencies struct {
	signals       <-chan os.Signal
	stopSignals   func()
	forward       func(processGroupID int, received os.Signal) SignalEvent
	killGroup     func(processGroupID int) error
	fallbackDelay time.Duration
}

// Run executes the workload in its own process group and forwards supported signals to that group.
func Run(argv []string, stdin io.Reader, stdout, stderr io.Writer, observers Observers) Result {
	signals := make(chan os.Signal, signalQueueCapacity)
	signal.Notify(signals, supportedSignals...)
	defer signal.Stop(signals)
	return run(argv, stdin, stdout, stderr, observers, runDependencies{
		signals:       signals,
		stopSignals:   func() { signal.Stop(signals) },
		forward:       forwardSignal,
		killGroup:     func(processGroupID int) error { return syscall.Kill(-processGroupID, syscall.SIGKILL) },
		fallbackDelay: groupKillFallbackDelay,
	})
}

func run(argv []string, stdin io.Reader, stdout, stderr io.Writer, observers Observers, dependencies runDependencies) Result {
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
	processGroupID := command.Process.Pid
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()

	if observers.Started != nil {
		if err := observers.Started(command.Process.Pid, processGroupID); err != nil {
			terminateAndWait(command, processGroupID, wait, dependencies)
			return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
		}
	}

	for _, received := range pending {
		if result, done := handleSignal(command, processGroupID, received, wait, observers.Signal, dependencies); done {
			return result
		}
	}

	for {
		select {
		case received := <-dependencies.signals:
			if result, done := handleSignal(command, processGroupID, received, wait, observers.Signal, dependencies); done {
				return result
			}
		case err := <-wait:
			dependencies.stopSignals()
			if notifyQueuedSignals(dependencies.signals, processGroupID, observers.Signal) != nil {
				return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
			}
			return resolveResult(err)
		}
	}
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

func handleSignal(command *exec.Cmd, processGroupID int, received os.Signal, wait <-chan error, observer func(SignalEvent) error, dependencies runDependencies) (Result, bool) {
	event := dependencies.forward(processGroupID, received)
	if observer != nil {
		if err := observer(event); err != nil {
			terminateAndWait(command, processGroupID, wait, dependencies)
			return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}, true
		}
	}
	if event.Outcome == SignalFailed {
		terminateAndWait(command, processGroupID, wait, dependencies)
		return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}, true
	}
	return Result{}, false
}

func terminateAndWait(command *exec.Cmd, processGroupID int, wait <-chan error, dependencies runDependencies) {
	_ = dependencies.killGroup(processGroupID)
	timer := time.NewTimer(dependencies.fallbackDelay)
	defer timer.Stop()
	select {
	case <-wait:
		return
	case <-timer.C:
		_ = command.Process.Kill()
		<-wait
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
				event := ignoredSignal(name, processGroupID, "target_exited")
				if err := observer(event); err != nil {
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

func resolveResult(err error) Result {
	if err == nil {
		return Result{ExitCode: exitCodes.Success, Started: true}
	}
	status, ok := err.(*exec.ExitError)
	if !ok {
		return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
	}
	waitStatus, ok := status.Sys().(syscall.WaitStatus)
	if !ok {
		return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
	}
	if waitStatus.Signaled() {
		return Result{ExitCode: 128 + int(waitStatus.Signal()), Started: true}
	}
	return Result{ExitCode: waitStatus.ExitStatus(), Started: true}
}
