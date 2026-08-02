package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	wait := flag.Duration("wait", 30*time.Second, "post-workload wait timeout")
	required := flag.Int64("required-scrapes", 0, "exit after this many completed scrapes; zero waits for timeout")
	ready := flag.Bool("ready", true, "readiness during post-workload wait")
	addr := flag.String("addr", ":19112", "listen address")
	flag.Parse()
	var scrapes atomic.Int64
	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !*ready {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ready")
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		body := fmt.Sprintf("# TYPE inv012_final_snapshot gauge\ninv012_final_snapshot 42\n# TYPE inv012_final_scrapes_total counter\ninv012_final_scrapes_total %d\n", scrapes.Load())
		if _, err := fmt.Fprint(w, body); err == nil && r.Context().Err() == nil {
			n := scrapes.Add(1)
			fmt.Printf("scrape_completed count=%d user_agent=%q\n", n, r.UserAgent())
			if *required > 0 && n >= *required {
				closeDone()
			}
		}
	})
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second, WriteTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	started := time.Now()
	fmt.Printf("workload_completed pid=%d ready=%t required_scrapes=%d\n", os.Getpid(), *ready, *required)
	timer := time.AfterFunc(*wait, closeDone)
	defer timer.Stop()
	<-done
	reason := "required_scrapes"
	if *required == 0 || scrapes.Load() < *required {
		reason = "timeout"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Printf("final reason=%s final_scrapes=%d wait_ms=%.3f\n", reason, scrapes.Load(), float64(time.Since(started).Microseconds())/1000)
}
