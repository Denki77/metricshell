package workload

import (
	"io"
	"os/exec"
	"syscall"

	exitCodes "github.com/Denki77/metricshell/implementation/internal/constants"
)

type Result struct {
	ExitCode int
	Started  bool
}

type StartObserver func(pid, processGroupID int) error

// Run executes the workload with the given arguments and returns the result.
func Run(argv []string, stdin io.Reader, stdout, stderr io.Writer, onStart StartObserver) Result {
	command := exec.Command(argv[0], argv[1:]...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := command.Start(); err != nil {
		return Result{ExitCode: exitCodes.ExitWorkloadStartFailed}
	}
	if onStart != nil {
		if err := onStart(command.Process.Pid, command.Process.Pid); err != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			_ = command.Wait()
			return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
		}
	}
	if err := command.Wait(); err == nil {
		return Result{ExitCode: exitCodes.Success, Started: true}
	}

	status, ok := command.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return Result{ExitCode: exitCodes.ExitInternalFailure, Started: true}
	}
	if status.Signaled() {
		return Result{ExitCode: 128 + int(status.Signal()), Started: true}
	}
	return Result{ExitCode: status.ExitStatus(), Started: true}
}
