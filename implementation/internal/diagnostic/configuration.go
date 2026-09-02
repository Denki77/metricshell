package diagnostic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/config"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
)

type record struct {
	Timestamp     string   `json:"timestamp"`
	Sequence      uint64   `json:"sequence"`
	SchemaVersion string   `json:"schema_version"`
	Level         string   `json:"level"`
	Event         string   `json:"event"`
	Component     string   `json:"component"`
	RuntimeID     string   `json:"runtime_id"`
	State         string   `json:"state"`
	Message       string   `json:"message"`
	Reason        string   `json:"reason,omitempty"`
	ErrorCode     string   `json:"error_code,omitempty"`
	ErrorMessage  string   `json:"error_message,omitempty"`
	PreviousState string   `json:"previous_state,omitempty"`
	PID           int      `json:"pid,omitempty"`
	Deadline      string   `json:"deadline,omitempty"`
	RemainingMS   *int64   `json:"remaining_ms,omitempty"`
	DurationMS    *float64 `json:"duration_ms,omitempty"`
	Signal        string   `json:"signal,omitempty"`
	WorkloadPID   int      `json:"workload_pid,omitempty"`
	WorkloadPGID  int      `json:"workload_pgid,omitempty"`
	ExitCode      *int     `json:"exit_code,omitempty"`
	Forced        *bool    `json:"forced,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Transport     string   `json:"transport,omitempty"`
	Outcome       string   `json:"outcome,omitempty"`
	Generation    *uint64  `json:"snapshot_generation,omitempty"`
	SnapshotBytes *int     `json:"snapshot_bytes,omitempty"`
	Series        *int     `json:"series,omitempty"`
	HTTPStatus    *int     `json:"http_status,omitempty"`
}

func (logger *Logger) WriteEndpointBound(component, state string) error {
	return logger.write(record{
		Level: "info", Event: "endpoint.bound", Component: component, State: state,
		Message: "required endpoint bound",
	})
}

func (logger *Logger) WriteEndpointBindFailed(component, state string) error {
	return logger.write(record{
		Level: "error", Event: "endpoint.bind_failed", Component: component, State: state,
		Message: "required endpoint could not be bound", Reason: selfmetric.RuntimeFailureBind,
		ErrorCode: "BIND_FAILED",
	})
}

func (logger *Logger) WriteExpositionFailed(state, outcome string, status int) error {
	return logger.write(record{
		Level: "warn", Event: "exposition.failed", Component: "exposition", State: state,
		Message: "metric exposition failed", Outcome: outcome, HTTPStatus: &status,
	})
}

func (logger *Logger) WriteRuntimeInitializing(pid int) error {
	return logger.write(record{
		Level:     "info",
		Event:     "runtime.initializing",
		Component: "runtime",
		State:     "initializing",
		Message:   "runtime initializing",
		PID:       pid,
	})
}

func (logger *Logger) WriteStateChanged(previous, current string) error {
	return logger.write(record{
		Level:         "info",
		Event:         "runtime.state_changed",
		Component:     "runtime",
		State:         current,
		Message:       "runtime state changed",
		PreviousState: previous,
	})
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
		Reason:       selfmetric.RuntimeFailureConfiguration,
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
		Reason:    selfmetric.RuntimeFailureWorkloadStart,
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
		Reason:       selfmetric.RuntimeFailureInternal,
		ErrorCode:    "INTERNAL_FAILURE",
		Signal:       signal,
		WorkloadPGID: processGroupID,
	})
}

func (logger *Logger) WriteWorkloadExited(exitCode int, forced bool) error {
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

func (logger *Logger) WriteShutdownStarted(signal string, deadline time.Time, remaining time.Duration) error {
	remainingMilliseconds := remaining.Milliseconds()
	return logger.write(record{
		Level:       "info",
		Event:       "shutdown.started",
		Component:   "shutdown",
		State:       "stopping",
		Message:     "external termination started",
		Signal:      signal,
		Deadline:    deadline.UTC().Format(time.RFC3339Nano),
		RemainingMS: &remainingMilliseconds,
	})
}

func (logger *Logger) WriteShutdownForced(signal string, processGroupID int) error {
	return logger.write(record{
		Level:        "warn",
		Event:        "shutdown.forced",
		Component:    "shutdown",
		State:        "stopping",
		Message:      "workload grace expired",
		Signal:       signal,
		WorkloadPGID: processGroupID,
	})
}

func (logger *Logger) WriteShutdownCompleted(exitCode int, duration time.Duration) error {
	durationMilliseconds := float64(duration) / float64(time.Millisecond)
	return logger.write(record{
		Level:      "info",
		Event:      "shutdown.completed",
		Component:  "shutdown",
		State:      "finalizing",
		Message:    "shutdown phases completed",
		ExitCode:   &exitCode,
		DurationMS: &durationMilliseconds,
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

func (logger *Logger) WriteSnapshotAccepted(state, transport string, generation uint64, snapshotBytes, series int, duration time.Duration) error {
	durationMilliseconds := float64(duration) / float64(time.Millisecond)
	return logger.write(record{
		Level: "debug", Event: "snapshot.accepted", Component: "ingestion", State: state,
		Message: "candidate snapshot accepted", Transport: transport, Generation: &generation,
		SnapshotBytes: &snapshotBytes, Series: &series, DurationMS: &durationMilliseconds,
	})
}

func (logger *Logger) WriteSnapshotRejected(state, transport, reason string, duration time.Duration) error {
	durationMilliseconds := float64(duration) / float64(time.Millisecond)
	return logger.write(record{
		Level: "warn", Event: "snapshot.rejected", Component: "ingestion", State: state,
		Message: "candidate snapshot rejected", Transport: transport, Reason: reason,
		DurationMS: &durationMilliseconds,
	})
}

func (logger *Logger) WriteIngestionOverloaded(state, transport string) error {
	return logger.write(record{
		Level: "warn", Event: "ingestion.overloaded", Component: "ingestion", State: state,
		Message: "ingestion capacity exhausted", Transport: transport, Outcome: "busy",
	})
}

func (logger *Logger) WriteRuntimeFailed() error {
	return logger.write(record{
		Level:     "error",
		Event:     "runtime.failed",
		Component: "runtime",
		State:     "failed",
		Message:   "process supervision failed",
		Reason:    selfmetric.RuntimeFailureInternal,
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
