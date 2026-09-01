package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	signalTimeout          = 3 * time.Second
	signalObservationGrace = 250 * time.Millisecond
)

const (
	exitedTargetHelperEnvironment = "FIXTURE_EXITED_TARGET_HELPER"
	exitedTargetSupervisorPID     = "FIXTURE_EXITED_TARGET_SUPERVISOR_PID"
	exitedTargetProcessGroupID    = "FIXTURE_EXITED_TARGET_PROCESS_GROUP_ID"
	exitedTargetSignal            = "FIXTURE_EXITED_TARGET_SIGNAL"
)

type observation struct {
	Event  string `json:"event"`
	Signal string `json:"signal,omitempty"`
	Count  int    `json:"count,omitempty"`
}

func main() {
	if os.Getenv(exitedTargetHelperEnvironment) != "" {
		runExitedTargetHelper()
		return
	}

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
		if os.Getenv("FIXTURE_EXIT_DURING_FORWARD") != "" {
			startExitedTargetHelper(requestedSignal)
			return
		}
		if err := syscall.Kill(os.Getppid(), requestedSignal.value); err != nil {
			os.Exit(125)
		}
		waitFor(requestedSignal, received, index+1)
	}
}

func startExitedTargetHelper(requested namedSignal) {
	command := exec.Command(os.Args[0])
	command.Env = append(os.Environ(),
		exitedTargetHelperEnvironment+"=1",
		exitedTargetSupervisorPID+"="+strconv.Itoa(os.Getppid()),
		exitedTargetProcessGroupID+"="+strconv.Itoa(syscall.Getpgrp()),
		exitedTargetSignal+"="+requested.name,
	)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		os.Exit(125)
	}
}

func runExitedTargetHelper() {
	supervisorPID := environmentPID(exitedTargetSupervisorPID)
	processGroupID := environmentPID(exitedTargetProcessGroupID)
	requested := parseSignals(os.Getenv(exitedTargetSignal))
	if len(requested) != 1 {
		os.Exit(125)
	}

	deadline := time.Now().Add(signalTimeout)
	for {
		err := syscall.Kill(-processGroupID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if err != nil || time.Now().After(deadline) {
			os.Exit(125)
		}
		time.Sleep(time.Millisecond)
	}
	if err := syscall.Kill(supervisorPID, requested[0].value); err != nil {
		os.Exit(125)
	}
	// Remain adopted until MetricShell has consumed the signal; otherwise it may finish reaping and stop notification.
	time.Sleep(signalObservationGrace)
}

func environmentPID(name string) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < 1 {
		os.Exit(125)
	}
	return value
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
