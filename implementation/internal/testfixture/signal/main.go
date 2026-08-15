package main

import (
	"encoding/json"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const signalTimeout = 3 * time.Second

type observation struct {
	Event  string `json:"event"`
	Signal string `json:"signal,omitempty"`
	Count  int    `json:"count,omitempty"`
}

func main() {
	received := make(chan os.Signal, 1)
	signal.Notify(received, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(received)

	if passive := os.Getenv("FIXTURE_PASSIVE_SIGNAL"); passive != "" {
		requested := parseSignals(passive)
		if len(requested) != 1 {
			os.Exit(125)
		}
		write(observation{Event: "fixture.ready"})
		waitFor(requested[0], received, 1)
		return
	}

	requested := parseSignals(os.Getenv("FIXTURE_SIGNALS"))
	write(observation{Event: "fixture.ready"})
	for index, requestedSignal := range requested {
		if err := syscall.Kill(os.Getppid(), requestedSignal.value); err != nil {
			os.Exit(125)
		}
		if os.Getenv("FIXTURE_EXIT_DURING_FORWARD") != "" {
			return
		}
		waitFor(requestedSignal, received, index+1)
	}
}

func waitFor(requested namedSignal, received <-chan os.Signal, count int) {
	select {
	case actual := <-received:
		if actual != requested.value {
			os.Exit(125)
		}
		write(observation{Event: "fixture.signal_received", Signal: requested.name, Count: count})
	case <-time.After(signalTimeout):
		os.Exit(125)
	}
}

type namedSignal struct {
	name  string
	value syscall.Signal
}

func parseSignals(value string) []namedSignal {
	if value == "" {
		value = "TERM"
	}
	parts := strings.Split(value, ",")
	result := make([]namedSignal, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "TERM":
			result = append(result, namedSignal{name: part, value: syscall.SIGTERM})
		case "INT":
			result = append(result, namedSignal{name: part, value: syscall.SIGINT})
		case "HUP":
			result = append(result, namedSignal{name: part, value: syscall.SIGHUP})
		case "QUIT":
			result = append(result, namedSignal{name: part, value: syscall.SIGQUIT})
		default:
			os.Exit(125)
		}
	}
	return result
}

func write(value observation) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		os.Exit(125)
	}
}
