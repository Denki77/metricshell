package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type event struct {
	Event string `json:"event"`
	Mode  string `json:"mode"`
}

func main() {
	mode := os.Getenv("TERMINATION_MODE")
	if mode == "child" {
		signal.Ignore(syscall.SIGTERM, syscall.SIGINT)
		select {}
	}
	if mode != "cooperative" && mode != "ignore" && mode != "fork-after-signal" {
		os.Exit(64)
	}

	received := make(chan os.Signal, 2)
	signal.Notify(received, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(received)
	write(event{Event: "fixture.ready", Mode: mode})

	for {
		<-received
		switch mode {
		case "cooperative":
			os.Exit(0)
		case "fork-after-signal":
			startChild()
			mode = "ignore"
		}
	}
}

func startChild() {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(70)
	}
	command := exec.Command(executable)
	command.Env = []string{"TERMINATION_MODE=child"}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		os.Exit(70)
	}
}

func write(value event) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		os.Exit(70)
	}
}
