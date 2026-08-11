package buildinfo

import "fmt"

var (
	Version  = "dev"
	Revision = "unknown"
)

// Info is the immutable identity embedded into one MetricShell binary.
type Info struct {
	Version  string
	Revision string
}

// Current returns the identity supplied by the linker or the development defaults.
func Current() Info {
	return Info{Version: Version, Revision: Revision}
}

func (i Info) String() string {
	return fmt.Sprintf("metricshell version=%s revision=%s", i.Version, i.Revision)
}
