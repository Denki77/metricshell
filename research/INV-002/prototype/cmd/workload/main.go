package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func pick(csv string, n int) int {
	parts := strings.Split(csv, ",")
	if n >= len(parts) {
		n = len(parts) - 1
	}
	v, err := strconv.Atoi(parts[n])
	if err != nil {
		panic(err)
	}
	return v
}

func main() {
	state := flag.String("state", "/tmp/attempt", "persistent attempt file")
	exits := flag.String("exits", "0", "comma-separated exit codes")
	increments := flag.String("increments", "1", "comma-separated counter increments")
	hold := flag.Duration("hold", 400*time.Millisecond, "time for observation")
	allocateMB := flag.Int("allocate-mb", 0, "touch this many MiB before exit")
	flag.Parse()
	attempt := 0
	if b, err := os.ReadFile(*state); err == nil {
		attempt, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	_ = os.WriteFile(*state, []byte(strconv.Itoa(attempt+1)), 0644)
	inc := pick(*increments, attempt)
	code := pick(*exits, attempt)
	fmt.Printf("WORKLOAD_READY attempt=%d increment=%d exit=%d\n", attempt+1, inc, code)
	fmt.Printf("METRIC_INCREMENT %d\n", inc)
	if *allocateMB > 0 {
		memory := make([]byte, *allocateMB*1024*1024)
		for i := 0; i < len(memory); i += 4096 {
			memory[i] = 1
		}
		fmt.Printf("MEMORY_ALLOCATED mb=%d\n", *allocateMB)
	}
	time.Sleep(*hold)
	fmt.Printf("WORKLOAD_EXIT attempt=%d exit=%d\n", attempt+1, code)
	os.Exit(code)
}
