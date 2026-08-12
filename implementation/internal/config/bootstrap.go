package config

import "errors"

const (
	ExitConfigurationInvalid = 64
	ErrorCodeConfigInvalid   = "CONFIG_INVALID"
	ReasonConfiguration      = "configuration"
)

var ErrStartupConfiguration = errors.New("a workload command after -- is required; workload execution is introduced by ISSUE-002")

// ValidateBootstrap is deliberately limited to the startup surface owned by ISSUE-001.
func ValidateBootstrap([]string) error {
	return ErrStartupConfiguration
}
