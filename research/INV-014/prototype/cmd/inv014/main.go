package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type snapshot struct {
	body   string
	series int
}

var metricName = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
var labelName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func main() {
	mode := flag.String("mode", "server", "server or allocate")
	addr := flag.String("addr", ":19114", "listen address")
	maxPayload := flag.Int64("max-payload", 4096, "candidate payload bytes")
	maxSeries := flag.Int("max-series", 1000, "candidate series")
	maxLabels := flag.Int("max-labels", 8, "labels per series")
	maxLabelBytes := flag.Int("max-label-bytes", 64, "label name/value bytes")
	maxConcurrent := flag.Int("max-concurrent", 4, "concurrent ingestion requests")
	allocate := flag.Int("allocate-bytes", 0, "bytes to touch in allocate mode")
	flag.Parse()
	if *mode == "allocate" {
		b := make([]byte, *allocate)
		for i := 0; i < len(b); i += 4096 {
			b[i] = 1
		}
		fmt.Printf("allocated=%d checksum=%d\n", len(b), b[len(b)-4096])
		return
	}
	initial := &snapshot{body: "", series: 0}
	var active atomic.Pointer[snapshot]
	active.Store(initial)
	var rejected atomic.Uint64
	sem := make(chan struct{}, *maxConcurrent)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") })
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		s := active.Load()
		_, _ = io.WriteString(w, s.body)
		fmt.Fprintf(w, "# TYPE metricshell_active_application_series gauge\nmetricshell_active_application_series %d\n# TYPE metricshell_snapshot_rejections_total counter\nmetricshell_snapshot_rejections_total %d\n", s.series, rejected.Load())
	})
	mux.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			http.Error(w, "busy", http.StatusTooManyRequests)
			return
		}
		if hold, _ := strconv.Atoi(r.Header.Get("X-Research-Hold-Millis")); hold > 0 && hold <= 1000 {
			time.Sleep(time.Duration(hold) * time.Millisecond)
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, *maxPayload+1))
		if err != nil || len(body) == 0 {
			rejected.Add(1)
			http.Error(w, "malformed payload", http.StatusBadRequest)
			return
		}
		if int64(len(body)) > *maxPayload {
			rejected.Add(1)
			http.Error(w, "payload limit", http.StatusRequestEntityTooLarge)
			return
		}
		candidate, status, err := validate(body, *maxSeries, *maxLabels, *maxLabelBytes)
		if err != nil {
			rejected.Add(1)
			http.Error(w, err.Error(), status)
			return
		}
		active.Store(candidate)
		w.WriteHeader(http.StatusNoContent)
	})
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(70)
	}
	fmt.Printf("ready uid=%d gid=%d addr=%s\n", os.Getuid(), os.Getgid(), *addr)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 500 * time.Millisecond, ReadTimeout: 1200 * time.Millisecond, WriteTimeout: 2 * time.Second, IdleTimeout: 2 * time.Second, MaxHeaderBytes: 4096}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func validate(body []byte, maxSeries, maxLabels, maxLabelBytes int) (*snapshot, int, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	seen := map[string]struct{}{}
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, http.StatusBadRequest, fmt.Errorf("malformed series")
		}
		identity := parts[0]
		name := identity
		labels := ""
		if i := strings.IndexByte(identity, '{'); i >= 0 {
			if !strings.HasSuffix(identity, "}") {
				return nil, http.StatusBadRequest, fmt.Errorf("malformed labels")
			}
			name, labels = identity[:i], identity[i+1:len(identity)-1]
		}
		if !metricName.MatchString(name) {
			return nil, http.StatusBadRequest, fmt.Errorf("invalid metric name")
		}
		if strings.Contains(strings.ToLower(identity), "secret") || strings.Contains(strings.ToLower(identity), "token") || strings.Contains(strings.ToLower(identity), "password") {
			return nil, http.StatusBadRequest, fmt.Errorf("secret-like label rejected")
		}
		if labels != "" {
			items := strings.Split(labels, ",")
			if len(items) > maxLabels {
				return nil, http.StatusUnprocessableEntity, fmt.Errorf("label limit")
			}
			for _, item := range items {
				kv := strings.SplitN(item, "=", 2)
				if len(kv) != 2 || !labelName.MatchString(kv[0]) || len(kv[0]) > maxLabelBytes {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid label")
				}
				value := strings.Trim(kv[1], `"`)
				if len(value) > maxLabelBytes {
					return nil, http.StatusUnprocessableEntity, fmt.Errorf("label value limit")
				}
			}
		}
		if _, ok := seen[identity]; ok {
			return nil, http.StatusBadRequest, fmt.Errorf("duplicate series")
		}
		seen[identity] = struct{}{}
		count++
		if count > maxSeries {
			return nil, http.StatusUnprocessableEntity, fmt.Errorf("series limit")
		}
		if _, err := strconv.ParseFloat(parts[1], 64); err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("invalid value")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, http.StatusBadRequest, err
	}
	if count == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("no series")
	}
	canonical := strings.TrimSpace(string(body)) + "\n"
	return &snapshot{body: canonical, series: count}, http.StatusNoContent, nil
}
