package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	err "github.com/Denki77/metricshell/implementation/internal/error"
	"github.com/Denki77/metricshell/implementation/internal/exposition"
	"github.com/Denki77/metricshell/implementation/internal/finalwait"
	"github.com/Denki77/metricshell/implementation/internal/shutdown"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

const (
	ErrorCodeConfigInvalid = "CONFIG_INVALID"
)

type LookupEnv func(string) (string, bool)

type Config struct {
	Workload            []string
	Shutdown            shutdown.Config
	Exposition          exposition.Config
	FinalWait           finalwait.Config
	IngestionTransport  string
	Limits              snapshot.Limits
	ConcurrentIngestion int
	PendingIngestion    int
	File                FileConfig
	UnixSocketPath      string
	Socket              SocketConfig
	HTTPIngestionListen string
	HTTPIngestion       HTTPConfig
}

type FileConfig struct {
	Path              string
	ReconcileInterval time.Duration
	DecodedBytes      int
}

type SocketConfig struct {
	FrameBytes         int
	Parts              int
	Connections        int
	Transactions       int
	TransactionTimeout time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	DecodedBytes       int
	SnapshotBytes      int
}

type HTTPConfig struct {
	WireBytes         int
	DecodedBytes      int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

var options = map[string]string{
	"--shutdown-total-grace":          "total_grace",
	"--workload-shutdown-timeout":     "workload_timeout",
	"--shutdown-reserve":              "reserve",
	"--shutdown-deadline":             "deadline",
	"--exposition-listen":             "exposition_listen",
	"--max-response-bytes":            "response_bytes",
	"--max-concurrent-scrapes":        "concurrent_scrapes",
	"--exposition-write-timeout":      "exposition_write_timeout",
	"--metrics-include":               "metrics_include",
	"--metrics-exclude":               "metrics_exclude",
	"--ingestion-transport":           "ingestion_transport",
	"--http-ingestion-listen":         "http_ingestion_listen",
	"--unix-socket-path":              "unix_socket_path",
	"--snapshot-file-path":            "snapshot_file_path",
	"--file-reconcile-interval":       "file_reconcile_interval",
	"--socket-max-frame-bytes":        "socket_frame_bytes",
	"--socket-max-parts":              "socket_parts",
	"--socket-max-connections":        "socket_connections",
	"--socket-max-transactions":       "socket_transactions",
	"--socket-transaction-timeout":    "socket_transaction_timeout",
	"--socket-read-timeout":           "socket_read_timeout",
	"--socket-write-timeout":          "socket_write_timeout",
	"--http-ingestion-max-wire-bytes": "http_ingestion_wire_bytes",
	"--http-read-header-timeout":      "http_read_header_timeout",
	"--http-read-timeout":             "http_read_timeout",
	"--http-write-timeout":            "http_write_timeout",
	"--http-idle-timeout":             "http_idle_timeout",
	"--http-max-header-bytes":         "http_max_header_bytes",
	"--max-snapshot-bytes":            "snapshot_bytes",
	"--max-decoded-input-bytes":       "decoded_input_bytes",
	"--max-series":                    "series",
	"--max-labels-per-series":         "labels_per_series",
	"--max-metric-name-bytes":         "metric_name_bytes",
	"--max-label-name-bytes":          "label_name_bytes",
	"--max-label-value-bytes":         "label_value_bytes",
	"--max-help-bytes":                "help_bytes",
	"--max-concurrent-ingestions":     "concurrent_ingestions",
	"--max-pending-ingestions":        "pending_ingestions",
	"--final-wait-mode":               "final_wait_mode",
	"--final-wait-duration":           "final_wait_duration",
	"--final-wait-timeout":            "final_wait_timeout",
	"--final-wait-required-scrapes":   "final_wait_required_scrapes",
	"--final-wait-completion-grace":   "final_wait_completion_grace",
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
	finalWaitConfiguration, parseErr := parseFinalWait(args[:separator], lookupEnv)
	if parseErr != nil {
		return Config{}, parseErr
	}
	ingestionConfiguration, parseErr := parseIngestion(args[:separator], lookupEnv)
	if parseErr != nil {
		return Config{}, parseErr
	}
	ingestionConfiguration.Workload = args[separator+1:]
	ingestionConfiguration.Shutdown = shutdownConfiguration
	ingestionConfiguration.Exposition = expositionConfiguration
	ingestionConfiguration.FinalWait = finalWaitConfiguration
	return ingestionConfiguration, nil
}

func parseIngestion(args []string, lookupEnv LookupEnv) (Config, error) {
	limits := snapshot.DefaultLimits()
	fileConfiguration := FileConfig{
		Path: "/run/metricshell/snapshot.json", ReconcileInterval: time.Second,
		DecodedBytes: limits.DecodedBytes,
	}
	socketConfiguration := SocketConfig{
		FrameBytes: 8 << 10, Parts: 256, Connections: 8, Transactions: 4,
		TransactionTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
		DecodedBytes: limits.DecodedBytes, SnapshotBytes: limits.SnapshotBytes,
	}
	httpConfiguration := HTTPConfig{
		WireBytes: 2 << 20, DecodedBytes: limits.DecodedBytes, ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second,
		MaxHeaderBytes: 8 << 10,
	}
	configuration := Config{
		IngestionTransport: "unix", Limits: limits, ConcurrentIngestion: 4, PendingIngestion: 0,
		File: fileConfiguration, UnixSocketPath: "/run/metricshell/ingest.sock",
		Socket: socketConfiguration, HTTPIngestionListen: "127.0.0.1:9091", HTTPIngestion: httpConfiguration,
	}
	values := map[string]string{}
	if lookupEnv != nil {
		for environment, property := range map[string]string{
			"METRICSHELL_INGESTION_TRANSPORT":           "ingestion_transport",
			"METRICSHELL_HTTP_INGESTION_LISTEN":         "http_ingestion_listen",
			"METRICSHELL_UNIX_SOCKET_PATH":              "unix_socket_path",
			"METRICSHELL_SNAPSHOT_FILE_PATH":            "snapshot_file_path",
			"METRICSHELL_FILE_RECONCILE_INTERVAL":       "file_reconcile_interval",
			"METRICSHELL_SOCKET_MAX_FRAME_BYTES":        "socket_frame_bytes",
			"METRICSHELL_SOCKET_MAX_PARTS":              "socket_parts",
			"METRICSHELL_SOCKET_MAX_CONNECTIONS":        "socket_connections",
			"METRICSHELL_SOCKET_MAX_TRANSACTIONS":       "socket_transactions",
			"METRICSHELL_SOCKET_TRANSACTION_TIMEOUT":    "socket_transaction_timeout",
			"METRICSHELL_SOCKET_READ_TIMEOUT":           "socket_read_timeout",
			"METRICSHELL_SOCKET_WRITE_TIMEOUT":          "socket_write_timeout",
			"METRICSHELL_HTTP_INGESTION_MAX_WIRE_BYTES": "http_ingestion_wire_bytes",
			"METRICSHELL_HTTP_READ_HEADER_TIMEOUT":      "http_read_header_timeout",
			"METRICSHELL_HTTP_READ_TIMEOUT":             "http_read_timeout",
			"METRICSHELL_HTTP_WRITE_TIMEOUT":            "http_write_timeout",
			"METRICSHELL_HTTP_IDLE_TIMEOUT":             "http_idle_timeout",
			"METRICSHELL_HTTP_MAX_HEADER_BYTES":         "http_max_header_bytes",
			"METRICSHELL_MAX_SNAPSHOT_BYTES":            "snapshot_bytes",
			"METRICSHELL_MAX_DECODED_INPUT_BYTES":       "decoded_input_bytes",
			"METRICSHELL_MAX_SERIES":                    "series",
			"METRICSHELL_MAX_LABELS_PER_SERIES":         "labels_per_series",
			"METRICSHELL_MAX_METRIC_NAME_BYTES":         "metric_name_bytes",
			"METRICSHELL_MAX_LABEL_NAME_BYTES":          "label_name_bytes",
			"METRICSHELL_MAX_LABEL_VALUE_BYTES":         "label_value_bytes",
			"METRICSHELL_MAX_HELP_BYTES":                "help_bytes",
			"METRICSHELL_MAX_CONCURRENT_INGESTIONS":     "concurrent_ingestions",
			"METRICSHELL_MAX_PENDING_INGESTIONS":        "pending_ingestions",
		} {
			if value, exists := lookupEnv(environment); exists {
				values[property] = value
			}
		}
	}
	for index := 0; index < len(args); index++ {
		name, value, hasValue := strings.Cut(args[index], "=")
		property, known := options[name]
		if !known {
			return Config{}, err.Bootstrap.UnknownOption
		}
		if !hasValue {
			index++
			if index == len(args) {
				return Config{}, fmt.Errorf("%s requires a value", name)
			}
			value = args[index]
		}
		values[property] = value
	}
	var parseErr error
	if value, exists := values["ingestion_transport"]; exists {
		configuration.IngestionTransport = value
	}
	if value, exists := values["snapshot_file_path"]; exists {
		configuration.File.Path = value
	}
	if value, exists := values["unix_socket_path"]; exists {
		configuration.UnixSocketPath = value
	}
	if value, exists := values["http_ingestion_listen"]; exists {
		configuration.HTTPIngestionListen = value
	}
	if value, exists := values["file_reconcile_interval"]; exists {
		configuration.File.ReconcileInterval, parseErr = parseDuration(value)
	}
	if parseErr == nil {
		parseErr = parseIngestionCounts(values, &configuration)
	}
	if parseErr != nil {
		return Config{}, fmt.Errorf("invalid ingestion configuration: %w", parseErr)
	}
	configuration.File.DecodedBytes = configuration.Limits.DecodedBytes
	configuration.Socket.DecodedBytes = configuration.Limits.DecodedBytes
	configuration.Socket.SnapshotBytes = configuration.Limits.SnapshotBytes
	configuration.HTTPIngestion.DecodedBytes = configuration.Limits.DecodedBytes
	if !validIngestionTransport(configuration.IngestionTransport) ||
		configuration.File.ReconcileInterval < 100*time.Millisecond || configuration.File.ReconcileInterval > time.Second ||
		configuration.ConcurrentIngestion < 1 || configuration.ConcurrentIngestion > 64 ||
		configuration.PendingIngestion < 0 || configuration.PendingIngestion > 64 ||
		configuration.Limits.SnapshotBytes < 64<<10 || configuration.Limits.SnapshotBytes > 64<<20 ||
		configuration.Limits.DecodedBytes < 64<<10 || configuration.Limits.DecodedBytes > 128<<20 ||
		configuration.Limits.DecodedBytes < configuration.Limits.SnapshotBytes ||
		configuration.Limits.Series < 1 || configuration.Limits.Series > 100_000 ||
		configuration.Limits.LabelsPerSeries < 0 || configuration.Limits.LabelsPerSeries > 64 ||
		configuration.Limits.MetricNameBytes < 1 || configuration.Limits.MetricNameBytes > 1024 ||
		configuration.Limits.LabelNameBytes < 1 || configuration.Limits.LabelNameBytes > 1024 ||
		configuration.Limits.LabelValueBytes < 1 || configuration.Limits.LabelValueBytes > 16<<10 ||
		configuration.Limits.HelpBytes < 0 || configuration.Limits.HelpBytes > 64<<10 ||
		configuration.Socket.FrameBytes < 1<<10 || configuration.Socket.FrameBytes > 64<<10 ||
		configuration.Socket.Parts < 1 || configuration.Socket.Parts > 1024 ||
		configuration.Socket.Connections < 1 || configuration.Socket.Connections > 64 ||
		configuration.Socket.Transactions < 1 || configuration.Socket.Transactions > 32 ||
		configuration.Socket.TransactionTimeout < 100*time.Millisecond || configuration.Socket.TransactionTimeout > time.Minute ||
		configuration.Socket.ReadTimeout < 100*time.Millisecond || configuration.Socket.ReadTimeout > time.Minute ||
		configuration.Socket.WriteTimeout < 100*time.Millisecond || configuration.Socket.WriteTimeout > time.Minute ||
		configuration.HTTPIngestion.WireBytes < 64<<10 || configuration.HTTPIngestion.WireBytes > 128<<20 ||
		configuration.HTTPIngestion.ReadHeaderTimeout < 100*time.Millisecond || configuration.HTTPIngestion.ReadHeaderTimeout > 30*time.Second ||
		configuration.HTTPIngestion.ReadTimeout < 100*time.Millisecond || configuration.HTTPIngestion.ReadTimeout > time.Minute ||
		configuration.HTTPIngestion.WriteTimeout < 100*time.Millisecond || configuration.HTTPIngestion.WriteTimeout > 2*time.Minute ||
		configuration.HTTPIngestion.IdleTimeout < time.Second || configuration.HTTPIngestion.IdleTimeout > 5*time.Minute ||
		configuration.HTTPIngestion.MaxHeaderBytes < 1<<10 || configuration.HTTPIngestion.MaxHeaderBytes > 64<<10 ||
		effectiveSocketDecodedCapacity(configuration.Socket.FrameBytes, configuration.Socket.Parts) < configuration.Limits.SnapshotBytes {
		return Config{}, fmt.Errorf("invalid ingestion configuration")
	}
	return configuration, nil
}

func parseIngestionCounts(values map[string]string, configuration *Config) error {
	counts := map[string]*int{
		"socket_frame_bytes":        &configuration.Socket.FrameBytes,
		"socket_parts":              &configuration.Socket.Parts,
		"socket_connections":        &configuration.Socket.Connections,
		"socket_transactions":       &configuration.Socket.Transactions,
		"http_ingestion_wire_bytes": &configuration.HTTPIngestion.WireBytes,
		"http_max_header_bytes":     &configuration.HTTPIngestion.MaxHeaderBytes,
		"snapshot_bytes":            &configuration.Limits.SnapshotBytes,
		"decoded_input_bytes":       &configuration.Limits.DecodedBytes,
		"series":                    &configuration.Limits.Series,
		"labels_per_series":         &configuration.Limits.LabelsPerSeries,
		"metric_name_bytes":         &configuration.Limits.MetricNameBytes,
		"label_name_bytes":          &configuration.Limits.LabelNameBytes,
		"label_value_bytes":         &configuration.Limits.LabelValueBytes,
		"help_bytes":                &configuration.Limits.HelpBytes,
		"concurrent_ingestions":     &configuration.ConcurrentIngestion,
		"pending_ingestions":        &configuration.PendingIngestion,
	}
	for property, target := range counts {
		value, exists := values[property]
		if !exists {
			continue
		}
		parsed, parseErr := parseBytesOrCount(property, value)
		if parseErr != nil {
			return parseErr
		}
		*target = parsed
	}
	durations := map[string]*time.Duration{
		"socket_transaction_timeout": &configuration.Socket.TransactionTimeout,
		"socket_read_timeout":        &configuration.Socket.ReadTimeout,
		"socket_write_timeout":       &configuration.Socket.WriteTimeout,
		"http_read_header_timeout":   &configuration.HTTPIngestion.ReadHeaderTimeout,
		"http_read_timeout":          &configuration.HTTPIngestion.ReadTimeout,
		"http_write_timeout":         &configuration.HTTPIngestion.WriteTimeout,
		"http_idle_timeout":          &configuration.HTTPIngestion.IdleTimeout,
	}
	for property, target := range durations {
		value, exists := values[property]
		if !exists {
			continue
		}
		parsed, parseErr := parseDuration(value)
		if parseErr != nil {
			return parseErr
		}
		*target = parsed
	}
	return nil
}

func parseBytesOrCount(property, value string) (int, error) {
	if property == "labels_per_series" || property == "help_bytes" || property == "pending_ingestions" {
		return parseNonNegativeCount(value)
	}
	if strings.Contains(property, "bytes") {
		return parseBytes(value)
	}
	return parseCount(value)
}

func parseNonNegativeCount(value string) (int, error) {
	if value == "0" {
		return 0, nil
	}
	return parseCount(value)
}

func validIngestionTransport(transport string) bool {
	return transport == "file" || transport == "unix" || transport == "http"
}

func effectiveSocketDecodedCapacity(frameBytes, parts int) int {
	total := 0
	for index := 0; index < parts; index++ {
		payloadCharacters := frameBytes - len("MSP/1 SNAPSHOT_PART ") - 64 - 1 - len(strconv.Itoa(index)) - 1 - 1
		if payloadCharacters > 0 {
			total += payloadCharacters * 3 / 4
		}
	}
	return total
}

func parseFinalWait(args []string, lookupEnv LookupEnv) (finalwait.Config, error) {
	configuration := finalwait.Defaults()
	values := map[string]string{}
	if lookupEnv != nil {
		for environment, property := range map[string]string{
			"METRICSHELL_FINAL_WAIT_MODE":             "final_wait_mode",
			"METRICSHELL_FINAL_WAIT_DURATION":         "final_wait_duration",
			"METRICSHELL_FINAL_WAIT_TIMEOUT":          "final_wait_timeout",
			"METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES": "final_wait_required_scrapes",
			"METRICSHELL_FINAL_WAIT_COMPLETION_GRACE": "final_wait_completion_grace",
		} {
			if value, exists := lookupEnv(environment); exists {
				values[property] = value
			}
		}
	}
	for index := 0; index < len(args); index++ {
		name, value, hasValue := strings.Cut(args[index], "=")
		property, known := options[name]
		if !known {
			return finalwait.Config{}, err.Bootstrap.UnknownOption
		}
		if !hasValue {
			index++
			if index == len(args) {
				return finalwait.Config{}, fmt.Errorf("%s requires a value", name)
			}
			value = args[index]
		}
		values[property] = value
	}
	var parseErr error
	if value, exists := values["final_wait_mode"]; exists {
		configuration.Mode = finalwait.Mode(value)
	}
	if value, exists := values["final_wait_duration"]; exists {
		configuration.Duration, parseErr = parseDuration(value)
	}
	if parseErr == nil {
		if value, exists := values["final_wait_timeout"]; exists {
			configuration.Timeout, parseErr = parseDuration(value)
		}
	}
	if parseErr == nil {
		if value, exists := values["final_wait_required_scrapes"]; exists {
			configuration.RequiredScrapes, parseErr = parseCount(value)
		}
	}
	if parseErr == nil {
		if value, exists := values["final_wait_completion_grace"]; exists {
			configuration.CompletionGrace, parseErr = parseDuration(value)
		}
	}
	if parseErr != nil || configuration.Validate() != nil {
		return finalwait.Config{}, fmt.Errorf("invalid final-wait configuration")
	}
	return configuration, nil
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
		property, known := options[name]
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
		property, known := options[name]
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
