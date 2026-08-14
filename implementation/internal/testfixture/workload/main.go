package main

import (
	"encoding/json"
	"os"
	"strconv"
	"syscall"
	"time"
)

type observation struct {
	Argv []string `json:"argv"`
	PID  int      `json:"pid"`
	PPID int      `json:"ppid"`
}

func main() {
	_ = json.NewEncoder(os.Stdout).Encode(observation{Argv: os.Args[1:], PID: os.Getpid(), PPID: os.Getppid()})

	if value := os.Getenv("FIXTURE_SLEEP"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil {
			os.Exit(125)
		}
		time.Sleep(duration)
	}
	if value := os.Getenv("FIXTURE_SIGNAL"); value != "" {
		signal, err := strconv.Atoi(value)
		if err != nil {
			os.Exit(125)
		}
		_ = syscall.Kill(os.Getpid(), syscall.Signal(signal))
		select {}
	}
	if value := os.Getenv("FIXTURE_EXIT"); value != "" {
		code, err := strconv.Atoi(value)
		if err != nil {
			os.Exit(125)
		}
		os.Exit(code)
	}
}
