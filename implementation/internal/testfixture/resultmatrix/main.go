package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/Denki77/metricshell/implementation/internal/config"
	exitCodes "github.com/Denki77/metricshell/implementation/internal/constants"
)

const (
	metricshellPath = "/metricshell"
	workloadPath    = "/workload-fixture"
)

type diagnostic struct {
	Event     string `json:"event"`
	ErrorCode string `json:"error_code"`
	ExitCode  *int   `json:"exit_code"`
}

type summary struct {
	Event                 string `json:"event"`
	ConfigurationsChecked int    `json:"configurations_checked"`
	ExitsChecked          int    `json:"exits_checked"`
	SignalsChecked        int    `json:"signals_checked"`
}

func main() {
	if err := verifyRejectedConfiguration(); err != nil {
		fail(err)
	}

	for code := 0; code <= 255; code++ {
		if err := verifyWorkloadResult("FIXTURE_EXIT="+strconv.Itoa(code), code); err != nil {
			fail(err)
		}
	}

	signals := []struct {
		value syscall.Signal
		exit  int
	}{
		{value: syscall.SIGINT, exit: 130},
		{value: syscall.SIGTERM, exit: 143},
	}
	for _, signal := range signals {
		if err := verifyWorkloadResult("FIXTURE_SIGNAL="+strconv.Itoa(int(signal.value)), signal.exit); err != nil {
			fail(err)
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(summary{
		Event:                 "fixture.result_matrix",
		ConfigurationsChecked: 1,
		ExitsChecked:          256,
		SignalsChecked:        len(signals),
	}); err != nil {
		os.Exit(70)
	}
}

func verifyRejectedConfiguration() error {
	command := exec.Command(metricshellPath)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	actualExit, err := runExitCode(command)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	if actualExit != exitCodes.ExitConfigurationInvalid {
		return fmt.Errorf("configuration: exit %d, want %d; stderr=%q", actualExit, exitCodes.ExitConfigurationInvalid, stderr.String())
	}
	if stdout.Len() != 0 {
		return fmt.Errorf("configuration: stdout=%q, want empty", stdout.String())
	}

	scanner := bufio.NewScanner(&stderr)
	var records []diagnostic
	for scanner.Scan() {
		var record diagnostic
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return fmt.Errorf("configuration: invalid diagnostic: %w", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("configuration: read diagnostics: %w", err)
	}
	rejected := 0
	for _, record := range records {
		if record.Event == "configuration.rejected" {
			rejected++
			if record.ErrorCode != config.ErrorCodeConfigInvalid {
				return fmt.Errorf("configuration: error_code=%q", record.ErrorCode)
			}
		}
	}
	if rejected != 1 {
		return fmt.Errorf("configuration: rejected diagnostics=%d, want 1", rejected)
	}
	return nil
}

func verifyWorkloadResult(environment string, wantExit int) error {
	command := exec.Command(metricshellPath, "--", workloadPath)
	command.Env = []string{environment}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	actualExit, err := runExitCode(command)
	if err != nil {
		return err
	}
	if actualExit != wantExit {
		return fmt.Errorf("%s: exit %d, want %d", environment, actualExit, wantExit)
	}
	if bytes.Count(stdout.Bytes(), []byte{'\n'}) != 1 {
		return fmt.Errorf("%s: workload executions are not exactly one", environment)
	}

	started := 0
	exited := 0
	scanner := bufio.NewScanner(&stderr)
	for scanner.Scan() {
		var record diagnostic
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return fmt.Errorf("%s: invalid diagnostic: %w", environment, err)
		}
		switch record.Event {
		case "workload.started":
			started++
		case "workload.exited":
			exited++
			if record.ExitCode == nil || *record.ExitCode != wantExit {
				return fmt.Errorf("%s: workload.exited result does not match %d", environment, wantExit)
			}
		case "runtime.failed", "workload.start_failed":
			return fmt.Errorf("%s: workload result was classified as %s", environment, record.Event)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: read diagnostics: %w", environment, err)
	}
	if started != 1 || exited != 1 {
		return fmt.Errorf("%s: started=%d exited=%d, want one of each", environment, started, exited)
	}
	return nil
}

func runExitCode(command *exec.Cmd) (int, error) {
	err := command.Run()
	if err == nil {
		return 0, nil
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return 0, err
	}
	status, ok := exitError.Sys().(syscall.WaitStatus)
	if !ok || !status.Exited() {
		return 0, fmt.Errorf("metricshell did not exit normally: %w", err)
	}
	return status.ExitStatus(), nil
}

func fail(err error) {
	if _, writeErr := io.WriteString(os.Stderr, err.Error()+"\n"); writeErr != nil {
		os.Exit(70)
	}
	os.Exit(1)
}
