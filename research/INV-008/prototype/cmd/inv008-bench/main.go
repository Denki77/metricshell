package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"metricshell/inv008/api"
)

const maxPayload = 1 << 20
const maxHTTPBody = 2*maxPayload + 64*1024

var (
	errEmpty    = errors.New("empty records")
	errTooLarge = errors.New("decoded payload exceeds limit")
)

type store struct{ accepted atomic.Uint64 }

func (s *store) accept(records [][]byte) error {
	var n int
	for _, r := range records {
		n += len(r)
	}
	if len(records) == 0 {
		return errEmpty
	}
	if n > maxPayload {
		return errTooLarge
	}
	s.accepted.Add(uint64(len(records)))
	return nil
}

type grpcSvc struct {
	api.UnimplementedIngestServer
	s *store
}

func (g grpcSvc) Push(_ context.Context, r *api.PushRequest) (*api.PushReply, error) {
	if err := g.s.accept(r.Records); err != nil {
		code := codes.InvalidArgument
		if errors.Is(err, errTooLarge) {
			code = codes.ResourceExhausted
		}
		return nil, status.Error(code, err.Error())
	}
	return &api.PushReply{Accepted: uint64(len(r.Records))}, nil
}

type result struct {
	transport                           string
	payload, batch, producers, requests int
	duration                            time.Duration
	accepted                            uint64
	lat                                 []float64
	errors                              int
}

func pct(v []float64, p float64) float64 {
	sort.Float64s(v)
	if len(v) == 0 {
		return 0
	}
	return v[int(float64(len(v)-1)*p)]
}

func startUnix(path string, s *store) (net.Listener, func()) {
	_ = os.Remove(path)
	l, e := net.Listen("unix", path)
	if e != nil {
		panic(e)
	}
	stop := make(chan struct{})
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				b := bufio.NewReader(c)
				for {
					var count uint32
					if binary.Read(b, binary.BigEndian, &count) != nil {
						return
					}
					records := make([][]byte, 0, min(int(count), 1024))
					total := 0
					invalid := count > 65536
					for i := uint32(0); i < count; i++ {
						var n uint32
						if binary.Read(b, binary.BigEndian, &n) != nil {
							return
						}
						if n > maxPayload || total+int(n) > maxPayload {
							invalid = true
							if _, e = io.CopyN(io.Discard, b, int64(n)); e != nil {
								return
							}
							continue
						}
						p := make([]byte, n)
						if _, e = io.ReadFull(b, p); e != nil {
							return
						}
						total += int(n)
						records = append(records, p)
					}
					err := errTooLarge
					if !invalid {
						err = s.accept(records)
					}
					var code uint32
					if err != nil {
						code = 1
					}
					_ = binary.Write(c, binary.BigEndian, code)
				}
			}(c)
		}
	}()
	return l, func() { close(stop); l.Close(); os.Remove(path) }
}
func startHTTP(s *store) (net.Listener, func()) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		panic(e)
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/metrics" {
			http.Error(w, "not found", 404)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxHTTPBody)
		var x struct {
			Records [][]byte `json:"records"`
		}
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&x); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, "encoded body too large", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "malformed json", http.StatusBadRequest)
			}
			return
		}
		if err := s.accept(x.Records); err != nil {
			if errors.Is(err, errTooLarge) {
				http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			}
			return
		}
		w.Header().Set("content-type", "application/json")
		fmt.Fprintf(w, `{"accepted":%d}`, len(x.Records))
	})
	srv := &http.Server{Handler: h, ReadHeaderTimeout: time.Second}
	go srv.Serve(l)
	return l, func() { srv.Close(); l.Close() }
}
func startGRPC(s *store) (net.Listener, func()) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		panic(e)
	}
	g := grpc.NewServer(grpc.MaxRecvMsgSize(maxPayload + 64*1024))
	api.RegisterIngestServer(g, grpcSvc{s: s})
	go g.Serve(l)
	return l, func() { g.Stop(); l.Close() }
}

func runUnix(path string, payload, batch, producers, requests int) result {
	r := result{transport: "unix", payload: payload, batch: batch, producers: producers, requests: requests}
	var wg sync.WaitGroup
	var mu sync.Mutex
	start := time.Now()
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, e := net.Dial("unix", path)
			if e != nil {
				mu.Lock()
				r.errors++
				mu.Unlock()
				return
			}
			defer c.Close()
			record := bytes.Repeat([]byte("x"), payload)
			var body bytes.Buffer
			_ = binary.Write(&body, binary.BigEndian, uint32(batch))
			for j := 0; j < batch; j++ {
				_ = binary.Write(&body, binary.BigEndian, uint32(len(record)))
				_, _ = body.Write(record)
			}
			wire := body.Bytes()
			for i := 0; i < requests; i++ {
				t := time.Now()
				_, e = c.Write(wire)
				var code uint32
				if e == nil {
					e = binary.Read(c, binary.BigEndian, &code)
				}
				mu.Lock()
				if e != nil || code != 0 {
					r.errors++
				} else {
					r.accepted += uint64(batch)
					r.lat = append(r.lat, float64(time.Since(t).Microseconds())/1000)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	r.duration = time.Since(start)
	return r
}
func runHTTP(addr string, payload, batch, producers, requests int) result {
	r := result{transport: "http", payload: payload, batch: batch, producers: producers, requests: requests}
	var wg sync.WaitGroup
	var mu sync.Mutex
	records := make([][]byte, batch)
	for i := range records {
		records[i] = bytes.Repeat([]byte("x"), payload)
	}
	body, _ := json.Marshal(struct {
		Records [][]byte `json:"records"`
	}{records})
	start := time.Now()
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cl := &http.Client{Timeout: 5 * time.Second}
			for i := 0; i < requests; i++ {
				t := time.Now()
				resp, e := cl.Post("http://"+addr+"/v1/metrics", "application/json", bytes.NewReader(body))
				if e == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				mu.Lock()
				if e != nil || resp.StatusCode != 200 {
					r.errors++
				} else {
					r.accepted += uint64(batch)
					r.lat = append(r.lat, float64(time.Since(t).Microseconds())/1000)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	r.duration = time.Since(start)
	return r
}
func runGRPC(addr string, payload, batch, producers, requests int) result {
	r := result{transport: "grpc", payload: payload, batch: batch, producers: producers, requests: requests}
	var wg sync.WaitGroup
	var mu sync.Mutex
	records := make([][]byte, batch)
	for i := range records {
		records[i] = bytes.Repeat([]byte("x"), payload)
	}
	start := time.Now()
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cc, e := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if e != nil {
				mu.Lock()
				r.errors++
				mu.Unlock()
				return
			}
			defer cc.Close()
			cl := api.NewIngestClient(cc)
			for i := 0; i < requests; i++ {
				t := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, e = cl.Push(ctx, &api.PushRequest{Records: records})
				cancel()
				mu.Lock()
				if e != nil {
					r.errors++
				} else {
					r.accepted += uint64(batch)
					r.lat = append(r.lat, float64(time.Since(t).Microseconds())/1000)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	r.duration = time.Since(start)
	return r
}

func unixPost(path string, records [][]byte) string {
	c, err := net.Dial("unix", path)
	if err != nil {
		return "transport_error"
	}
	defer c.Close()
	_ = binary.Write(c, binary.BigEndian, uint32(len(records)))
	for _, record := range records {
		_ = binary.Write(c, binary.BigEndian, uint32(len(record)))
		if _, err = c.Write(record); err != nil {
			return "transport_error"
		}
	}
	var code uint32
	if binary.Read(c, binary.BigEndian, &code) != nil {
		return "transport_error"
	}
	if code == 0 {
		return "accepted"
	}
	return "rejected"
}

func httpPost(addr string, records [][]byte) string {
	body, _ := json.Marshal(struct {
		Records [][]byte `json:"records"`
	}{records})
	resp, err := http.Post("http://"+addr+"/v1/metrics", "application/json", bytes.NewReader(body))
	if err != nil {
		return "transport_error"
	}
	defer resp.Body.Close()
	return strconv.Itoa(resp.StatusCode)
}

func grpcPost(addr string, records [][]byte) string {
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "transport_error"
	}
	defer cc.Close()
	_, err = api.NewIngestClient(cc).Push(context.Background(), &api.PushRequest{Records: records})
	if err == nil {
		return "accepted"
	}
	return status.Code(err).String()
}

func proc() (uint64, uint64) {
	b, _ := os.ReadFile("/proc/self/stat")
	f := strings.Fields(string(b))
	var ticks uint64
	if len(f) > 14 {
		a, _ := strconv.ParseUint(f[13], 10, 64)
		c, _ := strconv.ParseUint(f[14], 10, 64)
		ticks = a + c
	}
	b, _ = os.ReadFile("/proc/self/status")
	var rss uint64
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "VmHWM:") {
			fmt.Sscanf(l, "VmHWM: %d kB", &rss)
		}
	}
	return ticks, rss
}
func main() {
	out := "/out"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	_ = os.MkdirAll(out, 0755)
	sock := filepath.Join(os.TempDir(), "inv008.sock")
	s := &store{}
	_, stopU := startUnix(sock, s)
	hl, stopH := startHTTP(s)
	gl, stopG := startGRPC(s)
	defer stopU()
	defer stopH()
	defer stopG()
	perf, _ := os.Create(filepath.Join(out, "performance.tsv"))
	fmt.Fprintln(perf, "transport\tpayload_bytes\tbatch\tproducers\trequests_per_producer\taccepted\terrors\tduration_ms\tthroughput_records_s\tp50_ms\tp95_ms\tp99_ms")
	var all []result
	for _, size := range []int{64, 1024, 16384} {
		for _, batch := range []int{1, 16} {
			for _, prod := range []int{1, 8} {
				req := 300
				if size == 16384 {
					req = 100
				}
				all = append(all, runUnix(sock, size, batch, prod, req), runHTTP(hl.Addr().String(), size, batch, prod, req), runGRPC(gl.Addr().String(), size, batch, prod, req))
			}
		}
	}
	for _, r := range all {
		d := float64(r.duration.Microseconds()) / 1000
		thr := float64(r.accepted) / r.duration.Seconds()
		fmt.Fprintf(perf, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%.3f\t%.1f\t%.3f\t%.3f\t%.3f\n", r.transport, r.payload, r.batch, r.producers, r.requests, r.accepted, r.errors, d, thr, pct(r.lat, .5), pct(r.lat, .95), pct(r.lat, .99))
	}
	perf.Close()
	corr, _ := os.Create(filepath.Join(out, "correctness.tsv"))
	fmt.Fprintln(corr, "case\texpected\tactual\tresult")
	check := func(n, e, a string) {
		x := "fail"
		if e == a {
			x = "pass"
		}
		fmt.Fprintf(corr, "%s\t%s\t%s\t%s\n", n, e, a, x)
	}
	resp, e := http.Post("http://"+hl.Addr().String()+"/v1/metrics", "application/json", strings.NewReader("{"))
	a := "transport_error"
	if e == nil {
		a = strconv.Itoa(resp.StatusCode)
		resp.Body.Close()
	}
	check("http_malformed", "400", a)
	check("http_empty_records", "422", httpPost(hl.Addr().String(), nil))
	check("http_encoded_body_limit", "413", httpPost(hl.Addr().String(), [][]byte{bytes.Repeat([]byte("x"), maxHTTPBody)}))
	atLimit := [][]byte{bytes.Repeat([]byte("x"), maxPayload)}
	overLimit := [][]byte{bytes.Repeat([]byte("x"), maxPayload+1)}
	check("unix_decoded_1mib", "accepted", unixPost(sock, atLimit))
	check("unix_decoded_1mib_plus_1", "rejected", unixPost(sock, overLimit))
	check("http_decoded_1mib", "200", httpPost(hl.Addr().String(), atLimit))
	check("http_decoded_1mib_plus_1", "413", httpPost(hl.Addr().String(), overLimit))
	check("grpc_decoded_1mib", "accepted", grpcPost(gl.Addr().String(), atLimit))
	check("grpc_decoded_1mib_plus_1", "ResourceExhausted", grpcPost(gl.Addr().String(), overLimit))
	check("all_matrix_requests", "zero_errors", func() string {
		for _, r := range all {
			if r.errors != 0 {
				return "errors"
			}
		}
		return "zero_errors"
	}())
	check("http_loopback_bind", "127.0.0.1", hl.Addr().(*net.TCPAddr).IP.String())
	check("grpc_loopback_bind", "127.0.0.1", gl.Addr().(*net.TCPAddr).IP.String())
	before, _ := s.accepted.Load(), e
	stopH()
	_, e = http.Get("http://" + hl.Addr().String() + "/v1/metrics")
	after := s.accepted.Load()
	restart := "server_unavailable_state_retained"
	if e == nil || after != before {
		restart = "unexpected"
	}
	check("http_shutdown", "server_unavailable_state_retained", restart)
	hl, stopH = startHTTP(s)
	rr := runHTTP(hl.Addr().String(), 64, 1, 1, 10)
	check("http_restart", "accepted_10", fmt.Sprintf("accepted_%d", rr.accepted))
	corr.Close()
	t0, _ := proc()
	time.Sleep(2 * time.Second)
	t1, rss1 := proc()
	idle, _ := os.Create(filepath.Join(out, "resources.tsv"))
	fmt.Fprintln(idle, "phase\tticks\telapsed_ms\tcpu_percent_one_core\tvmhwm_kib")
	fmt.Fprintf(idle, "idle\t%d\t2000\t%.3f\t%d\n", t1-t0, float64(t1-t0)/2, rss1)
	a0 := time.Now()
	t0, _ = proc()
	ar := runGRPC(gl.Addr().String(), 1024, 16, 8, 500)
	t1, rss1 = proc()
	fmt.Fprintf(idle, "grpc_active\t%d\t%.3f\t%.3f\t%d\n", t1-t0, float64(time.Since(a0).Microseconds())/1000, float64(t1-t0)/time.Since(a0).Seconds(), rss1)
	idle.Close()
	_ = ar
	sum, _ := os.Create(filepath.Join(out, "summary.tsv"))
	fmt.Fprintln(sum, "metric\tvalue")
	fmt.Fprintf(sum, "matrix_rows\t%d\n", len(all))
	fmt.Fprintf(sum, "matrix_errors\t%d\n", func() int {
		x := 0
		for _, r := range all {
			x += r.errors
		}
		return x
	}())
	fmt.Fprintf(sum, "correctness_failures\t%d\n", func() int {
		b, _ := os.ReadFile(filepath.Join(out, "correctness.tsv"))
		return strings.Count(string(b), "\tfail\n")
	}())
	fmt.Fprintf(sum, "accepted_total\t%d\n", s.accepted.Load())
	sum.Close()
	env, _ := os.Create(filepath.Join(out, "container-environment.tsv"))
	fmt.Fprintln(env, "key\tvalue")
	fmt.Fprintf(env, "go_version\t%s\narchitecture\t%s\nos\t%s\nmax_payload_bytes\t%d\n", runtime.Version(), runtime.GOARCH, runtime.GOOS, maxPayload)
	env.Close()
}
