package workload

import (
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	exitCodes "github.com/Denki77/metricshell/implementation/internal/constants"
)

const helperProcessEnvironment = "METRICSHELL_WORKLOAD_TEST_HELPER"

func TestTerminationQueuedBeforeStartDoesNotStartWorkload(t *testing.T) {
	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGTERM
	started := false

	result := run([]string{"/workload-must-not-start"}, nil, io.Discard, io.Discard, Observers{
		Started: func(_, _ int) error {
			started = true
			return nil
		},
	}, testDependencies(signals))

	if result.ExitCode != 143 || result.Started || result.StartFailed || started {
		t.Fatalf("run() = %#v, observer started = %t; want pre-start TERM without workload start", result, started)
	}
}

func TestSignalObserverErrorWaitsForDirectChild(t *testing.T) {
	t.Setenv(helperProcessEnvironment, "1")
	signals := make(chan os.Signal, 1)
	var pid int

	result := run(helperCommand(), nil, io.Discard, io.Discard, Observers{
		Started: func(workloadPID, _ int) error {
			pid = workloadPID
			signals <- syscall.SIGTERM
			return nil
		},
		Signal: func(SignalEvent) error { return errors.New("observer failed") },
	}, testDependencies(signals))

	if result.ExitCode != exitCodes.ExitInternalFailure || !result.Started {
		t.Fatalf("run() = %#v, want started internal failure", result)
	}
	assertReaped(t, pid)
}

func TestForwardingErrorUsesDirectFallbackAndWaits(t *testing.T) {
	t.Setenv(helperProcessEnvironment, "1")
	signals := make(chan os.Signal, 1)
	dependencies := testDependencies(signals)
	dependencies.forward = func(processGroupID int, _ os.Signal) SignalEvent {
		return SignalEvent{Name: "TERM", ProcessGroupID: processGroupID, Outcome: SignalFailed, Reason: "signal_forward"}
	}
	groupKillCalled := false
	dependencies.killGroup = func(int) error {
		groupKillCalled = true
		return nil
	}
	var pid int

	result := run(helperCommand(), nil, io.Discard, io.Discard, Observers{
		Started: func(workloadPID, _ int) error {
			pid = workloadPID
			signals <- syscall.SIGTERM
			return nil
		},
	}, dependencies)

	if result.ExitCode != exitCodes.ExitInternalFailure || !groupKillCalled {
		t.Fatalf("run() = %#v, group kill called = %t; want forwarding failure cleanup", result, groupKillCalled)
	}
	assertReaped(t, pid)
}

func TestUnsupportedSignalIsIgnored(t *testing.T) {
	event := forwardSignal(1, syscall.SIGUSR1)
	if event.Outcome != SignalIgnored || event.Reason != "unsupported_signal" {
		t.Fatalf("forwardSignal(SIGUSR1) = %#v, want unsupported signal ignored", event)
	}
}

func TestDisappearedProcessGroupIsIgnored(t *testing.T) {
	event := forwardSignal(1<<30, syscall.SIGTERM)
	if event.Outcome != SignalIgnored || event.Reason != "target_exited" {
		t.Fatalf("forwardSignal() = %#v, want disappeared group ignored", event)
	}
}

func TestQueuedSignalAfterWaitIsIgnored(t *testing.T) {
	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGHUP

	var observed SignalEvent
	err := notifyQueuedSignals(signals, 42, func(event SignalEvent) error {
		observed = event
		return nil
	})
	if err != nil {
		t.Fatalf("notifyQueuedSignals() error = %v", err)
	}
	if observed.Name != "HUP" || observed.Outcome != SignalIgnored || observed.Reason != "target_exited" {
		t.Fatalf("notifyQueuedSignals() event = %#v, want late HUP ignored", observed)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperProcessEnvironment) != "1" {
		return
	}
	for {
		time.Sleep(time.Hour)
	}
}

func helperCommand() []string {
	return []string{os.Args[0], "-test.run=^TestHelperProcess$"}
}

func testDependencies(signals <-chan os.Signal) runDependencies {
	return runDependencies{
		signals:       signals,
		stopSignals:   func() {},
		forward:       forwardSignal,
		killGroup:     func(processGroupID int) error { return syscall.Kill(-processGroupID, syscall.SIGKILL) },
		fallbackDelay: 10 * time.Millisecond,
	}
}

func assertReaped(t *testing.T, pid int) {
	t.Helper()
	var status syscall.WaitStatus
	waited, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if waited != -1 || !errors.Is(err, syscall.ECHILD) {
		t.Fatalf("Wait4(%d) = (%d, %v), want direct child already reaped", pid, waited, err)
	}
}
