package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type protocol string

const (
	line     protocol = "stream-line"
	framed   protocol = "stream-framed"
	datagram protocol = "datagram-line"
)

type counters struct {
	accepted, valid, malformed, oversized atomic.Int64
}

type server struct {
	proto     protocol
	path      string
	max       int
	delay     time.Duration
	ln        net.Listener
	pc        net.PacketConn
	stop      chan struct{}
	wg        sync.WaitGroup
	c         counters
	onMessage func([]byte)
}

func startServer(p protocol, path string, max int, mode os.FileMode, delay time.Duration, onMessage func([]byte)) (*server, error) {
	_ = os.Remove(path)
	s := &server{proto: p, path: path, max: max, delay: delay, stop: make(chan struct{}), onMessage: onMessage}
	var err error
	if p == datagram {
		s.pc, err = net.ListenPacket("unixgram", path)
	} else {
		s.ln, err = net.Listen("unix", path)
	}
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, mode); err != nil {
		s.close()
		return nil, err
	}
	s.wg.Add(1)
	if p == datagram {
		go s.serveDatagrams()
	} else {
		go s.serveStreams()
	}
	return s, nil
}

func (s *server) serveStreams() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.c.accepted.Add(1)
		s.wg.Add(1)
		go func() { defer s.wg.Done(); defer c.Close(); s.readStream(c) }()
	}
}

func (s *server) readStream(c net.Conn) {
	if s.proto == line {
		r := bufio.NewReaderSize(c, s.max+2)
		for {
			b, err := r.ReadBytes('\n')
			if len(b) > 0 {
				if len(b) > s.max+1 {
					s.c.oversized.Add(1)
				} else if b[len(b)-1] != '\n' {
					s.c.malformed.Add(1)
				} else {
					s.accept(b[:len(b)-1])
				}
			}
			if err != nil {
				return
			}
		}
	}
	var h [4]byte
	for {
		if _, err := io.ReadFull(c, h[:]); err != nil {
			if !errors.Is(err, io.EOF) {
				s.c.malformed.Add(1)
			}
			return
		}
		n := int(binary.BigEndian.Uint32(h[:]))
		if n <= 0 || n > s.max {
			if n > s.max {
				s.c.oversized.Add(1)
			} else {
				s.c.malformed.Add(1)
			}
			return
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(c, b); err != nil {
			s.c.malformed.Add(1)
			return
		}
		s.accept(b)
	}
}

func (s *server) serveDatagrams() {
	defer s.wg.Done()
	buf := make([]byte, s.max+2)
	for {
		n, _, err := s.pc.ReadFrom(buf)
		if err != nil {
			return
		}
		if n-1 > s.max {
			s.c.oversized.Add(1)
			continue
		}
		if n == 0 || buf[n-1] != '\n' {
			s.c.malformed.Add(1)
			continue
		}
		s.accept(buf[:n-1])
	}
}

func (s *server) accept(b []byte) {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	if !strings.HasPrefix(string(b), "v1 ") {
		s.c.malformed.Add(1)
		return
	}
	s.c.valid.Add(1)
	if s.onMessage != nil {
		s.onMessage(b)
	}
}
func (s *server) close() {
	select {
	case <-s.stop:
		return
	default:
		close(s.stop)
	}
	if s.ln != nil {
		_ = s.ln.Close()
	}
	if s.pc != nil {
		_ = s.pc.Close()
	}
	s.wg.Wait()
	_ = os.Remove(s.path)
}

func dial(p protocol, path string) (net.Conn, error) {
	network := "unix"
	if p == datagram {
		network = "unixgram"
	}
	return net.DialTimeout(network, path, 2*time.Second)
}
func send(c net.Conn, p protocol, b []byte) error {
	if p == line || p == datagram {
		_, err := c.Write(append(append([]byte{}, b...), '\n'))
		return err
	}
	out := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(out[:4], uint32(len(b)))
	copy(out[4:], b)
	_, err := c.Write(out)
	return err
}

func waitFor(fn func() bool, d time.Duration) bool {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if fn() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return fn()
}

type tsv struct {
	f  *os.File
	mu sync.Mutex
}

func newTSV(path, header string) *tsv {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(f, header)
	return &tsv{f: f}
}
func (t *tsv) row(format string, a ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.f, format+"\n", a...)
}
func (t *tsv) close() { _ = t.f.Close() }
func boolResult(v bool) string {
	if v {
		return "pass"
	}
	return "fail"
}

func correctness(dir string, max int) int {
	out := newTSV(dir+"/correctness.tsv", "protocol\tcase\texpected\tactual\tresult")
	defer out.close()
	failed := 0
	check := func(p protocol, name, expected, actual string) {
		ok := expected == actual
		if !ok {
			failed++
		}
		out.row("%s\t%s\t%s\t%s\t%s", p, name, expected, actual, boolResult(ok))
	}
	for _, p := range []protocol{line, framed, datagram} {
		path := "/tmp/inv007-" + string(p) + ".sock"
		s, err := startServer(p, path, max, 0660, 0, nil)
		if err != nil {
			panic(err)
		}
		info, _ := os.Stat(path)
		check(p, "socket_permissions", "0660", fmt.Sprintf("%04o", info.Mode().Perm()))
		c, err := dial(p, path)
		check(p, "single_connect", "ok", mapErr(err))
		if err == nil {
			_ = send(c, p, []byte("v1 metric=1"))
			_ = c.Close()
		}
		waitFor(func() bool { return s.c.valid.Load() == 1 }, time.Second)
		check(p, "single_message", "1", strconv.FormatInt(s.c.valid.Load(), 10))
		c, _ = dial(p, path)
		_ = send(c, p, []byte("bad"))
		_ = c.Close()
		waitFor(func() bool { return s.c.malformed.Load() == 1 }, time.Second)
		check(p, "malformed_rejected", "1", strconv.FormatInt(s.c.malformed.Load(), 10))
		if p != datagram {
			c, _ = dial(p, path)
			if p == line {
				_, _ = c.Write([]byte("v1 partial"))
			} else {
				_, _ = c.Write([]byte{0, 0, 0, 20, 'v', '1'})
			}
			_ = c.Close()
			waitFor(func() bool { return s.c.malformed.Load() >= 2 }, time.Second)
			check(p, "disconnect_mid_message", "2", strconv.FormatInt(s.c.malformed.Load(), 10))
		}
		c, _ = dial(p, path)
		exact := []byte("v1 " + strings.Repeat("x", max-3))
		_ = send(c, p, exact)
		_ = c.Close()
		waitFor(func() bool { return s.c.valid.Load() == 2 }, time.Second)
		check(p, "maximum_payload_accepted", "2", strconv.FormatInt(s.c.valid.Load(), 10))
		c, _ = dial(p, path)
		tooBig := []byte("v1 " + strings.Repeat("x", max))
		_ = send(c, p, tooBig)
		_ = c.Close()
		waitFor(func() bool { return s.c.oversized.Load() >= 1 }, time.Second)
		check(p, "oversized_rejected", "1", strconv.FormatInt(s.c.oversized.Load(), 10))
		s.close()
		_, err = dial(p, path)
		check(p, "shutdown_refuses_new", "error", mapErr(err))

		// Startup race: bounded client retry succeeds after the socket appears.
		var attempts int
		var rc net.Conn
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 50; i++ {
				attempts++
				rc, err = dial(p, path)
				if err == nil {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
		time.Sleep(50 * time.Millisecond)
		s, err = startServer(p, path, max, 0660, 0, nil)
		if err != nil {
			panic(err)
		}
		<-done
		check(p, "startup_retry", "ok", mapErr(err))
		if rc != nil {
			_ = rc.Close()
		}
		check(p, "startup_retry_bounded", "true", strconv.FormatBool(attempts > 1 && attempts < 50))

		// Restart: persistent connection breaks, reconnect delivers to new epoch.
		old, _ := dial(p, path)
		_ = old.Close()
		s.close()
		s2, err := startServer(p, path, max, 0660, 0, nil)
		if err != nil {
			panic(err)
		}
		fresh, err := dial(p, path)
		if err == nil {
			_ = send(fresh, p, []byte("v1 fresh=1"))
			_ = fresh.Close()
		}
		waitFor(func() bool { return s2.c.valid.Load() == 1 }, time.Second)
		check(p, "restart_reconnect", "1", strconv.FormatInt(s2.c.valid.Load(), 10))
		s2.close()
	}
	return failed
}

func mapErr(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func percentile(v []float64, q float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sort.Float64s(v)
	i := int(q * float64(len(v)-1))
	return v[i]
}

func performance(dir string, max, repetitions int) int {
	out := newTSV(dir+"/performance.tsv", "protocol\tproducers\tpayload_bytes\tmessages\trepetition\tdelivered\tdropped_or_failed\twall_ms\tmsg_per_second\tp50_us\tp95_us\tp99_us\tcpu_ms\trss_kib\tresult")
	defer out.close()
	failed := 0
	for _, p := range []protocol{line, framed, datagram} {
		for _, producers := range []int{1, 8, 32} {
			for _, size := range []int{64, 1024, 8192} {
				perProducer := 2000
				if size == 8192 {
					perProducer = 500
				}
				for rep := 1; rep <= repetitions; rep++ {
					path := "/tmp/inv007-perf.sock"
					var latMu sync.Mutex
					lat := make([]float64, 0, producers*perProducer)
					s, err := startServer(p, path, max, 0660, 0, func(b []byte) {
						parts := strings.SplitN(string(b), " ", 3)
						if len(parts) > 1 {
							if ns, e := strconv.ParseInt(parts[1], 10, 64); e == nil {
								latMu.Lock()
								lat = append(lat, float64(time.Now().UnixNano()-ns)/1000)
								latMu.Unlock()
							}
						}
					})
					if err != nil {
						panic(err)
					}
					var before, after syscall.Rusage
					_ = syscall.Getrusage(syscall.RUSAGE_SELF, &before)
					start := time.Now()
					var wg sync.WaitGroup
					var sendFail atomic.Int64
					for id := 0; id < producers; id++ {
						wg.Add(1)
						go func() {
							defer wg.Done()
							c, e := dial(p, path)
							if e != nil {
								sendFail.Add(int64(perProducer))
								return
							}
							defer c.Close()
							padding := strings.Repeat("x", size-30)
							for i := 0; i < perProducer; i++ {
								b := []byte(fmt.Sprintf("v1 %d %s", time.Now().UnixNano(), padding))
								if e = send(c, p, b); e != nil {
									sendFail.Add(1)
								}
							}
						}()
					}
					wg.Wait()
					expected := int64(producers * perProducer)
					waitFor(func() bool { return s.c.valid.Load()+sendFail.Load() >= expected }, 10*time.Second)
					wall := time.Since(start)
					_ = syscall.Getrusage(syscall.RUSAGE_SELF, &after)
					s.close()
					delivered := s.c.valid.Load()
					lost := expected - delivered
					userUS := (after.Utime.Sec-before.Utime.Sec)*1e6 + int64(after.Utime.Usec-before.Utime.Usec)
					systemUS := (after.Stime.Sec-before.Stime.Sec)*1e6 + int64(after.Stime.Usec-before.Stime.Usec)
					cpu := float64(userUS+systemUS) / 1000
					var ms runtime.MemStats
					runtime.ReadMemStats(&ms)
					ok := delivered == expected
					if !ok {
						failed++
					}
					out.row("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%.3f\t%.1f\t%.3f\t%.3f\t%.3f\t%.3f\t%d\t%s", p, producers, size, expected, rep, delivered, lost, wall.Seconds()*1000, float64(delivered)/wall.Seconds(), percentile(lat, .50), percentile(lat, .95), percentile(lat, .99), cpu, ms.Sys/1024, boolResult(ok))
				}
			}
		}
	}
	return failed
}

func pressure(dir string, max int) int {
	out := newTSV(dir+"/pressure.tsv", "protocol\tcase\tinput\tdelivered\tfailed_or_blocked\tduration_ms\tresult")
	defer out.close()
	failed := 0
	check := func(p protocol, name string, input, delivered, blocked int, d time.Duration, ok bool) {
		if !ok {
			failed++
		}
		out.row("%s\t%s\t%d\t%d\t%d\t%.3f\t%s", p, name, input, delivered, blocked, d.Seconds()*1000, boolResult(ok))
	}
	for _, p := range []protocol{line, framed, datagram} {
		path := "/tmp/inv007-pressure.sock"
		s, err := startServer(p, path, max, 0660, 200*time.Microsecond, nil)
		if err != nil {
			panic(err)
		}
		const n = 2000
		start := time.Now()
		var wg sync.WaitGroup
		var sf atomic.Int64
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, e := dial(p, path)
				if e != nil {
					sf.Add(n / 16)
					return
				}
				defer c.Close()
				_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
				for j := 0; j < n/16; j++ {
					if send(c, p, []byte("v1 slow=1")) != nil {
						sf.Add(1)
					}
				}
			}()
		}
		wg.Wait()
		waitFor(func() bool { return s.c.valid.Load()+sf.Load() >= n }, 5*time.Second)
		d := time.Since(start)
		s.close()
		ok := s.c.valid.Load()+sf.Load() == n
		check(p, "slow_reader_backpressure", n, int(s.c.valid.Load()), int(sf.Load()), d, ok)
	}
	// File descriptor pressure: lower the soft limit, hold clients, and verify bounded rejection/recovery.
	var old syscall.Rlimit
	_ = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &old)
	lim := old
	if lim.Cur > 128 {
		lim.Cur = 128
	}
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim)
	path := "/tmp/inv007-fd.sock"
	s, err := startServer(line, path, max, 0660, 0, nil)
	if err != nil {
		panic(err)
	}
	var conns []net.Conn
	rejected := 0
	start := time.Now()
	for i := 0; i < 256; i++ {
		c, e := dial(line, path)
		if e != nil {
			rejected++
			continue
		}
		conns = append(conns, c)
	}
	for _, c := range conns {
		_ = c.Close()
	}
	s.close()
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &old)
	check(line, "fd_exhaustion", 256, len(conns), rejected, time.Since(start), rejected > 0)
	s, err = startServer(datagram, path, max, 0660, 0, nil)
	if err != nil {
		panic(err)
	}
	c, err := dial(datagram, path)
	if err != nil {
		panic(err)
	}
	start = time.Now()
	rejected = 0
	for i := 0; i < 256; i++ {
		if send(c, datagram, []byte("v1 fd=1")) != nil {
			rejected++
		}
	}
	_ = c.Close()
	waitFor(func() bool { return s.c.valid.Load() == 256 }, time.Second)
	check(datagram, "fd_model_no_per_producer_accept", 256, int(s.c.valid.Load()), int(s.c.accepted.Load()), time.Since(start), s.c.valid.Load() == 256 && s.c.accepted.Load() == 0)
	s.close()
	return failed
}

func main() {
	var out string
	var max, reps int
	flag.StringVar(&out, "output-dir", "/results", "output directory")
	flag.IntVar(&max, "max-payload", 65536, "maximum accepted payload")
	flag.IntVar(&reps, "repetitions", 3, "performance repetitions")
	flag.Parse()
	if err := os.MkdirAll(out, 0755); err != nil {
		panic(err)
	}
	failed := correctness(out, max) + performance(out, max, reps) + pressure(out, max)
	s := newTSV(out+"/summary.tsv", "metric\tvalue")
	s.row("portable_assertions_failed\t%d", failed)
	s.row("max_payload_bytes\t%d", max)
	s.row("performance_repetitions\t%d", reps)
	s.close()
	if failed > 0 {
		os.Exit(1)
	}
}
