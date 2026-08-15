package diagnostic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/config"
)

type configurationRejected struct {
	Timestamp     string `json:"timestamp"`
	Sequence      uint64 `json:"sequence"`
	SchemaVersion string `json:"schema_version"`
	Level         string `json:"level"`
	Event         string `json:"event"`
	Component     string `json:"component"`
	RuntimeID     string `json:"runtime_id"`
	State         string `json:"state"`
	Message       string `json:"message"`
	Reason        string `json:"reason"`
	ErrorCode     string `json:"error_code"`
	ErrorMessage  string `json:"error_message"`
}

type workloadStartFailed struct {
	Timestamp     string `json:"timestamp"`
	Sequence      uint64 `json:"sequence"`
	SchemaVersion string `json:"schema_version"`
	Level         string `json:"level"`
	Event         string `json:"event"`
	Component     string `json:"component"`
	RuntimeID     string `json:"runtime_id"`
	State         string `json:"state"`
	Message       string `json:"message"`
	Reason        string `json:"reason"`
	ErrorCode     string `json:"error_code"`
}

type workloadStarted struct {
	Timestamp     string `json:"timestamp"`
	Sequence      uint64 `json:"sequence"`
	SchemaVersion string `json:"schema_version"`
	Level         string `json:"level"`
	Event         string `json:"event"`
	Component     string `json:"component"`
	RuntimeID     string `json:"runtime_id"`
	State         string `json:"state"`
	Message       string `json:"message"`
	WorkloadPID   int    `json:"workload_pid"`
	WorkloadPGID  int    `json:"workload_pgid"`
}

// WriteConfigurationRejected emits the normative startup failure as one JSON Lines record.
func WriteConfigurationRejected(dst io.Writer, now time.Time, err error) error {
	record := configurationRejected{
		Timestamp:     now.UTC().Format(time.RFC3339Nano),
		Sequence:      1,
		SchemaVersion: "1",
		Level:         "error",
		Event:         "configuration.rejected",
		Component:     "runtime",
		RuntimeID:     runtimeID(),
		State:         "initializing",
		Message:       "startup configuration rejected",
		Reason:        config.ReasonConfiguration,
		ErrorCode:     config.ErrorCodeConfigInvalid,
		ErrorMessage:  err.Error(),
	}
	return json.NewEncoder(dst).Encode(record)
}

func WriteWorkloadStartFailed(dst io.Writer, now time.Time) error {
	record := workloadStartFailed{
		Timestamp:     now.UTC().Format(time.RFC3339Nano),
		Sequence:      1,
		SchemaVersion: "1",
		Level:         "error",
		Event:         "workload.start_failed",
		Component:     "workload",
		RuntimeID:     runtimeID(),
		State:         "failed",
		Message:       "workload could not be started",
		Reason:        "workload_start",
		ErrorCode:     "WORKLOAD_START_FAILED",
	}
	return json.NewEncoder(dst).Encode(record)
}

func WriteWorkloadStarted(dst io.Writer, now time.Time, pid, processGroupID int) error {
	record := workloadStarted{
		Timestamp:     now.UTC().Format(time.RFC3339Nano),
		Sequence:      1,
		SchemaVersion: "1",
		Level:         "info",
		Event:         "workload.started",
		Component:     "workload",
		RuntimeID:     runtimeID(),
		State:         "running",
		Message:       "workload started",
		WorkloadPID:   pid,
		WorkloadPGID:  processGroupID,
	}
	return json.NewEncoder(dst).Encode(record)
}

func runtimeID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(value[:])
}
