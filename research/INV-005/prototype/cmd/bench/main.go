package main

import (
	"context"
	"encoding/binary"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

const (
	publish  = "publish-only"
	observed = "consumer-observed"
	acked    = "acknowledged"
)

type sample struct {
	transport, profile, scenario string
	producer, seq, bytes         int
	latency                      time.Duration
	ok                           bool
}
type scenarioResult struct {
	transport, profile, scenario string
	samples                      []sample
	elapsed                      time.Duration
}
type endpoint struct {
	send  func([]byte, uint64, string) error
	close func()
}
type tracker struct {
	mu      sync.Mutex
	waiters map[uint64]chan struct{}
}

func newTracker() *tracker { return &tracker{waiters: map[uint64]chan struct{}{}} }
func (t *tracker) register(seq uint64) chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	ch := make(chan struct{})
	t.waiters[seq] = ch
	return ch
}
func (t *tracker) mark(seq uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ch, ok := t.waiters[seq]; ok {
		close(ch)
		delete(t.waiters, seq)
	}
}
func waitObserved(ch chan struct{}) error {
	select {
	case <-ch:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("consumer observation timeout")
	}
}

func frame(seq uint64, payload []byte) []byte {
	b := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint32(b[:4], uint32(8+len(payload)))
	binary.BigEndian.PutUint64(b[4:12], seq)
	copy(b[12:], payload)
	return b
}
func readFrame(r io.Reader) (uint64, []byte, error) {
	var h [4]byte
	if _, e := io.ReadFull(r, h[:]); e != nil {
		return 0, nil, e
	}
	b := make([]byte, binary.BigEndian.Uint32(h[:]))
	if _, e := io.ReadFull(r, b); e != nil {
		return 0, nil, e
	}
	return binary.BigEndian.Uint64(b[:8]), b[8:], nil
}
func fileEndpoint(path string) (endpoint, error) {
	tr := newTracker()
	done := make(chan struct{})
	go func() {
		var last uint64
		for {
			select {
			case <-done:
				return
			default:
			}
			b, e := os.ReadFile(path)
			if e == nil && len(b) >= 8 {
				n := binary.BigEndian.Uint64(b[:8])
				if n > last {
					last = n
					tr.mark(n)
				}
			}
			runtime.Gosched()
		}
	}()
	return endpoint{send: func(payload []byte, seq uint64, profile string) error {
		var ch chan struct{}
		if profile == observed {
			ch = tr.register(seq)
		}
		b := make([]byte, 8+len(payload))
		binary.BigEndian.PutUint64(b[:8], seq)
		copy(b[8:], payload)
		tmp := fmt.Sprintf("%s.tmp.%d", path, seq)
		if e := os.WriteFile(tmp, b, 0600); e != nil {
			return e
		}
		if e := os.Rename(tmp, path); e != nil {
			return e
		}
		if profile == observed {
			return waitObserved(ch)
		}
		return nil
	}, close: func() { close(done); os.Remove(path) }}, nil
}

func streamEndpoint(path string) (endpoint, error) {
	os.Remove(path)
	l, e := net.Listen("unix", path)
	if e != nil {
		return endpoint{}, e
	}
	tr := newTracker()
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			go func() {
				defer c.Close()
				seq, _, e := readFrame(c)
				if e == nil {
					tr.mark(seq)
					_, _ = c.Write([]byte{1})
				}
			}()
		}
	}()
	return endpoint{send: func(payload []byte, seq uint64, profile string) error {
		var ch chan struct{}
		if profile == observed {
			ch = tr.register(seq)
		}
		c, e := net.Dial("unix", path)
		if e != nil {
			return e
		}
		defer c.Close()
		if _, e = c.Write(frame(seq, payload)); e != nil {
			return e
		}
		switch profile {
		case observed:
			return waitObserved(ch)
		case acked:
			var a [1]byte
			_, e = io.ReadFull(c, a[:])
			return e
		default:
			return nil
		}
	}, close: func() { l.Close(); os.Remove(path) }}, nil
}

func dgramEndpoint(path string) (endpoint, error) {
	os.Remove(path)
	a, _ := net.ResolveUnixAddr("unixgram", path)
	l, e := net.ListenUnixgram("unixgram", a)
	if e != nil {
		return endpoint{}, e
	}
	tr := newTracker()
	go func() {
		b := make([]byte, 1<<20)
		for {
			n, _, e := l.ReadFromUnix(b)
			if e != nil {
				return
			}
			if n >= 8 {
				tr.mark(binary.BigEndian.Uint64(b[:8]))
			}
		}
	}()
	return endpoint{send: func(payload []byte, seq uint64, profile string) error {
		var ch chan struct{}
		if profile == observed {
			ch = tr.register(seq)
		}
		c, e := net.DialUnix("unixgram", nil, a)
		if e != nil {
			return e
		}
		defer c.Close()
		b := make([]byte, 8+len(payload))
		binary.BigEndian.PutUint64(b, seq)
		copy(b[8:], payload)
		if _, e = c.Write(b); e != nil {
			return e
		}
		if profile == observed {
			return waitObserved(ch)
		}
		return nil
	}, close: func() { l.Close(); os.Remove(path) }}, nil
}

func httpEndpoint() (endpoint, error) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return endpoint{}, e
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); w.WriteHeader(204) })
	s := &http.Server{Handler: mux}
	go s.Serve(l)
	client := &http.Client{Timeout: 2 * time.Second}
	return endpoint{send: func(payload []byte, _ uint64, _ string) error {
		r, e := client.Post("http://"+l.Addr().String()+"/ingest", "application/octet-stream", strings.NewReader(string(payload)))
		if e != nil {
			return e
		}
		r.Body.Close()
		if r.StatusCode != 204 {
			return fmt.Errorf("status %d", r.StatusCode)
		}
		return nil
	}, close: func() { s.Close() }}, nil
}

type rawCodec struct{}

func (rawCodec) Name() string                  { return "raw" }
func (rawCodec) Marshal(v any) ([]byte, error) { return *v.(*[]byte), nil }
func (rawCodec) Unmarshal(b []byte, v any) error {
	*v.(*[]byte) = append((*v.(*[]byte))[:0], b...)
	return nil
}

type grpcSink struct{}
type grpcService interface{ marker() }

func (*grpcSink) marker() {}
func grpcHandler(_ any, _ context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var b []byte
	if e := dec(&b); e != nil {
		return nil, e
	}
	out := []byte{}
	return &out, nil
}
func grpcEndpoint(path string) (endpoint, error) {
	os.Remove(path)
	l, e := net.Listen("unix", path)
	if e != nil {
		return endpoint{}, e
	}
	encoding.RegisterCodec(rawCodec{})
	s := grpc.NewServer()
	s.RegisterService(&grpc.ServiceDesc{ServiceName: "inv005.Ingest", HandlerType: (*grpcService)(nil),
		Methods: []grpc.MethodDesc{{MethodName: "Push", Handler: grpcHandler}}}, &grpcSink{})
	go s.Serve(l)
	c, e := grpc.NewClient("unix://"+path, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(rawCodec{})))
	if e != nil {
		return endpoint{}, e
	}
	return endpoint{send: func(payload []byte, _ uint64, _ string) error {
		out := []byte{}
		return c.Invoke(context.Background(), "/inv005.Ingest/Push", &payload, &out)
	}, close: func() { c.Close(); s.Stop(); os.Remove(path) }}, nil
}

func mmapEndpoint(path string, shm bool, size int) (endpoint, error) {
	if shm {
		path = filepath.Join("/dev/shm", filepath.Base(path))
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return endpoint{}, e
	}
	if e = f.Truncate(int64(size + 16)); e != nil {
		return endpoint{}, e
	}
	m, e := syscall.Mmap(int(f.Fd()), 0, size+16, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if e != nil {
		return endpoint{}, e
	}
	var mu sync.Mutex
	tr := newTracker()
	var last uint64
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			n := atomic.LoadUint64((*uint64)(unsafe.Pointer(&m[0])))
			if n != last {
				last = n
				tr.mark(n)
			}
			runtime.Gosched()
		}
	}()
	return endpoint{send: func(payload []byte, seq uint64, profile string) error {
		var ch chan struct{}
		if profile == observed {
			ch = tr.register(seq)
		}
		mu.Lock()
		copy(m[16:], payload)
		atomic.StoreUint64((*uint64)(unsafe.Pointer(&m[0])), seq)
		mu.Unlock()
		if profile == observed {
			return waitObserved(ch)
		}
		return nil
	}, close: func() { close(done); syscall.Munmap(m); f.Close(); os.Remove(path) }}, nil
}

var profiles = map[string][]string{
	"file": {publish, observed}, "unix-stream": {publish, observed, acked},
	"unix-dgram": {publish, observed}, "http": {acked}, "grpc": {acked},
	"shared-memory": {publish, observed}, "mmap": {publish, observed},
}

func makeEndpoint(t, dir string, size int) (endpoint, error) {
	p := filepath.Join(dir, t)
	switch t {
	case "file":
		return fileEndpoint(p)
	case "unix-stream":
		return streamEndpoint(p)
	case "unix-dgram":
		return dgramEndpoint(p)
	case "http":
		return httpEndpoint()
	case "grpc":
		return grpcEndpoint(p)
	case "shared-memory":
		return mmapEndpoint(p, true, size)
	case "mmap":
		return mmapEndpoint(p, false, size)
	}
	return endpoint{}, fmt.Errorf("unknown transport")
}

func run(t, profile, name string, producers, count, size int, dir string) scenarioResult {
	e, err := makeEndpoint(t, dir, size)
	if err != nil {
		panic(err)
	}
	defer e.close()
	payload := make([]byte, size)
	out := make(chan sample, producers*count)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var sequence atomic.Uint64
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			<-start
			for i := 0; i < count; i++ {
				seq := sequence.Add(1)
				st := time.Now()
				err := e.send(payload, seq, profile)
				out <- sample{t, profile, name, p, i, size, time.Since(st), err == nil}
			}
		}(p)
	}
	wallStart := time.Now()
	close(start)
	wg.Wait()
	elapsed := time.Since(wallStart)
	close(out)
	var samples []sample
	for x := range out {
		samples = append(samples, x)
	}
	return scenarioResult{t, profile, name, samples, elapsed}
}
func percentile(v []int64, p float64) int64 {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	if len(v) == 0 {
		return 0
	}
	return v[int(float64(len(v)-1)*p)]
}

func runProbes(out string) {
	f, _ := os.Create(filepath.Join(out, "probes.tsv"))
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = '\t'
	defer w.Flush()
	w.Write([]string{"probe", "expected", "actual", "result"})
	missing := filepath.Join(os.TempDir(), "inv005-missing.sock")
	_, e := net.DialTimeout("unix", missing, 50*time.Millisecond)
	writeProbe(w, "missing_unix_socket_rejected", "error", boolWord(e != nil), e != nil)
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	l.Close()
	c := &http.Client{Timeout: 100 * time.Millisecond}
	_, e = c.Get("http://" + addr)
	writeProbe(w, "http_connection_refused", "error", boolWord(e != nil), e != nil)
	src, _ := os.CreateTemp("/tmp", "inv005-cross-device-")
	src.WriteString("x")
	src.Close()
	dst := filepath.Join("/dev/shm", filepath.Base(src.Name()))
	e = os.Rename(src.Name(), dst)
	writeProbe(w, "cross_filesystem_rename_rejected", "error", boolWord(e != nil), e != nil)
	os.Remove(src.Name())
	os.Remove(dst)
	p := filepath.Join(os.TempDir(), "inv005-remap")
	mf, _ := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	mf.Truncate(4096)
	m, _ := syscall.Mmap(int(mf.Fd()), 0, 4096, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	mf.Truncate(8192)
	unchanged := len(m) == 4096
	writeProbe(w, "mmap_length_unchanged_after_grow", "true", boolWord(unchanged), unchanged)
	syscall.Munmap(m)
	mf.Close()
	os.Remove(p)
	// Measure the largest AF_UNIX datagram accepted in this environment.
	dir, _ := os.MkdirTemp("", "inv005-probe-")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "d")
	a, _ := net.ResolveUnixAddr("unixgram", path)
	dl, _ := net.ListenUnixgram("unixgram", a)
	defer dl.Close()
	go func() {
		b := make([]byte, 2<<20)
		for {
			if _, e := dl.Read(b); e != nil {
				return
			}
		}
	}()
	dc, _ := net.DialUnix("unixgram", nil, a)
	defer dc.Close()
	lo, hi := 1, 1<<20
	for lo < hi {
		mid := (lo + hi + 1) / 2
		dc.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
		if _, e = dc.Write(make([]byte, mid)); e == nil {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	writeProbe(w, "unix_datagram_max_accepted_bytes", ">0", strconv.Itoa(lo), lo > 0)
}
func boolWord(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
func writeProbe(w *csv.Writer, name, expected, actual string, pass bool) {
	r := "fail"
	if pass {
		r = "pass"
	}
	w.Write([]string{name, expected, actual, r})
}
func integrationServer(kind, out string) {
	switch kind {
	case "unix-stream":
		path := "/shared/php.sock"
		os.Remove(path)
		l, e := net.Listen("unix", path)
		if e != nil {
			panic(e)
		}
		defer l.Close()
		c, e := l.Accept()
		if e != nil {
			panic(e)
		}
		b, _ := io.ReadAll(c)
		c.Close()
		os.WriteFile(filepath.Join(out, "php-unix.received"), b, 0644)
	case "http":
		done := make(chan struct{})
		s := &http.Server{Addr: "127.0.0.1:19090"}
		http.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			os.WriteFile(filepath.Join(out, "php-http.received"), b, 0644)
			w.WriteHeader(204)
			close(done)
		})
		go func() {
			if e := s.ListenAndServe(); e != nil && e != http.ErrServerClosed {
				panic(e)
			}
		}()
		<-done
		s.Shutdown(context.Background())
	}
}

func main() {
	out := flag.String("out", "/results", "output")
	count := flag.Int("count", 500, "operations per producer")
	probesOnly := flag.Bool("probes", false, "run executable probes")
	integration := flag.String("integration-server", "", "serve one PHP integration request")
	flag.Parse()
	os.MkdirAll(*out, 0755)
	if *integration != "" {
		integrationServer(*integration, *out)
		return
	}
	if *probesOnly {
		runProbes(*out)
		return
	}
	dir, _ := os.MkdirTemp("", "inv005-")
	defer os.RemoveAll(dir)
	transports := []string{"file", "unix-stream", "unix-dgram", "http", "grpc", "shared-memory", "mmap"}
	scenarios := []struct {
		name            string
		producers, size int
	}{{"baseline", 1, 64}, {"multi4", 4, 64}, {"payload1k", 1, 1024}, {"payload16k", 1, 16384}}
	var results []scenarioResult
	for _, t := range transports {
		for _, p := range profiles[t] {
			for _, s := range scenarios {
				results = append(results, run(t, p, s.name, s.producers, *count, s.size, dir))
			}
		}
	}
	sf, _ := os.Create(filepath.Join(*out, "samples.tsv"))
	sw := csv.NewWriter(sf)
	sw.Comma = '\t'
	sw.Write([]string{"transport", "profile", "scenario", "producer", "sequence", "bytes", "latency_ns", "result"})
	sumf, _ := os.Create(filepath.Join(*out, "summary.tsv"))
	sumw := csv.NewWriter(sumf)
	sumw.Comma = '\t'
	sumw.Write([]string{"transport", "profile", "scenario", "operations", "passed", "wall_elapsed_ns", "aggregate_ops_s", "p50_us", "p95_us", "p99_us"})
	for _, r := range results {
		var lat []int64
		passed := 0
		for _, s := range r.samples {
			result := "fail"
			if s.ok {
				result = "pass"
				passed++
			}
			lat = append(lat, s.latency.Nanoseconds())
			sw.Write([]string{s.transport, s.profile, s.scenario, strconv.Itoa(s.producer), strconv.Itoa(s.seq), strconv.Itoa(s.bytes), strconv.FormatInt(s.latency.Nanoseconds(), 10), result})
		}
		throughput := float64(len(r.samples)) / r.elapsed.Seconds()
		sumw.Write([]string{r.transport, r.profile, r.scenario, strconv.Itoa(len(r.samples)), strconv.Itoa(passed), strconv.FormatInt(r.elapsed.Nanoseconds(), 10),
			fmt.Sprintf("%.1f", throughput), fmt.Sprintf("%.3f", float64(percentile(lat, .5))/1e3), fmt.Sprintf("%.3f", float64(percentile(lat, .95))/1e3), fmt.Sprintf("%.3f", float64(percentile(lat, .99))/1e3)})
	}
	sw.Flush()
	sf.Close()
	sumw.Flush()
	sumf.Close()
}
