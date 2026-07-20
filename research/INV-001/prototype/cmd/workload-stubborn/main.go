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
	emit("stubborn_workload_start", nil)
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	emit("stubborn_workload_ready", nil)
	for sig := range sigCh {
		emit("stubborn_signal_ignored", sig)
	}
}
