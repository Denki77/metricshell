package diagnostic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/config"
)

type record struct {
	Timestamp     string `json:"timestamp"`
	Sequence      uint64 `json:"sequence"`
	SchemaVersion string `json:"schema_version"`
	Level         string `json:"level"`
	Event         string `json:"event"`
	Component     string `json:"component"`
	RuntimeID     string `json:"runtime_id"`
	State         string `json:"state"`
	Message       string `json:"message"`
	Reason        string `json:"reason,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
	Signal        string `json:"signal,omitempty"`
	WorkloadPID   int    `json:"workload_pid,omitempty"`
	WorkloadPGID  int    `json:"workload_pgid,omitempty"`
	ExitCode      *int   `json:"exit_code,omitempty"`
	Forced        *bool  `json:"forced,omitempty"`
	Kind          string `json:"kind,omitempty"`
}

type Logger struct {
	destination io.Writer
	now         func() time.Time
	runtimeID   string

	mu       sync.Mutex
	sequence uint64
}

func New(destination io.Writer, now func() time.Time) *Logger {
	return &Logger{destination: destination, now: now, runtimeID: newRuntimeID()}
}

func (logger *Logger) WriteConfigurationRejected(err error) error {
	return logger.write(record{
		Level:        "error",
		Event:        "configuration.rejected",
		Component:    "runtime",
		State:        "initializing",
		Message:      "startup configuration rejected",
		Reason:       config.ReasonConfiguration,
		ErrorCode:    config.ErrorCodeConfigInvalid,
		ErrorMessage: err.Error(),
	})
}

func (logger *Logger) WriteWorkloadStartFailed() error {
	return logger.write(record{
		Level:     "error",
		Event:     "workload.start_failed",
		Component: "workload",
		State:     "failed",
		Message:   "workload could not be started",
		Reason:    "workload_start",
		ErrorCode: "WORKLOAD_START_FAILED",
	})
}

func (logger *Logger) WriteWorkloadStarted(pid, processGroupID int) error {
	return logger.write(record{
		Level:        "info",
		Event:        "workload.started",
		Component:    "workload",
		State:        "running",
		Message:      "workload started",
		WorkloadPID:  pid,
		WorkloadPGID: processGroupID,
	})
}

func (logger *Logger) WriteSignalForwarded(signal, state string, processGroupID int) error {
	return logger.write(record{
		Level:        "info",
		Event:        "workload.signal_forwarded",
		Component:    "workload",
		State:        state,
		Message:      "signal forwarded to workload group",
		Signal:       signal,
		WorkloadPGID: processGroupID,
	})
}

func (logger *Logger) WriteSignalIgnored(signal, reason string, processGroupID int) error {
	return logger.write(record{
		Level:        "info",
		Event:        "workload.signal_ignored",
		Component:    "workload",
		State:        "finalizing",
		Message:      "late signal ignored",
		Reason:       reason,
		Signal:       signal,
		WorkloadPGID: processGroupID,
	})
}

func (logger *Logger) WriteSignalFailed(signal string, processGroupID int) error {
	return logger.write(record{
		Level:        "error",
		Event:        "runtime.failed",
		Component:    "runtime",
		State:        "failed",
		Message:      "signal could not be forwarded",
		Reason:       "internal",
		ErrorCode:    "INTERNAL_FAILURE",
		Signal:       signal,
		WorkloadPGID: processGroupID,
	})
}

func (logger *Logger) WriteWorkloadExited(exitCode int) error {
	forced := false
	return logger.write(record{
		Level:     "info",
		Event:     "workload.exited",
		Component: "workload",
		State:     "finalizing",
		Message:   "primary workload exited",
		ExitCode:  &exitCode,
		Forced:    &forced,
	})
}

func (logger *Logger) WriteChildReaped(kind, state string) error {
	return logger.write(record{
		Level:     "debug",
		Event:     "child.reaped",
		Component: "workload",
		State:     state,
		Message:   "managed child reaped",
		Kind:      kind,
	})
}

func (logger *Logger) WriteRuntimeFailed() error {
	return logger.write(record{
		Level:     "error",
		Event:     "runtime.failed",
		Component: "runtime",
		State:     "failed",
		Message:   "process supervision failed",
		Reason:    "internal",
		ErrorCode: "INTERNAL_FAILURE",
	})
}

func (logger *Logger) write(value record) error {
	logger.mu.Lock()
	defer logger.mu.Unlock()

	logger.sequence++
	value.Timestamp = logger.now().UTC().Format(time.RFC3339Nano)
	value.Sequence = logger.sequence
	value.SchemaVersion = "1"
	value.RuntimeID = logger.runtimeID
	return json.NewEncoder(logger.destination).Encode(value)
}

func newRuntimeID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(value[:])
}
