package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type snapshot struct {
	prom       []byte
	openMetric []byte
	generation string
	series     int
}

type server struct {
	active        atomic.Pointer[snapshot]
	scrapes       atomic.Uint64
	rejected      atomic.Uint64
	maxSnapshot   int64
	responseLimit int64
}

func main() {
	addr := flag.String("addr", ":19100", "listen address")
	series := flag.Int("series", 1000, "initial gauge series")
	responseLimit := flag.Int64("response-limit", 32<<20, "maximum uncompressed response bytes")
	maxSnapshot := flag.Int64("snapshot-limit", 64<<20, "maximum candidate snapshot bytes")
	flag.Parse()

	s := &server{maxSnapshot: *maxSnapshot, responseLimit: *responseLimit}
	initial, err := parseSnapshot([]byte(generateSnapshot("initial", *series)))
	if err != nil {
		panic(err)
	}
	s.active.Store(initial)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") })
	mux.HandleFunc("/snapshot", s.install)
	mux.HandleFunc("/metrics", s.metrics)
	mux.HandleFunc("/generate", s.generate)
	h := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second}
	fmt.Printf("ready addr=%s series=%d\n", *addr, *series)
	if err := h.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func (s *server) install(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, s.maxSnapshot+1))
	if err != nil || int64(len(body)) > s.maxSnapshot || len(body) == 0 {
		s.rejected.Add(1)
		http.Error(w, "invalid snapshot", http.StatusRequestEntityTooLarge)
		return
	}
	candidate, err := parseSnapshot(body)
	if err != nil {
		s.rejected.Add(1)
		http.Error(w, "invalid snapshot: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.active.Store(candidate)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) generate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	n, err := strconv.Atoi(r.URL.Query().Get("series"))
	if err != nil || n < 0 || n > 1_000_000 {
		http.Error(w, "bad series", http.StatusBadRequest)
		return
	}
	generation := r.URL.Query().Get("generation")
	if generation == "" {
		generation = "generated"
	}
	candidate, err := parseSnapshot([]byte(generateSnapshot(generation, n)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.active.Store(candidate)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) metrics(w http.ResponseWriter, r *http.Request) {
	current := s.active.Load() // one immutable complete application snapshot per scrape
	om := strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/openmetrics-text")
	body := current.prom
	contentType := "text/plain; version=0.0.4; charset=utf-8"
	if om {
		body = current.openMetric
		contentType = "application/openmetrics-text; version=1.0.0; charset=utf-8"
	}
	self := fmt.Sprintf("# HELP metricshell_exposition_scrapes_total Completed exposition attempts.\n# TYPE metricshell_exposition_scrapes_total counter\nmetricshell_exposition_scrapes_total %d\n# HELP metricshell_snapshot_rejections_total Rejected complete candidate snapshots.\n# TYPE metricshell_snapshot_rejections_total counter\nmetricshell_snapshot_rejections_total %d\n", s.scrapes.Load(), s.rejected.Load())
	if om {
		body = append(append([]byte{}, body...), []byte(self+"# EOF\n")...)
	} else {
		body = append(append([]byte{}, body...), []byte(self)...)
	}
	if int64(len(body)) > s.responseLimit {
		http.Error(w, "response limit", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Application-Snapshot-Generation", current.generation)
	w.Header().Set("X-Application-Series", strconv.Itoa(current.series))
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, err := gz.Write(body)
		closeErr := gz.Close()
		if err == nil && closeErr == nil {
			s.scrapes.Add(1)
		}
		return
	}
	if _, err := w.Write(body); err == nil {
		s.scrapes.Add(1)
	}
}

func parseSnapshot(input []byte) (*snapshot, error) {
	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	if len(families) == 0 {
		return nil, fmt.Errorf("zero-family snapshot must use an explicit product encoding")
	}
	names := make([]string, 0, len(families))
	for name := range families {
		names = append(names, name)
	}
	sort.Strings(names)
	var prom bytes.Buffer
	series := 0
	generation := "unknown"
	for _, name := range names {
		mf := families[name]
		series += len(mf.Metric)
		if _, err := expfmt.MetricFamilyToText(&prom, mf); err != nil {
			return nil, err
		}
		if name == "inv010_generation" && len(mf.Metric) > 0 {
			for _, lp := range mf.Metric[0].Label {
				if lp.GetName() == "id" {
					generation = lp.GetValue()
				}
			}
		}
	}
	// The parser and canonical encoder are the research subject. OpenMetrics 1.0
	// accepts this canonical metric-family representation plus the EOF marker.
	return &snapshot{prom: prom.Bytes(), openMetric: append([]byte{}, prom.Bytes()...), generation: generation, series: series}, nil
}

func generateSnapshot(generation string, series int) string {
	var b strings.Builder
	b.WriteString("# HELP inv010_generation Complete snapshot generation.\n# TYPE inv010_generation gauge\n")
	fmt.Fprintf(&b, "inv010_generation{id=%q} 1\n", generation)
	b.WriteString("# HELP inv010_series Synthetic application gauge.\n# TYPE inv010_series gauge\n")
	for i := 0; i < series; i++ {
		fmt.Fprintf(&b, "inv010_series{generation=%q,id=%q} %d\n", generation, strconv.Itoa(i), i)
	}
	b.WriteString("# HELP inv010_requests_total Synthetic counter.\n# TYPE inv010_requests_total counter\ninv010_requests_total 17\n")
	b.WriteString("# HELP inv010_duration_seconds Synthetic classic histogram.\n# TYPE inv010_duration_seconds histogram\ninv010_duration_seconds_bucket{le=\"0.1\"} 2\ninv010_duration_seconds_bucket{le=\"1\"} 4\ninv010_duration_seconds_bucket{le=\"+Inf\"} 5\ninv010_duration_seconds_sum 1.7\ninv010_duration_seconds_count 5\n")
	return b.String()
}

var _ = dto.MetricFamily{}
