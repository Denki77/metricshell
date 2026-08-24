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

func TestSubreaperFailureDoesNotStartWorkload(t *testing.T) {
	signals := make(chan os.Signal)
	dependencies := testDependencies(signals)
	dependencies.enableSubreaper = func() error { return syscall.EPERM }
	started := false
	failed := false

	result := run([]string{"/workload-must-not-start"}, nil, io.Discard, io.Discard, Observers{
		Started: func(_, _ int) error {
			started = true
			return nil
		},
		Failed: func() error {
			failed = true
			return nil
		},
	}, dependencies)

	if result.ExitCode != exitCodes.ExitInternalFailure || result.Started || result.StartFailed || started || !failed {
		t.Fatalf("run() = %#v, observer started = %t, failed = %t; want pre-start internal failure", result, started, failed)
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

func TestReaperErrorCleansUpAndWaitsForDirectChild(t *testing.T) {
	t.Setenv(helperProcessEnvironment, "1")
	signals := make(chan os.Signal)
	dependencies := testDependencies(signals)
	firstWait := true
	dependencies.wait4 = func(pid int, status *syscall.WaitStatus, options int, usage *syscall.Rusage) (int, error) {
		if firstWait {
			firstWait = false
			return -1, syscall.EIO
		}
		return syscall.Wait4(pid, status, options, usage)
	}
	failed := false
	var pid int

	result := run(helperCommand(), nil, io.Discard, io.Discard, Observers{
		Started: func(workloadPID, _ int) error {
			pid = workloadPID
			return nil
		},
		Failed: func() error {
			failed = true
			return nil
		},
	}, dependencies)

	if result.ExitCode != exitCodes.ExitInternalFailure || !result.Started || !failed {
		t.Fatalf("run() = %#v, failed = %t; want reaper failure cleanup", result, failed)
	}
	assertReaped(t, pid)
}

func TestReusedPrimaryPIDDoesNotReplacePrimaryResult(t *testing.T) {
	const primaryPID = 41
	events := []reapEvent{
		{pid: primaryPID, status: syscall.WaitStatus(7 << 8)},
		{pid: 42, status: syscall.WaitStatus(8 << 8)},
		{pid: primaryPID, status: syscall.WaitStatus(9 << 8)},
	}
	var primaryResult *Result
	primaryEvents := 0
	releases := 0
	kinds := make([]string, 0, len(events))
	observers := Observers{
		PrimaryExited: func(exitCode int) error {
			primaryEvents++
			if exitCode != 7 {
				t.Fatalf("primary exit code = %d, want 7", exitCode)
			}
			return nil
		},
		ChildReaped: func(kind string) error {
			kinds = append(kinds, kind)
			return nil
		},
	}

	for _, event := range events {
		if err := processReapEvent(event, primaryPID, &primaryResult, func() { releases++ }, observers); err != nil {
			t.Fatalf("processReapEvent() error = %v", err)
		}
	}

	if primaryResult == nil || primaryResult.ExitCode != 7 {
		t.Fatalf("primary result = %#v, want preserved exit code 7", primaryResult)
	}
	if primaryEvents != 1 || releases != 1 {
		t.Fatalf("primary events = %d, releases = %d; want one of each", primaryEvents, releases)
	}
	wantKinds := []string{"direct", "adopted", "adopted"}
	for index, want := range wantKinds {
		if kinds[index] != want {
			t.Fatalf("child kinds = %v, want %v", kinds, wantKinds)
		}
	}
}

func TestResolveWaitStatusMatrix(t *testing.T) {
	for exitCode := 0; exitCode <= 255; exitCode++ {
		result := resolveWaitStatus(syscall.WaitStatus(exitCode << 8))
		if result.ExitCode != exitCode || !result.Started || result.StartFailed {
			t.Fatalf("exit %d resolved to %#v", exitCode, result)
		}
	}

	signals := []struct {
		value syscall.Signal
		want  int
	}{
		{value: syscall.SIGINT, want: 130},
		{value: syscall.SIGTERM, want: 143},
	}
	for _, signal := range signals {
		result := resolveWaitStatus(syscall.WaitStatus(signal.value))
		if result.ExitCode != signal.want || !result.Started || result.StartFailed {
			t.Fatalf("signal %s resolved to %#v, want exit %d", signal.value, result, signal.want)
		}
	}
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
		signals:         signals,
		stopSignals:     func() {},
		enableSubreaper: func() error { return nil },
		forward:         forwardSignal,
		killGroup:       func(processGroupID int) error { return syscall.Kill(-processGroupID, syscall.SIGKILL) },
		wait4:           syscall.Wait4,
		fallbackDelay:   10 * time.Millisecond,
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
