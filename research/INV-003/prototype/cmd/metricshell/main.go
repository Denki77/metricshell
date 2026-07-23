package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type eventLog struct {
	start time.Time
	mu    sync.Mutex
}

func (l *eventLog) event(name string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var line strings.Builder
	fmt.Fprintf(&line, "EVENT name=%s elapsed_ms=%.3f", name, float64(time.Since(l.start).Microseconds())/1000)
	for i := 0; i+1 < len(fields); i += 2 {
		fmt.Fprintf(&line, " %v=%v", fields[i], fields[i+1])
	}
	fmt.Println(line.String())
}

func main() {
	total := flag.Duration("total-grace", 10*time.Second, "external shutdown grace known by MetricShell")
	policy := flag.String("policy", "explicit", "explicit, fixed, percentage, or deadline")
	workloadTimeout := flag.Duration("workload-timeout", 8*time.Second, "explicit workload timeout")
	reserve := flag.Duration("reserve", 2*time.Second, "explicit/fixed MetricShell reserve")
	ratio := flag.Float64("workload-ratio", .8, "percentage policy workload share")
	deadlineUnixMS := flag.Int64("shutdown-deadline-unix-ms", 0, "absolute wall-clock shutdown deadline in Unix milliseconds")
	deadlineFile := flag.String("shutdown-deadline-file", "", "file containing an absolute Unix-millisecond deadline, read when termination begins")
	finalizeDelay := flag.Duration("finalize-delay", 20*time.Millisecond, "synthetic metric/diagnostic finalization")
	httpTimeout := flag.Duration("http-timeout", 500*time.Millisecond, "maximum HTTP drain time")
	postExit := flag.Duration("post-exit", 0, "post-workload scrape wait on natural completion only")
	listen := flag.String("http", ":9090", "HTTP listen address")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "workload required after --")
		os.Exit(64)
	}
	if *total <= 0 || *reserve < 0 || *workloadTimeout < 0 || *ratio < 0 || *ratio > 1 {
		fmt.Fprintln(os.Stderr, "invalid budget")
		os.Exit(64)
	}
	if *policy == "deadline" && *deadlineUnixMS == 0 && *deadlineFile == "" {
		fmt.Fprintln(os.Stderr, "deadline policy requires --shutdown-deadline-unix-ms or --shutdown-deadline-file")
		os.Exit(64)
	}

	log := &eventLog{start: time.Now()}
	var shutting atomic.Bool
	srv, _ := serve(*listen, &shutting, log)
	cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		log.event("start_failed", "error", strconv.Quote(err.Error()))
		os.Exit(127)
	}
	log.event("workload_started", "pid", cmd.Process.Pid)
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-waitCh:
		code := exitCode(err)
		log.event("workload_exited", "exit", code, "forced", false)
		if *postExit > 0 {
			log.event("post_exit_begin", "configured_ms", postExit.Milliseconds())
			time.Sleep(*postExit)
			log.event("post_exit_end")
		}
		shutdownHTTP(srv, *httpTimeout, log)
		os.Exit(code)
	case sig := <-sigs:
		shutdownStart := time.Now()
		shutting.Store(true)
		effectiveTotal := *total
		resolvedDeadlineUnixMS := *deadlineUnixMS
		if *policy == "deadline" {
			if *deadlineFile != "" {
				raw, err := os.ReadFile(*deadlineFile)
				if err != nil {
					log.event("deadline_read_failed", "error", strconv.Quote(err.Error()))
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
					os.Exit(64)
				}
				resolvedDeadlineUnixMS, err = strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
				if err != nil {
					log.event("deadline_parse_failed")
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
					os.Exit(64)
				}
			}
			effectiveTotal = time.Until(time.UnixMilli(resolvedDeadlineUnixMS))
			if effectiveTotal < 0 {
				effectiveTotal = 0
			}
		}
		workBudget := calculateBudget(*policy, effectiveTotal, *workloadTimeout, *reserve, *ratio)
		if workBudget < 0 {
			log.event("budget_rejected", "policy", *policy)
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			os.Exit(64)
		}
		if workBudget > effectiveTotal {
			workBudget = effectiveTotal
		}
		actualReserve := effectiveTotal - workBudget
		log.event("shutdown_started", "signal", sig, "policy", *policy, "configured_total_ms", total.Milliseconds(), "remaining_total_ms", effectiveTotal.Milliseconds(), "workload_budget_ms", workBudget.Milliseconds(), "reserve_ms", actualReserve.Milliseconds(), "deadline_unix_ms", resolvedDeadlineUnixMS)
		httpBudget := *httpTimeout
		if httpBudget > effectiveTotal {
			httpBudget = effectiveTotal
		}
		httpDone := make(chan struct{})
		go func() { shutdownHTTP(srv, httpBudget, log); close(httpDone) }()
		_ = syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal))
		log.event("signal_forwarded")
		forced := false
		var err error
		select {
		case err = <-waitCh:
		case <-time.After(workBudget):
			forced = true
			log.event("workload_budget_expired")
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			err = <-waitCh
		}
		code := exitCode(err)
		log.event("workload_exited", "exit", code, "forced", forced, "shutdown_elapsed_ms", fmt.Sprintf("%.3f", float64(time.Since(shutdownStart).Microseconds())/1000))
		remaining := effectiveTotal - time.Since(shutdownStart)
		if remaining < 0 {
			remaining = 0
		}
		finalize := *finalizeDelay
		if finalize > remaining {
			finalize = remaining
		}
		time.Sleep(finalize)
		log.event("finalization_complete", "requested_ms", finalizeDelay.Milliseconds(), "spent_ms", finalize.Milliseconds())
		remaining = effectiveTotal - time.Since(shutdownStart)
		if remaining < 0 {
			remaining = 0
		}
		select {
		case <-httpDone:
		case <-time.After(remaining):
		}
		log.event("shutdown_complete", "total_elapsed_ms", fmt.Sprintf("%.3f", float64(time.Since(shutdownStart).Microseconds())/1000))
		os.Exit(code)
	}
}

func calculateBudget(policy string, total, explicit, reserve time.Duration, ratio float64) time.Duration {
	switch policy {
	case "explicit":
		if explicit+reserve > total {
			return -1
		}
		return explicit
	case "fixed", "deadline":
		if reserve > total {
			if policy == "deadline" {
				return 0
			}
			return -1
		}
		return total - reserve
	case "percentage":
		return time.Duration(float64(total) * ratio)
	default:
		return -1
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return ee.ExitCode()
	}
	return 70
}

func serve(addr string, shutting *atomic.Bool, log *eventLog) (*http.Server, net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if shutting.Load() {
			log.event("scrape_admission_rejected")
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}
		delay, _ := time.ParseDuration(r.URL.Query().Get("delay"))
		log.event("scrape_begin", "delay_ms", delay.Milliseconds())
		time.Sleep(delay)
		fmt.Fprintf(w, "metricshell_shutting_down %d\n", map[bool]int{false: 0, true: 1}[shutting.Load()])
		log.event("scrape_end")
	})
	s := &http.Server{Addr: addr, Handler: mux}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(70)
	}
	go func() { _ = s.Serve(ln) }()
	return s, ln
}
func shutdownHTTP(s *http.Server, timeout time.Duration, log *eventLog) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := s.Shutdown(ctx)
	result := "drained"
	if err != nil {
		result = "timeout"
	}
	log.event("http_shutdown", "budget_ms", timeout.Milliseconds(), "elapsed_ms", fmt.Sprintf("%.3f", float64(time.Since(started).Microseconds())/1000), "result", result)
}
