package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type event struct {
	Time   string `json:"time"`
	Event  string `json:"event"`
	PID    int    `json:"pid"`
	PPID   int    `json:"ppid"`
	Signal string `json:"signal,omitempty"`
}

func emit(name string, sig os.Signal) {
	ev := event{
		Time:  time.Now().UTC().Format(time.RFC3339Nano),
		Event: name,
		PID:   os.Getpid(),
		PPID:  os.Getppid(),
	}
	if sig != nil {
		ev.Signal = sig.String()
	}
	data, _ := json.Marshal(ev)
	fmt.Println(string(data))
}

func main() {
	emit("workload_start", nil)
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	emit("workload_ready", nil)
	for sig := range sigCh {
		switch sig {
		case syscall.SIGTERM:
			emit("workload_term", sig)
			emit("workload_exit_after_term", nil)
			os.Exit(0)
		case syscall.SIGINT:
			emit("workload_int", sig)
			os.Exit(130)
		case syscall.SIGHUP:
			emit("workload_hup", sig)
		}
	}
}
