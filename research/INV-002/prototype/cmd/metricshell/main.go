package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var value atomic.Int64
var attempts atomic.Int64
var lastExit atomic.Int64

func main() {
	policy := flag.String("policy", "single", "single or internal-restart")
	metricState := flag.String("metric-state", "reset", "reset or preserve")
	maxAttempts := flag.Int("max-attempts", 3, "maximum internal attempts")
	postExit := flag.Duration("post-exit", 0, "bounded metrics availability after final exit")
	listen := flag.String("http", ":9090", "listen address")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "workload required after --")
		os.Exit(64)
	}
	lastExit.Store(-1)
	srv := serve(*listen)
	final := 0
	for n := 1; ; n++ {
		if n > 1 && *metricState == "reset" {
			value.Store(0)
		}
		attempts.Store(int64(n))
		fmt.Printf("EVENT attempt_started attempt=%d value=%d\n", n, value.Load())
		code, err := run(flag.Args())
		if err != nil {
			fmt.Printf("EVENT start_failed error=%q\n", err)
			fmt.Printf("EVENT lifecycle_finalized attempts=0 exit=127 value=%d\n", value.Load())
			shutdown(srv)
			os.Exit(127)
		}
		lastExit.Store(int64(code))
		final = code
		fmt.Printf("EVENT attempt_exited attempt=%d exit=%d value=%d\n", n, code, value.Load())
		if code == 0 || *policy == "single" || n >= *maxAttempts {
			break
		}
	}
	fmt.Printf("EVENT lifecycle_finalized attempts=%d exit=%d value=%d\n", attempts.Load(), final, value.Load())
	postExitStarted := time.Now()
	fmt.Printf("EVENT post_exit_begin duration=%s\n", postExit.String())
	time.Sleep(*postExit)
	fmt.Printf("EVENT post_exit_end configured_ms=%d elapsed_ms=%.3f\n", postExit.Milliseconds(), float64(time.Since(postExitStarted).Microseconds())/1000)
	shutdown(srv)
	os.Exit(final)
}

func run(args []string) (int, error) {
	cmd := exec.Command(args[0], args[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return 0, err
	}
	done := make(chan struct{})
	go func() {
		s := bufio.NewScanner(stdout)
		for s.Scan() {
			line := s.Text()
			fmt.Println(line)
			if strings.HasPrefix(line, "METRIC_INCREMENT ") {
				n, _ := strconv.ParseInt(strings.TrimPrefix(line, "METRIC_INCREMENT "), 10, 64)
				value.Add(n)
			}
		}
		close(done)
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	var signalForwardedNS atomic.Int64
	signalDone := make(chan struct{})
	go func() {
		select {
		case sig := <-sigs:
			signalForwardedNS.Store(time.Now().UnixNano())
			fmt.Printf("EVENT signal_forwarded signal=%s\n", sig)
			_ = cmd.Process.Signal(sig)
		case <-signalDone:
		}
	}()
	err = cmd.Wait()
	signal.Stop(sigs)
	close(signalDone)
	<-done
	if forwardedNS := signalForwardedNS.Load(); forwardedNS > 0 {
		fmt.Printf("EVENT signal_to_exit elapsed_ms=%.3f\n", float64(time.Now().UnixNano()-forwardedNS)/1e6)
	}
	if err == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), nil
		}
		return ee.ExitCode(), nil
	}
	return 0, err
}

func serve(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "# TYPE app_events_total counter\napp_events_total %d\n", value.Load())
		fmt.Fprintf(w, "# TYPE metricshell_workload_attempt gauge\nmetricshell_workload_attempt %d\n", attempts.Load())
		fmt.Fprintf(w, "# TYPE metricshell_workload_exit_code gauge\nmetricshell_workload_exit_code %d\n", lastExit.Load())
	})
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	return srv
}

func shutdown(s *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.Shutdown(ctx)
}
