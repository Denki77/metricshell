package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

type event struct {
	Time  string `json:"time"`
	Event string `json:"event"`
	PID   int    `json:"pid"`
	PPID  int    `json:"ppid"`
}

func emit(name string) {
	data, _ := json.Marshal(event{
		Time:  time.Now().UTC().Format(time.RFC3339Nano),
		Event: name,
		PID:   os.Getpid(),
		PPID:  os.Getppid(),
	})
	fmt.Println(string(data))
}

func main() {
	duration := 3 * time.Second
	if len(os.Args) > 1 {
		seconds, err := strconv.ParseFloat(os.Args[1], 64)
		if err == nil && seconds > 0 {
			duration = time.Duration(seconds * float64(time.Second))
		}
	}

	emit("cpu_workload_start")
	deadline := time.Now().Add(duration)
	var x uint64
	for time.Now().Before(deadline) {
		x = x*1664525 + 1013904223
	}
	if x == 0 {
		fmt.Fprintln(os.Stderr, "unreachable")
	}
	emit("cpu_workload_exit")
}
