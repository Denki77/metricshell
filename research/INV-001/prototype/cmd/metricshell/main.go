package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

type event struct {
	Time         string `json:"time"`
	MonoNS       int64  `json:"mono_ns"`
	Event        string `json:"event"`
	PID          int    `json:"pid,omitempty"`
	PPID         int    `json:"ppid,omitempty"`
	Signal       string `json:"signal,omitempty"`
	Target       int    `json:"target,omitempty"`
	ExitCode     int    `json:"exit_code,omitempty"`
	WorkloadPID  int    `json:"workload_pid,omitempty"`
	HTTPAddr     string `json:"http_addr,omitempty"`
	PostExit     string `json:"post_exit,omitempty"`
	Grace        string `json:"grace,omitempty"`
	Subreaper    bool   `json:"subreaper,omitempty"`
	ProcessGroup bool   `json:"process_group,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

var started = time.Now()

func emit(name string, mutate func(*event)) {
	ev := event{
		Time:   time.Now().UTC().Format(time.RFC3339Nano),
		MonoNS: time.Since(started).Nanoseconds(),
		Event:  name,
		PID:    os.Getpid(),
		PPID:   os.Getppid(),
	}
	if mutate != nil {
		mutate(&ev)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		log.Printf(`{"event":"marshal_error","detail":%q}`, err.Error())
		return
	}
	fmt.Println(string(data))
}

func main() {
	var (
		useSubreaper  = flag.Bool("subreaper", false, "mark MetricShell prototype as a child subreaper")
		usePGroup     = flag.Bool("process-group", false, "start workload in its own process group and signal the group")
		postExit      = flag.Duration("post-exit", 0, "bounded post-workload survival duration")
		shutdownGrace = flag.Duration("shutdown-grace", 0, "force-kill workload after this grace period once shutdown starts")
		httpAddr      = flag.String("http", ":9090", "HTTP listen address")
		internalFail  = flag.Bool("internal-fail", false, "simulate MetricShell internal failure before workload start")
	)
	flag.Parse()

	if flag.NArg() == 0 && !*internalFail {
		fmt.Fprintln(os.Stderr, "usage: metricshell [flags] -- workload [args...]")
		os.Exit(64)
	}

	emit("metricshell_start", func(ev *event) {
		ev.Subreaper = *useSubreaper
		ev.ProcessGroup = *usePGroup
		ev.HTTPAddr = *httpAddr
		ev.Grace = shutdownGrace.String()
	})
	writePIDFile()

	if *useSubreaper {
		if err := setSubreaper(); err != nil {
			emit("subreaper_error", func(ev *event) { ev.Detail = err.Error() })
			os.Exit(70)
		}
		emit("subreaper_enabled", nil)
	}

	var workloadExit atomic.Int64
	workloadExit.Store(-1)
	server := startHTTP(*httpAddr, &workloadExit)

	if *internalFail {
		emit("internal_failure", func(ev *event) { ev.ExitCode = 70 })
		shutdownHTTP(server)
		os.Exit(70)
	}

	cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "METRICSHELL_PARENT_PID="+strconv.Itoa(os.Getpid()))
	if *usePGroup {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if err := cmd.Start(); err != nil {
		emit("workload_start_error", func(ev *event) {
			ev.Detail = err.Error()
			ev.ExitCode = 127
		})
		shutdownHTTP(server)
		os.Exit(127)
	}

	workloadPID := cmd.Process.Pid
	emit("workload_started", func(ev *event) { ev.WorkloadPID = workloadPID })

	sigCh := make(chan os.Signal, 16)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(sigCh)

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	var shutdownStarted atomic.Bool
	for {
		select {
		case sig := <-sigCh:
			forwardSignal(sig, workloadPID, *usePGroup)
			if *shutdownGrace > 0 && shutdownStarted.CompareAndSwap(false, true) {
				go forceKillAfter(*shutdownGrace, workloadPID, *usePGroup)
			}
		case err := <-waitCh:
			exitCode := resolveExitCode(err)
			workloadExit.Store(int64(exitCode))
			emit("workload_exited", func(ev *event) {
				ev.WorkloadPID = workloadPID
				ev.ExitCode = exitCode
				if err != nil {
					ev.Detail = err.Error()
				}
			})
			reapDescendantsFor(750 * time.Millisecond)
			if *postExit > 0 {
				emit("post_exit_begin", func(ev *event) { ev.PostExit = postExit.String() })
				time.Sleep(*postExit)
				emit("post_exit_end", nil)
			}
			shutdownHTTP(server)
			emit("metricshell_exit", func(ev *event) { ev.ExitCode = exitCode })
			os.Exit(exitCode)
		}
	}
}

func writePIDFile() {
	if err := os.WriteFile("/tmp/metricshell.pid", []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		emit("pid_file_error", func(ev *event) { ev.Detail = err.Error() })
	}
}

func startHTTP(addr string, workloadExit *atomic.Int64) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "# TYPE metricshell_workload_exit_code gauge\n")
		fmt.Fprintf(w, "metricshell_workload_exit_code %d\n", workloadExit.Load())
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		emit("http_listen", func(ev *event) { ev.HTTPAddr = addr })
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			emit("http_error", func(ev *event) { ev.Detail = err.Error() })
		}
	}()
	return server
}

func shutdownHTTP(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		emit("http_shutdown_error", func(ev *event) { ev.Detail = err.Error() })
		return
	}
	emit("http_shutdown", nil)
}

func forwardSignal(sig os.Signal, workloadPID int, usePGroup bool) {
	sysSig, ok := sig.(syscall.Signal)
	if !ok {
		return
	}
	target := workloadPID
	if usePGroup {
		target = -workloadPID
	}
	emit("signal_received", func(ev *event) {
		ev.Signal = sysSig.String()
		ev.Target = target
	})
	if err := syscall.Kill(target, sysSig); err != nil {
		emit("signal_forward_error", func(ev *event) {
			ev.Signal = sysSig.String()
			ev.Target = target
			ev.Detail = err.Error()
		})
		return
	}
	emit("signal_forwarded", func(ev *event) {
		ev.Signal = sysSig.String()
		ev.Target = target
	})
}

func forceKillAfter(grace time.Duration, workloadPID int, usePGroup bool) {
	time.Sleep(grace)
	target := workloadPID
	if usePGroup {
		target = -workloadPID
	}
	if err := syscall.Kill(target, syscall.SIGKILL); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			emit("force_kill_skipped", func(ev *event) {
				ev.Signal = syscall.SIGKILL.String()
				ev.Target = target
				ev.Grace = grace.String()
				ev.Detail = err.Error()
			})
			return
		}
		emit("force_kill_error", func(ev *event) {
			ev.Signal = syscall.SIGKILL.String()
			ev.Target = target
			ev.Grace = grace.String()
			ev.Detail = err.Error()
		})
		return
	}
	emit("force_kill_sent", func(ev *event) {
		ev.Signal = syscall.SIGKILL.String()
		ev.Target = target
		ev.Grace = grace.String()
	})
}

func resolveExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok {
			return 1
		}
		if status.Signaled() {
			return 128 + int(status.Signal())
		}
		return status.ExitStatus()
	}
	return 1
}

func reapDescendantsFor(duration time.Duration) {
	deadline := time.Now().Add(duration)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		<-ticker.C
		for {
			var status syscall.WaitStatus
			var usage syscall.Rusage
			pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &usage)
			if pid == 0 || errors.Is(err, syscall.ECHILD) {
				break
			}
			if err != nil {
				emit("reap_error", func(ev *event) { ev.Detail = err.Error() })
				break
			}
			emit("descendant_reaped", func(ev *event) {
				ev.WorkloadPID = pid
				ev.ExitCode = waitStatusExitCode(status)
			})
		}
	}
}

func waitStatusExitCode(status syscall.WaitStatus) int {
	if status.Signaled() {
		return 128 + int(status.Signal())
	}
	return status.ExitStatus()
}
