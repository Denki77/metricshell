package managedclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	ExitAccepted  = 0
	ExitLocal     = 2
	ExitRejected  = 3
	ExitOverload  = 4
	ExitProtocol  = 5
	ExitTransport = 6
	ExitUnknown   = 7
)

type LookupEnv func(string) (string, bool)

func RunCLI(args []string, stdout, stderr io.Writer, lookup LookupEnv) int {
	configuration := Config{SocketPath: "/run/metricshell/managed.sock", Timeout: 5 * time.Second}
	if lookup != nil {
		if value, exists := lookup("METRICSHELL_MANAGED_SOCKET_PATH"); exists {
			configuration.SocketPath = value
		}
		if value, exists := lookup("METRICSHELL_MANAGED_CLIENT_TIMEOUT"); exists {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return localError(stderr, "invalid client timeout")
			}
			configuration.Timeout = duration
		}
	}
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		name, value, found := strings.Cut(args[0], "=")
		if !found || (name != "--socket" && name != "--timeout") {
			return localError(stderr, "managed client options require --name=value")
		}
		switch name {
		case "--socket":
			configuration.SocketPath = value
		case "--timeout":
			duration, err := time.ParseDuration(value)
			if err != nil {
				return localError(stderr, "invalid client timeout")
			}
			configuration.Timeout = duration
		}
		args = args[1:]
	}
	if configuration.SocketPath == "" || configuration.Timeout <= 0 || configuration.Timeout > time.Minute {
		return localError(stderr, "invalid managed client configuration")
	}
	request, err := parseCommand(args)
	if err != nil {
		return localError(stderr, err.Error())
	}
	frame, err := EncodeRequest(request)
	if err != nil {
		return localError(stderr, "cannot encode request")
	}
	result := Send(context.Background(), configuration, frame)
	encoded, _ := json.Marshal(result)
	_, _ = fmt.Fprintln(stdout, string(encoded))
	switch result.Category {
	case Accepted:
		return ExitAccepted
	case Rejected:
		return ExitRejected
	case Overload:
		return ExitOverload
	case Protocol:
		return ExitProtocol
	case Transport:
		return ExitTransport
	default:
		return ExitUnknown
	}
}

func parseCommand(args []string) (map[string]any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("managed operation is required")
	}
	operation := strings.ReplaceAll(args[0], "-", "_")
	if operation == "declare" {
		if len(args) < 4 {
			return nil, fmt.Errorf("usage: managed declare NAME TYPE HELP [LABELS [BUCKET ...]]")
		}
		labels := []string{}
		if len(args) > 4 && args[4] != "-" {
			labels = strings.Split(args[4], ",")
		}
		buckets := []string{}
		if len(args) > 5 {
			buckets = append(buckets, args[5:]...)
		}
		return map[string]any{
			"version": 1, "op": "declare", "name": args[1], "type": args[2], "help": args[3],
			"label_names": labels, "buckets": buckets,
		}, nil
	}
	if operation != "counter_initialize" && operation != "counter_add" && operation != "gauge_set" && operation != "histogram_observe" {
		return nil, fmt.Errorf("unknown managed operation")
	}
	if len(args) < 3 {
		return nil, fmt.Errorf("usage: managed OPERATION NAME VALUE [LABEL=VALUE ...]")
	}
	if _, err := strconv.ParseFloat(args[2], 64); err != nil {
		return nil, fmt.Errorf("invalid operation value")
	}
	labels := map[string]string{}
	for _, input := range args[3:] {
		name, value, found := strings.Cut(input, "=")
		if !found || name == "" {
			return nil, fmt.Errorf("labels use NAME=VALUE")
		}
		if _, exists := labels[name]; exists {
			return nil, fmt.Errorf("duplicate label")
		}
		labels[name] = value
	}
	return map[string]any{"version": 1, "op": operation, "name": args[1], "value": args[2], "labels": labels}, nil
}

func localError(destination io.Writer, message string) int {
	_, _ = fmt.Fprintln(destination, message)
	return ExitLocal
}
