package config

import (
	err "github.com/Denki77/metricshell/implementation/internal/error"
)

const (
	ErrorCodeConfigInvalid = "CONFIG_INVALID"
	ReasonConfiguration    = "configuration"
)

type Config struct {
	Workload []string
}

func Parse(args []string) (Config, error) {
	for index, argument := range args {
		if argument != "--" {
			continue
		}
		if index == 0 && len(args) > 1 {
			return Config{Workload: args[1:]}, nil
		}
		if index == 0 {
			return Config{}, err.Bootstrap.CommandRequired
		}
		return Config{}, err.Bootstrap.UnknownOption
	}
	if len(args) == 0 {
		return Config{}, err.Bootstrap.SeparatorRequired
	}
	return Config{}, err.Bootstrap.UnknownOption
}
