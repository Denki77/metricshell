package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/config"
	exitCodes "github.com/Denki77/metricshell/implementation/internal/constants"
	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
	"github.com/Denki77/metricshell/implementation/internal/workload"
)

const usage = `Usage:
  metricshell --version
  metricshell --help
  metricshell [options] -- executable [argument ...]

`

// Run executes the command line interface with the given arguments and returns the exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, identity buildinfo.Info, now func() time.Time) int {
	logger := diagnostic.New(stderr, now)

	if len(args) == 1 {
		switch args[0] {
		case "--version":
			if _, err := fmt.Fprintln(stdout, identity.String()); err != nil {
				return exitCodes.ExitInternalFailure
			}
			return exitCodes.Success
		case "--help", "-h":
			if _, err := fmt.Fprint(stdout, usage); err != nil {
				return exitCodes.ExitInternalFailure
			}
			return exitCodes.Success
		}
	}
	if err := logger.WriteRuntimeInitializing(os.Getpid()); err != nil {
		return exitCodes.ExitInternalFailure
	}
	machine, err := lifecycle.New(func(change lifecycle.Change) error {
		return logger.WriteStateChanged(string(change.Previous), string(change.Current))
	})
	if err != nil {
		return exitCodes.ExitInternalFailure
	}

	configuration, err := config.Parse(args)
	if err == nil {
		if err := machine.TransitionEvent(lifecycle.ConfigurationValidated); err != nil {
			return failLifecycle(machine, logger)
		}
		result := workload.Run(configuration.Workload, stdin, stdout, stderr, workload.Observers{
			Started: func(pid, processGroupID int) error {
				if err := machine.TransitionEvent(lifecycle.WorkloadStarted); err != nil {
					return err
				}
				return logger.WriteWorkloadStarted(pid, processGroupID)
			},
			PrimaryExited: func(exitCode int) error {
				if err := machine.TransitionEvent(lifecycle.WorkloadExited); err != nil {
					return err
				}
				return logger.WriteWorkloadExited(exitCode)
			},
			ChildReaped: func(kind string) error {
				return logger.WriteChildReaped(kind, string(machine.State()))
			},
			Failed: func() error {
				if machine.State() != lifecycle.Failed {
					if err := machine.TransitionEvent(lifecycle.RuntimeFailed); err != nil {
						return err
					}
				}
				return logger.WriteRuntimeFailed()
			},
			Signal: func(event workload.SignalEvent) error {
				if event.Name == "TERM" || event.Name == "INT" {
					if err := machine.TransitionEvent(lifecycle.TerminationAfterSpawn); err != nil {
						return err
					}
				}
				switch event.Outcome {
				case workload.SignalForwarded:
					return logger.WriteSignalForwarded(event.Name, string(machine.State()), event.ProcessGroupID)
				case workload.SignalIgnored:
					return logger.WriteSignalIgnored(event.Name, event.Reason, event.ProcessGroupID)
				default:
					return logger.WriteSignalFailed(event.Name, event.ProcessGroupID)
				}
			},
		})
		if result.StartFailed {
			if err := machine.TransitionEvent(lifecycle.WorkloadStartFailed); err != nil {
				return failLifecycle(machine, logger)
			}
			if writeErr := logger.WriteWorkloadStartFailed(); writeErr != nil {
				return exitCodes.ExitInternalFailure
			}
		}
		if result.Started && machine.State() == lifecycle.Finalizing {
			if err := machine.TransitionEvent(lifecycle.FinalizationImmediate); err != nil {
				return failLifecycle(machine, logger)
			}
		} else if !result.Started && !result.StartFailed && machine.State() == lifecycle.StartingWorkload {
			if err := machine.TransitionEvent(lifecycle.TerminationBeforeSpawn); err != nil {
				return failLifecycle(machine, logger)
			}
		}
		if machine.State() == lifecycle.Failed {
			_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
		}
		return result.ExitCode
	}
	if writeErr := logger.WriteConfigurationRejected(err); writeErr != nil {
		_, err := fmt.Fprintln(stderr, `{"schema_version":"1","level":"error","event":"runtime.failed","component":"runtime","state":"initializing","message":"diagnostic write failed","reason":"internal","error_code":"INTERNAL_FAILURE"}`)
		if err != nil {
			fmt.Println(err.Error())
		}
		return exitCodes.ExitConfigurationRejected
	}
	if transitionErr := machine.TransitionEvent(lifecycle.InitializationFailed); transitionErr == nil {
		_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
	}
	return exitCodes.ExitConfigurationInvalid
}

func failLifecycle(machine *lifecycle.Machine, logger *diagnostic.Logger) int {
	if machine.State() != lifecycle.Failed {
		_ = machine.TransitionEvent(lifecycle.RuntimeFailed)
	}
	_ = logger.WriteRuntimeFailed()
	_ = machine.TransitionEvent(lifecycle.CleanupCompleted)
	return exitCodes.ExitInternalFailure
}
