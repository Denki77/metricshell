package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/config"
	exitCodes "github.com/Denki77/metricshell/implementation/internal/constants"
	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
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
			_, err := fmt.Fprintln(stdout, identity.String())
			if err != nil {
				fmt.Println(err.Error())
			}
			return exitCodes.Success
		case "--help", "-h":
			_, err := fmt.Fprint(stdout, usage)
			if err != nil {
				fmt.Println(err.Error())
			}
			return exitCodes.Success
		}
	}

	configuration, err := config.Parse(args)
	if err == nil {
		runtimeState := "running"
		result := workload.Run(configuration.Workload, stdin, stdout, stderr, workload.Observers{
			Started: logger.WriteWorkloadStarted,
			Signal: func(event workload.SignalEvent) error {
				switch event.Outcome {
				case workload.SignalForwarded:
					if event.Name == "TERM" || event.Name == "INT" {
						runtimeState = "stopping"
					}
					return logger.WriteSignalForwarded(event.Name, runtimeState, event.ProcessGroupID)
				case workload.SignalIgnored:
					return logger.WriteSignalIgnored(event.Name, event.Reason, event.ProcessGroupID)
				default:
					return logger.WriteSignalFailed(event.Name, event.ProcessGroupID)
				}
			},
		})
		if result.StartFailed {
			if writeErr := logger.WriteWorkloadStartFailed(); writeErr != nil {
				return exitCodes.ExitInternalFailure
			}
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
	return exitCodes.ExitConfigurationInvalid
}
