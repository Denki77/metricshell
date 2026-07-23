package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	termDelay := flag.Duration("term-delay", 0, "delay after TERM; negative means ignore TERM")
	exitCode := flag.Int("exit-code", 0, "normal exit code after TERM")
	flag.Parse()
	fmt.Printf("WORKLOAD_READY pid=%d term_delay_ms=%d\n", os.Getpid(), termDelay.Milliseconds())
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	for sig := range sigs {
		fmt.Printf("WORKLOAD_SIGNAL signal=%s\n", sig)
		if *termDelay < 0 {
			continue
		}
		time.Sleep(*termDelay)
		fmt.Printf("WORKLOAD_EXIT code=%d\n", *exitCode)
		os.Exit(*exitCode)
	}
}
