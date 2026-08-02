package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type app struct {
	body string
	hash string
}

type service struct {
	application     app
	mode            string
	required        int64
	eligibleToken   string
	paddingBytes    int
	chunkDelay      time.Duration
	completionGrace time.Duration
	completed       atomic.Int64
	attempted       atomic.Int64
	self            atomic.Int64
	done            chan struct{}
	once            sync.Once
}

func main() {
	mode := flag.String("mode", "scrapes", "immediate, duration, or scrapes")
	required := flag.Int64("required", 1, "eligible completed scrapes required")
	wait := flag.Duration("wait", 3*time.Second, "duration or scrape timeout")
	addr := flag.String("addr", ":19111", "listen address")
	eligibleToken := flag.String("eligible-token", "", "optional X-Final-Scrape-Token value")
	paddingBytes := flag.Int("padding-bytes", 0, "response padding for disconnect tests")
	chunkDelay := flag.Duration("chunk-delay", 0, "delay between response chunks")
	completionGrace := flag.Duration("completion-grace", 100*time.Millisecond, "grace period for in-flight responses after scrape threshold")
	flag.Parse()
	if *required < 1 {
		fmt.Fprintln(os.Stderr, "required must be positive")
		os.Exit(64)
	}
	body := "# HELP application_jobs_total Frozen final application counter.\n# TYPE application_jobs_total counter\napplication_jobs_total 42\n"
	h := sha256.Sum256([]byte(body))
	s := &service{application: app{body: body, hash: hex.EncodeToString(h[:])}, mode: *mode, required: *required, eligibleToken: *eligibleToken, paddingBytes: *paddingBytes, chunkDelay: *chunkDelay, completionGrace: *completionGrace, done: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ready\n") })
	mux.HandleFunc("/state", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "completed=%d attempted=%d\n", s.completed.Load(), s.attempted.Load())
	})
	mux.HandleFunc("/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "application snapshot is frozen", http.StatusConflict)
	})
	mux.HandleFunc("/metrics", s.metrics)
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second, WriteTimeout: 30 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	fmt.Printf("ready pid=%d mode=%s required=%d application_sha256=%s\n", os.Getpid(), *mode, *required, s.application.hash)
	switch *mode {
	case "immediate":
		s.closeDone()
	case "duration":
		time.AfterFunc(*wait, s.closeDone)
	case "scrapes":
		time.AfterFunc(*wait, s.closeDone)
	default:
		fmt.Fprintln(os.Stderr, "unknown mode")
		os.Exit(64)
	}
	started := time.Now()
	<-s.done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	reason := "required_scrapes"
	if *mode == "immediate" {
		reason = "immediate"
	}
	if *mode == "duration" {
		reason = "duration_elapsed"
	}
	if *mode == "scrapes" && s.completed.Load() < s.required {
		reason = "timeout"
	}
	fmt.Printf("final reason=%s final_scrapes=%d attempted_scrapes=%d self=%d application_sha256=%s wait_ms=%.3f\n", reason, s.completed.Load(), s.attempted.Load(), s.self.Load(), s.application.hash, float64(time.Since(started).Microseconds())/1000)
}

func (s *service) closeDone() { s.once.Do(func() { close(s.done) }) }

func (s *service) metrics(w http.ResponseWriter, r *http.Request) {
	s.attempted.Add(1)
	self := s.self.Add(1)
	body := s.application.body + "# HELP metricshell_final_scrapes_completed Completed eligible final responses.\n# TYPE metricshell_final_scrapes_completed gauge\nmetricshell_final_scrapes_completed " + strconv.FormatInt(s.completed.Load(), 10) + "\n# HELP metricshell_scrape_attempts_total Final response attempts.\n# TYPE metricshell_scrape_attempts_total counter\nmetricshell_scrape_attempts_total " + strconv.FormatInt(self, 10) + "\n"
	if s.paddingBytes > 0 {
		prefix := "# HELP metricshell_padding Synthetic response padding.\n# TYPE metricshell_padding gauge\n"
		line := "metricshell_padding{id=\"" + strings.Repeat("x", 240) + "\"} 1\n"
		var padded strings.Builder
		padded.Grow(s.paddingBytes + len(line))
		padded.WriteString(body)
		padded.WriteString(prefix)
		for padded.Len() < s.paddingBytes {
			padded.WriteString(line)
		}
		body = padded.String()
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("X-Application-Snapshot-SHA256", s.application.hash)
	complete := true
	for offset := 0; offset < len(body); offset += 16 << 10 {
		end := offset + (16 << 10)
		if end > len(body) {
			end = len(body)
		}
		if _, err := io.WriteString(w, body[offset:end]); err != nil {
			complete = false
			break
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		if s.chunkDelay > 0 {
			time.Sleep(s.chunkDelay)
		}
	}
	if r.Context().Err() != nil {
		complete = false
	}
	eligible := s.eligibleToken == "" || r.Header.Get("X-Final-Scrape-Token") == s.eligibleToken
	if complete && eligible {
		for {
			old := s.completed.Load()
			if old >= s.required {
				break
			}
			if s.completed.CompareAndSwap(old, old+1) {
				if old+1 >= s.required && s.mode == "scrapes" {
					time.AfterFunc(s.completionGrace, s.closeDone)
				}
				break
			}
		}
	}
}
