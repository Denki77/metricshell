package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/config"
	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
)

const usage = `Usage:
  metricshell --version
  metricshell --help
  metricshell [options] -- executable [argument ...]

Workload execution is introduced by ISSUE-002.
`

// Run executes the bootstrap command surface and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer, identity buildinfo.Info, now func() time.Time) int {
	if len(args) == 1 {
		switch args[0] {
		case "--version":
			fmt.Fprintln(stdout, identity.String())
			return 0
		case "--help", "-h":
			fmt.Fprint(stdout, usage)
			return 0
		}
	}

	err := config.ValidateBootstrap(args)
	if writeErr := diagnostic.WriteConfigurationRejected(stderr, now(), err); writeErr != nil {
		fmt.Fprintln(stderr, `{"schema_version":"1","level":"error","event":"runtime.failed","component":"runtime","state":"initializing","message":"diagnostic write failed","reason":"internal","error_code":"INTERNAL_FAILURE"}`)
		return 70
	}
	return config.ExitConfigurationInvalid
}
