package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	err "github.com/Denki77/metricshell/implementation/internal/error"
	"github.com/Denki77/metricshell/implementation/internal/exposition"
	"github.com/Denki77/metricshell/implementation/internal/shutdown"
)

const (
	ErrorCodeConfigInvalid = "CONFIG_INVALID"
)

type LookupEnv func(string) (string, bool)

type Config struct {
	Workload   []string
	Shutdown   shutdown.Config
	Exposition exposition.Config
}

var shutdownOptions = map[string]string{
	"--shutdown-total-grace":      "total_grace",
	"--workload-shutdown-timeout": "workload_timeout",
	"--shutdown-reserve":          "reserve",
	"--shutdown-deadline":         "deadline",
	"--exposition-listen":         "exposition_listen",
	"--max-response-bytes":        "response_bytes",
	"--max-concurrent-scrapes":    "concurrent_scrapes",
	"--exposition-write-timeout":  "exposition_write_timeout",
	"--metrics-include":           "metrics_include",
	"--metrics-exclude":           "metrics_exclude",
}

var unsupportedSharedMemoryEnvironment = [...]string{
	"METRICSHELL_MMAP_ENABLED",
	"METRICSHELL_MMAP_PATH",
	"METRICSHELL_SHARED_MEMORY_PATH",
	"METRICSHELL_SHM_PATH",
}

func Parse(args []string, now time.Time, lookupEnv LookupEnv) (Config, error) {
	if lookupEnv != nil {
		for _, name := range unsupportedSharedMemoryEnvironment {
			if _, exists := lookupEnv(name); exists {
				return Config{}, err.Bootstrap.UnknownOption
			}
		}
		if transport, exists := lookupEnv("METRICSHELL_INGESTION_TRANSPORT"); exists && (transport == "mmap" || transport == "shared_memory" || transport == "shm") {
			return Config{}, err.Bootstrap.UnknownOption
		}
	}
	separator := -1
	for index, argument := range args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		if len(args) == 0 {
			return Config{}, err.Bootstrap.SeparatorRequired
		}
		return Config{}, err.Bootstrap.UnknownOption
	}
	if separator == len(args)-1 {
		return Config{}, err.Bootstrap.CommandRequired
	}

	shutdownConfiguration, parseErr := parseShutdown(args[:separator], now, lookupEnv)
	if parseErr != nil {
		return Config{}, parseErr
	}
	expositionConfiguration, parseErr := parseExposition(args[:separator], lookupEnv)
	if parseErr != nil {
		return Config{}, parseErr
	}
	return Config{Workload: args[separator+1:], Shutdown: shutdownConfiguration, Exposition: expositionConfiguration}, nil
}

func parseExposition(args []string, lookupEnv LookupEnv) (exposition.Config, error) {
	configuration := exposition.DefaultConfig()
	values := map[string]string{}
	if lookupEnv != nil {
		for environment, property := range map[string]string{
			"METRICSHELL_EXPOSITION_LISTEN":        "exposition_listen",
			"METRICSHELL_MAX_RESPONSE_BYTES":       "response_bytes",
			"METRICSHELL_MAX_CONCURRENT_SCRAPES":   "concurrent_scrapes",
			"METRICSHELL_EXPOSITION_WRITE_TIMEOUT": "exposition_write_timeout",
		} {
			if value, exists := lookupEnv(environment); exists {
				values[property] = value
			}
		}
		if value, exists := lookupEnv("METRICSHELL_METRICS_INCLUDE"); exists {
			configuration.Include = splitList(value)
		}
		if value, exists := lookupEnv("METRICSHELL_METRICS_EXCLUDE"); exists {
			configuration.Exclude = splitList(value)
		}
	}
	includeCLI, excludeCLI := false, false
	for index := 0; index < len(args); index++ {
		name, value, hasValue := strings.Cut(args[index], "=")
		property, known := shutdownOptions[name]
		if !known {
			return exposition.Config{}, err.Bootstrap.UnknownOption
		}
		if !hasValue {
			index++
			if index == len(args) {
				return exposition.Config{}, fmt.Errorf("%s requires a value", name)
			}
			value = args[index]
		}
		switch property {
		case "metrics_include":
			if !includeCLI {
				configuration.Include, includeCLI = nil, true
			}
			configuration.Include = append(configuration.Include, value)
		case "metrics_exclude":
			if !excludeCLI {
				configuration.Exclude, excludeCLI = nil, true
			}
			configuration.Exclude = append(configuration.Exclude, value)
		case "exposition_listen", "response_bytes", "concurrent_scrapes", "exposition_write_timeout":
			values[property] = value
		}
	}
	var parseErr error
	if value, exists := values["exposition_listen"]; exists {
		configuration.Listen = value
	}
	if value, exists := values["response_bytes"]; exists {
		configuration.ResponseBytes, parseErr = parseBytes(value)
	}
	if parseErr == nil {
		if value, exists := values["concurrent_scrapes"]; exists {
			configuration.Concurrent, parseErr = parseCount(value)
		}
	}
	if parseErr == nil {
		if value, exists := values["exposition_write_timeout"]; exists {
			configuration.WriteTimeout, parseErr = parseDuration(value)
		}
	}
	if parseErr != nil || configuration.ResponseBytes < 64<<10 || configuration.ResponseBytes > 64<<20 || configuration.Concurrent < 1 || configuration.Concurrent > 128 || configuration.WriteTimeout < time.Second || configuration.WriteTimeout > 2*time.Minute {
		return exposition.Config{}, fmt.Errorf("invalid exposition configuration")
	}
	if validateErr := configuration.Validate(); validateErr != nil {
		return exposition.Config{}, validateErr
	}
	return configuration, nil
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func parseBytes(value string) (int, error) {
	units := []struct {
		suffix string
		value  uint64
	}{{"MiB", 1 << 20}, {"KiB", 1 << 10}, {"B", 1}}
	for _, unit := range units {
		if !strings.HasSuffix(value, unit.suffix) {
			continue
		}
		digits := strings.TrimSuffix(value, unit.suffix)
		parsed, parseErr := strconv.ParseUint(digits, 10, 64)
		if parseErr != nil || digits == "" || digits[0] == '0' || parsed > uint64(math.MaxInt)/unit.value {
			return 0, fmt.Errorf("invalid byte size %q", value)
		}
		return int(parsed * unit.value), nil
	}
	return 0, fmt.Errorf("invalid byte size %q", value)
}

func parseCount(value string) (int, error) {
	parsed, err := strconv.ParseUint(value, 10, 31)
	if err != nil || value == "" || value[0] == '0' {
		return 0, fmt.Errorf("invalid count %q", value)
	}
	return int(parsed), nil
}

func parseShutdown(args []string, now time.Time, lookupEnv LookupEnv) (shutdown.Config, error) {
	configuration := shutdown.Defaults()
	values := map[string]string{}
	if lookupEnv != nil {
		for environment, property := range map[string]string{
			"METRICSHELL_SHUTDOWN_TOTAL_GRACE":      "total_grace",
			"METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT": "workload_timeout",
			"METRICSHELL_SHUTDOWN_RESERVE":          "reserve",
			"METRICSHELL_SHUTDOWN_DEADLINE":         "deadline",
		} {
			if value, exists := lookupEnv(environment); exists {
				values[property] = value
			}
		}
	}

	for index := 0; index < len(args); index++ {
		name, value, hasValue := strings.Cut(args[index], "=")
		property, known := shutdownOptions[name]
		if !known {
			return shutdown.Config{}, err.Bootstrap.UnknownOption
		}
		if !hasValue {
			index++
			if index == len(args) {
				return shutdown.Config{}, fmt.Errorf("%s requires a value", name)
			}
			value = args[index]
		}
		values[property] = value
	}

	var parseErr error
	if value, exists := values["total_grace"]; exists {
		configuration.TotalGrace, parseErr = parseDuration(value)
	}
	if parseErr == nil {
		if value, exists := values["workload_timeout"]; exists {
			configuration.WorkloadTimeout, parseErr = parseDuration(value)
		}
	}
	if parseErr == nil {
		if value, exists := values["reserve"]; exists {
			configuration.Reserve, parseErr = parseDuration(value)
		}
	}
	if parseErr == nil {
		if value, exists := values["deadline"]; exists && value != "" {
			configuration.Deadline, parseErr = time.Parse(time.RFC3339, value)
		}
	}
	if parseErr != nil {
		return shutdown.Config{}, fmt.Errorf("invalid shutdown configuration: %w", parseErr)
	}
	if validateErr := configuration.Validate(now); validateErr != nil {
		return shutdown.Config{}, validateErr
	}
	return configuration, nil
}

func parseDuration(value string) (time.Duration, error) {
	if value == "0" {
		return 0, nil
	}
	units := []struct {
		suffix string
		value  time.Duration
	}{{"ns", time.Nanosecond}, {"us", time.Microsecond}, {"ms", time.Millisecond}, {"s", time.Second}, {"m", time.Minute}, {"h", time.Hour}}
	for _, unit := range units {
		if !strings.HasSuffix(value, unit.suffix) {
			continue
		}
		digits := strings.TrimSuffix(value, unit.suffix)
		if digits == "" || digits[0] == '0' {
			break
		}
		parsed, parseErr := strconv.ParseUint(digits, 10, 64)
		if parseErr != nil || parsed > uint64(math.MaxInt64)/uint64(unit.value) {
			return 0, fmt.Errorf("duration %q overflows", value)
		}
		return time.Duration(parsed) * unit.value, nil
	}
	return 0, fmt.Errorf("duration %q does not match the value grammar", value)
}
