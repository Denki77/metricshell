package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxFrame = 65536

type operation struct {
	Version int               `json:"version"`
	Op      string            `json:"op"`
	Metric  string            `json:"metric"`
	Value   float64           `json:"value"`
	Labels  map[string]string `json:"labels,omitempty"`
}
type reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Epoch string `json:"epoch,omitempty"`
}
type histogram struct {
	Count   uint64   `json:"count"`
	Sum     float64  `json:"sum"`
	Buckets []uint64 `json:"buckets"`
}
type registry struct {
	sync.Mutex
	epoch              string
	counters           map[string]float64
	gauges             map[string]float64
	histograms         map[string]histogram
	accepted, rejected uint64
}

func newRegistry() *registry {
	return &registry{epoch: strconv.FormatInt(time.Now().UnixNano(), 36), counters: map[string]float64{}, gauges: map[string]float64{}, histograms: map[string]histogram{}}
}
func key(metric string, labels map[string]string) string {
	ks := make([]string, 0, len(labels))
	for k := range labels {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var b strings.Builder
	b.WriteString(metric)
	for _, k := range ks {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}
	return b.String()
}
func (r *registry) apply(op operation) reply {
	r.Lock()
	defer r.Unlock()
	reject := func(s string) reply { r.rejected++; return reply{OK: false, Error: s, Epoch: r.epoch} }
	if op.Version != 1 {
		return reject("unsupported_version")
	}
	if op.Metric == "" || len(op.Metric) > 128 {
		return reject("invalid_metric")
	}
	if op.Value != op.Value || op.Value > 1.7976931348623157e308 || op.Value < -1.7976931348623157e308 {
		return reject("invalid_value")
	}
	k := key(op.Metric, op.Labels)
	switch op.Op {
	case "inc", "add":
		if op.Value < 0 {
			return reject("counter_negative")
		}
		r.counters[k] += op.Value
	case "set":
		r.gauges[k] = op.Value
	case "observe":
		if op.Value < 0 {
			return reject("histogram_negative")
		}
		h := r.histograms[k]
		if h.Buckets == nil {
			h.Buckets = make([]uint64, 4)
		}
		h.Count++
		h.Sum += op.Value
		bounds := []float64{0.1, 0.5, 1}
		for i, v := range bounds {
			if op.Value <= v {
				h.Buckets[i]++
			}
		}
		h.Buckets[3]++
		r.histograms[k] = h
	default:
		return reject("unknown_operation")
	}
	r.accepted++
	return reply{OK: true, Epoch: r.epoch}
}
func (r *registry) state() []byte {
	r.Lock()
	defer r.Unlock()
	b, _ := json.Marshal(map[string]interface{}{"epoch": r.epoch, "counters": r.counters, "gauges": r.gauges, "histograms": r.histograms, "accepted": r.accepted, "rejected": r.rejected})
	return b
}
func (r *registry) metrics() []byte {
	r.Lock()
	defer r.Unlock()
	var b strings.Builder
	keys := func(m map[string]float64) []string {
		a := make([]string, 0, len(m))
		for k := range m {
			a = append(a, k)
		}
		sort.Strings(a)
		return a
	}
	for _, k := range keys(r.counters) {
		fmt.Fprintf(&b, "%s %g\n", strings.ReplaceAll(k, "|", "_"), r.counters[k])
	}
	for _, k := range keys(r.gauges) {
		fmt.Fprintf(&b, "%s %g\n", strings.ReplaceAll(k, "|", "_"), r.gauges[k])
	}
	for k, h := range r.histograms {
		fmt.Fprintf(&b, "%s_count %d\n%s_sum %g\n", strings.ReplaceAll(k, "|", "_"), h.Count, strings.ReplaceAll(k, "|", "_"), h.Sum)
	}
	return []byte(b.String())
}
func decodeApply(r *registry, body []byte) reply {
	if len(body) == 0 {
		return reply{OK: false, Error: "empty"}
	}
	if len(body) > maxFrame {
		return reply{OK: false, Error: "frame_too_large"}
	}
	var op operation
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&op); err != nil {
		return reply{OK: false, Error: "malformed"}
	}
	return r.apply(op)
}
func serveConn(c net.Conn, r *registry) {
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	rd := bufio.NewReaderSize(c, maxFrame+2)
	line, err := rd.ReadBytes('\n')
	var out reply
	if errors.Is(err, io.EOF) {
		out = reply{OK: false, Error: "incomplete_frame"}
	} else if err != nil {
		out = reply{OK: false, Error: "incomplete_or_too_large"}
	} else if len(line) > maxFrame+1 {
		out = reply{OK: false, Error: "frame_too_large"}
	} else {
		out = decodeApply(r, bytes.TrimSpace(line))
	}
	_ = json.NewEncoder(c).Encode(out)
}
func server(args []string) {
	f := flag.NewFlagSet("server", flag.ExitOnError)
	unix := f.String("unix", "/run/metricshell/managed.sock", "")
	httpAddr := f.String("http", ":8080", "")
	f.Parse(args)
	r := newRegistry()
	_ = os.Remove(*unix)
	l, err := net.Listen("unix", *unix)
	if err != nil {
		panic(err)
	}
	if err = os.Chmod(*unix, 0660); err != nil {
		panic(err)
	}
	defer l.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/operations", func(w http.ResponseWriter, q *http.Request) {
		if q.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		body, e := io.ReadAll(io.LimitReader(q.Body, maxFrame+1))
		if e != nil {
			w.WriteHeader(400)
			return
		}
		out := decodeApply(r, body)
		w.Header().Set("Content-Type", "application/json")
		if !out.OK {
			w.WriteHeader(422)
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/debug/state", func(w http.ResponseWriter, q *http.Request) { w.Write(r.state()) })
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, q *http.Request) { w.Write(r.metrics()) })
	go func() {
		if err := http.ListenAndServe(*httpAddr, mux); err != nil {
			panic(err)
		}
	}()
	fmt.Printf("READY epoch=%s unix=%s http=%s\n", r.epoch, *unix, *httpAddr)
	for {
		c, e := l.Accept()
		if e != nil {
			panic(e)
		}
		go serveConn(c, r)
	}
}
func sendUnix(path string, payload []byte, newline bool) (reply, error) {
	c, e := net.DialTimeout("unix", path, time.Second)
	if e != nil {
		return reply{}, e
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	wire := payload
	if newline {
		wire = append(wire, '\n')
	}
	if _, e = c.Write(wire); e != nil {
		return reply{}, e
	}
	if !newline {
		if u, ok := c.(*net.UnixConn); ok {
			_ = u.CloseWrite()
		}
	}
	var out reply
	e = json.NewDecoder(c).Decode(&out)
	return out, e
}
func sendHTTPWithClient(cl *http.Client, url string, payload []byte) (reply, error) {
	res, e := cl.Post(strings.TrimRight(url, "/")+"/v1/operations", "application/json", bytes.NewReader(payload))
	if e != nil {
		return reply{}, e
	}
	defer res.Body.Close()
	var out reply
	e = json.NewDecoder(res.Body).Decode(&out)
	return out, e
}

func sendHTTP(url string, payload []byte) (reply, error) {
	cl := &http.Client{Timeout: 2 * time.Second}
	return sendHTTPWithClient(cl, url, payload)
}

func percentile(sorted []int64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(len(sorted)-1)*p + 0.5)
	return float64(sorted[i]) / 1e6
}

func benchmark(args []string) {
	f := flag.NewFlagSet("benchmark", flag.ExitOnError)
	transport := f.String("transport", "unix", "")
	endpoint := f.String("endpoint", "/run/metricshell/managed.sock", "")
	profile := f.String("profile", "connection-per-operation", "")
	clients := f.Int("clients", 1, "")
	operations := f.Int("operations-per-client", 200, "")
	f.Parse(args)
	if *clients < 1 || *operations < 1 {
		fmt.Fprintln(os.Stderr, "clients and operations must be positive")
		os.Exit(2)
	}
	if *transport == "unix" && *profile != "connection-per-operation" {
		fmt.Fprintln(os.Stderr, "unix reused connection is not supported by the candidate protocol")
		os.Exit(2)
	}
	payload, _ := json.Marshal(operation{Version: 1, Op: "inc", Metric: "persistent_benchmark_total", Value: 1, Labels: map[string]string{"transport": *transport, "profile": *profile}})
	latencies := make([]int64, 0, *clients**operations)
	var mu sync.Mutex
	errorsCount := 0
	start := time.Now()
	var wg sync.WaitGroup
	for worker := 0; worker < *clients; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var reused *http.Client
			if *transport == "http" && *profile == "reused-connection" {
				reused = &http.Client{Timeout: 2 * time.Second}
			}
			local := make([]int64, 0, *operations)
			localErrors := 0
			for i := 0; i < *operations; i++ {
				before := time.Now()
				var out reply
				var err error
				if *transport == "unix" {
					out, err = sendUnix(*endpoint, payload, true)
				} else if reused != nil {
					out, err = sendHTTPWithClient(reused, *endpoint, payload)
				} else {
					fresh := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
					out, err = sendHTTPWithClient(fresh, *endpoint, payload)
					fresh.CloseIdleConnections()
				}
				local = append(local, time.Since(before).Nanoseconds())
				if err != nil || !out.OK {
					localErrors++
				}
			}
			if reused != nil {
				reused.CloseIdleConnections()
			}
			mu.Lock()
			latencies = append(latencies, local...)
			errorsCount += localErrors
			mu.Unlock()
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	total := *clients * *operations
	fmt.Printf("%s\t%s\t%d\t%d\t%.3f\t%.1f\t%.3f\t%.3f\t%.3f\t%d\t%d\n", *transport, *profile, *clients, total, float64(elapsed.Nanoseconds())/1e6, float64(total)/elapsed.Seconds(), percentile(latencies, .50), percentile(latencies, .95), percentile(latencies, .99), errorsCount, runtime.NumGoroutine())
}

func badServer(args []string) {
	f := flag.NewFlagSet("badserver", flag.ExitOnError)
	path := f.String("unix", "/run/metricshell/bad.sock", "")
	f.Parse(args)
	_ = os.Remove(*path)
	l, err := net.Listen("unix", *path)
	if err != nil {
		panic(err)
	}
	_ = os.Chmod(*path, 0666)
	defer l.Close()
	fmt.Println("READY")
	c, err := l.Accept()
	if err != nil {
		panic(err)
	}
	defer c.Close()
	_, _ = bufio.NewReader(c).ReadBytes('\n')
	_, _ = c.Write([]byte("not-json\n"))
}
func client(args []string) {
	f := flag.NewFlagSet("client", flag.ExitOnError)
	transport := f.String("transport", "unix", "")
	endpoint := f.String("endpoint", "/run/metricshell/managed.sock", "")
	op := f.String("op", "inc", "")
	metric := f.String("metric", "jobs_total", "")
	value := f.Float64("value", 1, "")
	labels := f.String("labels", "", "")
	raw := f.String("raw", "", "")
	size := f.Int("raw-size", 0, "")
	padSize := f.Int("pad-size", 0, "")
	noNewline := f.Bool("no-newline", false, "")
	retries := f.Int("retries", 0, "")
	delay := f.Duration("retry-delay", 20*time.Millisecond, "")
	f.Parse(args)
	lm := map[string]string{}
	if *labels != "" {
		for _, p := range strings.Split(*labels, ",") {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 {
				fmt.Fprintln(os.Stderr, "invalid labels")
				os.Exit(2)
			}
			lm[kv[0]] = kv[1]
		}
	}
	payload, _ := json.Marshal(operation{Version: 1, Op: *op, Metric: *metric, Value: *value, Labels: lm})
	if *raw != "" {
		payload = []byte(*raw)
	}
	if *size > 0 {
		payload = []byte(strings.Repeat("x", *size))
	}
	if *padSize > len(payload) {
		payload = append(payload, []byte(strings.Repeat(" ", *padSize-len(payload)))...)
	}
	var out reply
	var err error
	for i := 0; i <= *retries; i++ {
		if *transport == "unix" {
			out, err = sendUnix(*endpoint, payload, !*noNewline)
		} else {
			out, err = sendHTTP(*endpoint, payload)
		}
		if err == nil {
			break
		}
		if i < *retries {
			time.Sleep(*delay)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	json.NewEncoder(os.Stdout).Encode(out)
	if !out.OK {
		os.Exit(4)
	}
}
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "server|client")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "server":
		server(os.Args[2:])
	case "client":
		client(os.Args[2:])
	case "benchmark":
		benchmark(os.Args[2:])
	case "badserver":
		badServer(os.Args[2:])
	default:
		os.Exit(2)
	}
}
