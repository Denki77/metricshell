package managedclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedprotocol"
)

type Category string

const (
	Accepted  Category = "accepted"
	Rejected  Category = "rejected"
	Overload  Category = "overload"
	Protocol  Category = "protocol"
	Transport Category = "transport"
	Unknown   Category = "unknown"
)

type Config struct {
	SocketPath string
	Timeout    time.Duration
}

type Result struct {
	Category   Category `json:"category"`
	Reason     string   `json:"reason,omitempty"`
	Generation uint64   `json:"generation,omitempty"`
	Commit     uint64   `json:"commit,omitempty"`
}

func Send(ctx context.Context, configuration Config, frame []byte) Result {
	if configuration.SocketPath == "" || configuration.Timeout <= 0 || len(frame) == 0 || frame[len(frame)-1] != '\n' {
		return Result{Category: Protocol, Reason: "invalid_client_configuration"}
	}
	dialer := net.Dialer{Timeout: configuration.Timeout}
	connection, err := dialer.DialContext(ctx, "unix", configuration.SocketPath)
	if err != nil {
		return Result{Category: Transport, Reason: "connect"}
	}
	defer connection.Close()
	deadline := time.Now().Add(configuration.Timeout)
	if err := connection.SetDeadline(deadline); err != nil {
		return Result{Category: Transport, Reason: "deadline"}
	}
	written := 0
	for written < len(frame) {
		count, writeErr := connection.Write(frame[written:])
		written += count
		if writeErr != nil {
			if written == 0 {
				return Result{Category: Transport, Reason: "write"}
			}
			return Result{Category: Unknown, Reason: "write_after_submission"}
		}
	}
	line, err := bufio.NewReader(connection).ReadBytes('\n')
	if err != nil {
		return Result{Category: Unknown, Reason: "missing_response"}
	}
	var response managedprotocol.Response
	if err := json.Unmarshal(line, &response); err != nil || response.Version != managedprotocol.Version {
		return Result{Category: Protocol, Reason: "invalid_response"}
	}
	result := Result{Reason: response.Reason, Generation: response.Generation, Commit: response.Commit}
	switch response.Outcome {
	case managed.OutcomeCommitted:
		result.Category = Accepted
	case managed.OutcomeRejected, managed.OutcomeClosed, managed.OutcomeCancelled:
		result.Category = Rejected
	case managed.OutcomeOverloaded:
		result.Category = Overload
	case managed.OutcomeUnknown:
		result.Category = Unknown
	case "protocol":
		result.Category = Protocol
	default:
		result.Category, result.Reason = Protocol, "invalid_response_outcome"
	}
	return result
}

func EncodeRequest(request map[string]any) ([]byte, error) {
	if len(request) == 0 {
		return nil, errors.New("empty request")
	}
	content, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
