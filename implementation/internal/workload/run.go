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

// Run executes the workload with the given arguments and returns the result.
func Run(argv []string, stdin io.Reader, stdout, stderr io.Writer) Result {
	command := exec.Command(argv[0], argv[1:]...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr

	if err := command.Start(); err != nil {
		return Result{ExitCode: exitCodes.ExitWorkloadStartFailed}
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
