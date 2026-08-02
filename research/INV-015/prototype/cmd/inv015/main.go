package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

type snapshot struct {
	body   string
	series int
}

func main() {
	mode := flag.String("mode", "server", "server, benchmark, or allocate")
	addr := flag.String("addr", ":19115", "")
	series := flag.Int("series", 1000, "")
	required := flag.Int64("required-scrapes", 0, "")
	wait := flag.Duration("wait", 15*time.Second, "")
	iterations := flag.Int("iterations", 10, "")
	allocate := flag.Int("allocate-bytes", 0, "")
	maxConcurrent := flag.Int("max-concurrent", 4, "")
	chunkDelay := flag.Duration("chunk-delay", 0, "")
	flag.Parse()
	switch *mode {
	case "benchmark":
		runBench(*iterations)
		return
	case "allocate":
		b := make([]byte, *allocate)
		for i := range b {
			if i%4096 == 0 {
				b[i] = 1
			}
		}
		fmt.Printf("allocated=%d\n", len(b))
		return
	}
	runServer(*addr, *series, *required, *wait, *maxConcurrent, *chunkDelay)
}

func runServer(addr string, initialSeries int, required int64, wait time.Duration, maxConcurrent int, chunkDelay time.Duration) {
	var active atomic.Pointer[snapshot]
	active.Store(generate("initial", initialSeries))
	var scrapes atomic.Int64
	var rejected atomic.Int64
	sem := make(chan struct{}, maxConcurrent)
	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	mux.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			http.Error(w, "busy", 429)
			return
		}
		if h, _ := strconv.Atoi(r.Header.Get("X-Research-Hold-Millis")); h > 0 {
			time.Sleep(time.Duration(h) * time.Millisecond)
		}
		n, e := strconv.Atoi(r.URL.Query().Get("series"))
		if e != nil || n < 0 || n > 200000 {
			rejected.Add(1)
			http.Error(w, "series", 422)
			return
		}
		active.Store(generate(r.URL.Query().Get("generation"), n))
		w.WriteHeader(204)
	})
	mux.HandleFunc("/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", 405)
			return
		}
		b, e := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if e != nil || len(b) == 0 {
			rejected.Add(1)
			http.Error(w, "malformed", 400)
			return
		}
		sc := bufio.NewScanner(strings.NewReader(string(b)))
		n := 0
		for sc.Scan() {
			p := strings.Fields(sc.Text())
			if len(p) != 2 || strings.ContainsAny(p[0], "{}") {
				rejected.Add(1)
				http.Error(w, "malformed", 400)
				return
			}
			if _, e = strconv.ParseFloat(p[1], 64); e != nil {
				rejected.Add(1)
				http.Error(w, "malformed", 400)
				return
			}
			n++
		}
		if n == 0 {
			rejected.Add(1)
			http.Error(w, "malformed", 400)
			return
		}
		active.Store(&snapshot{body: strings.TrimSpace(string(b)) + "\n", series: n})
		w.WriteHeader(204)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if h, _ := strconv.Atoi(r.Header.Get("X-Research-Hold-Millis")); h > 0 && h <= 2000 {
			time.Sleep(time.Duration(h) * time.Millisecond)
		}
		s := active.Load()
		body := s.body + fmt.Sprintf("metricshell_snapshot_rejections_total %d\n", rejected.Load())
		complete := true
		for offset := 0; offset < len(body); offset += 16 << 10 {
			end := offset + (16 << 10)
			if end > len(body) {
				end = len(body)
			}
			if _, e := io.WriteString(w, body[offset:end]); e != nil {
				complete = false
				break
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			if chunkDelay > 0 {
				time.Sleep(chunkDelay)
			}
		}
		if complete && r.Context().Err() == nil {
			n := scrapes.Add(1)
			if required > 0 && n >= required {
				closeDone()
			}
		}
	})
	ln, e := net.Listen("tcp", addr)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(70)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 20 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	fmt.Printf("ready pid=%d series=%d\n", os.Getpid(), initialSeries)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if required == 0 {
		<-signalCtx.Done()
	} else {
		timer := time.AfterFunc(wait, closeDone)
		defer timer.Stop()
		select {
		case <-done:
		case <-signalCtx.Done():
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Printf("final scrapes=%d required=%d\n", scrapes.Load(), required)
}

func generate(generation string, n int) *snapshot {
	if generation == "" {
		generation = "generated"
	}
	var b strings.Builder
	b.Grow(n * 45)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "inv015_series{generation=%q,id=%q} %d\n", generation, strconv.Itoa(i), i)
	}
	return &snapshot{body: b.String(), series: n}
}

func runBench(iterations int) {
	fmt.Println("section\tshape\titeration\tmetric\tvalue\tunit")
	for i := 0; i < 3; i++ {
		_ = generate("warmup", 10000)
	}
	for _, rate := range []int{100, 1000, 10000} {
		for it := 1; it <= iterations; it++ {
			count := rate / 10
			if count < 1 {
				count = 1
			}
			lat := make([]int64, 0, count)
			var p atomic.Pointer[snapshot]
			p.Store(generate("old", 100))
			started := time.Now()
			interval := time.Second / time.Duration(rate)
			next := started
			for j := 0; j < count; j++ {
				for time.Now().Before(next) {
					runtime.Gosched()
				}
				s := time.Now()
				p.Store(generate(strconv.Itoa(j), 100))
				lat = append(lat, time.Since(s).Nanoseconds())
				next = next.Add(interval)
			}
			elapsed := time.Since(started)
			emitStats("ingestion", strconv.Itoa(rate), it, lat)
			fmt.Printf("ingestion\t%d\t%d\taccepted\t%d\tcount\n", rate, it, count)
			fmt.Printf("ingestion\t%d\t%d\tachieved_per_second\t%.3f\tops/s\n", rate, it, float64(count)/elapsed.Seconds())
		}
	}
	for _, n := range []int{100, 1000, 10000, 100000} {
		for it := 1; it <= iterations; it++ {
			s := time.Now()
			snap := generate("cardinality", n)
			elapsed := time.Since(s).Nanoseconds()
			fmt.Printf("cardinality\t%d\t%d\tencode_ns\t%d\tns\n", n, it, elapsed)
			fmt.Printf("cardinality\t%d\t%d\tbytes\t%d\tbytes\n", n, it, len(snap.body))
		}
	}
	base := generate("concurrent", 10000)
	for _, c := range []int{1, 2, 5, 10} {
		for it := 1; it <= iterations; it++ {
			started := time.Now()
			var wg sync.WaitGroup
			for j := 0; j < c; j++ {
				wg.Add(1)
				go func() { defer wg.Done(); _ = len(base.body) }()
			}
			wg.Wait()
			fmt.Printf("concurrent_scrape\t%d\t%d\twall_ns\t%d\tns\n", c, it, time.Since(started).Nanoseconds())
		}
	}
	for _, mode := range []string{"polling", "inotify", "hybrid"} {
		for it := 1; it <= iterations; it++ {
			lat, err := detectFile(mode)
			if err != nil {
				fmt.Fprintf(os.Stderr, "detect %s: %v\n", mode, err)
				os.Exit(1)
			}
			fmt.Printf("file_detection\t%s\t%d\tlatency_ns\t%d\tns\n", mode, it, lat)
		}
	}
	for it := 1; it <= iterations; it++ {
		s := time.Now()
		_ = generate("startup", 1000)
		fmt.Printf("initialization\t1000\t%d\tlatency_ns\t%d\tns\n", it, time.Since(s).Nanoseconds())
	}
}

func emitStats(section, shape string, it int, v []int64) {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	pick := func(q float64) int64 { idx := int(float64(len(v)-1) * q); return v[idx] }
	fmt.Printf("%s\t%s\t%d\tp50_ns\t%d\tns\n", section, shape, it, pick(.50))
	fmt.Printf("%s\t%s\t%d\tp95_ns\t%d\tns\n", section, shape, it, pick(.95))
	fmt.Printf("%s\t%s\t%d\tp99_ns\t%d\tns\n", section, shape, it, pick(.99))
}

func detectFile(mode string) (int64, error) {
	f, err := os.CreateTemp("", "inv015-detect-")
	if err != nil {
		return 0, err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)
	ready := make(chan struct{})
	done := make(chan error, 2)
	if mode == "polling" {
		info, _ := os.Stat(path)
		go func() {
			close(ready)
			for {
				cur, statErr := os.Stat(path)
				if statErr != nil {
					done <- statErr
					return
				}
				if cur.Size() != info.Size() {
					done <- nil
					return
				}
				time.Sleep(100 * time.Microsecond)
			}
		}()
	} else {
		fd, initErr := syscall.InotifyInit1(syscall.IN_CLOEXEC)
		if initErr != nil {
			return 0, initErr
		}
		if _, addErr := syscall.InotifyAddWatch(fd, path, syscall.IN_MODIFY|syscall.IN_CLOSE_WRITE); addErr != nil {
			return 0, addErr
		}
		go func() {
			defer syscall.Close(fd)
			close(ready)
			buf := make([]byte, 4096)
			_, _, errno := syscall.Syscall(syscall.SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
			if errno != 0 {
				done <- errno
				return
			}
			done <- nil
		}()
		if mode == "hybrid" {
			info, _ := os.Stat(path)
			go func() {
				for {
					cur, statErr := os.Stat(path)
					if statErr != nil {
						done <- statErr
						return
					}
					if cur.Size() != info.Size() {
						done <- nil
						return
					}
					time.Sleep(time.Millisecond)
				}
			}()
		}
	}
	<-ready
	started := time.Now()
	if err := os.WriteFile(path, []byte("complete snapshot\n"), 0600); err != nil {
		return 0, err
	}
	select {
	case err = <-done:
	case <-time.After(time.Second):
		return 0, fmt.Errorf("timeout")
	}
	return time.Since(started).Nanoseconds(), err
}
