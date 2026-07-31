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
	line                   protocol = "stream-line"
	framed                 protocol = "stream-framed"
	datagram               protocol = "datagram-line"
	defaultConnectionLimit          = 64
)

type counters struct {
	accepted, valid, malformed, oversized, connectionRejected, active, maxBuffered atomic.Int64
}

type server struct {
	proto     protocol
	path      string
	max       int
	delay     time.Duration
	connLimit int
	ack       bool
	ln        net.Listener
	pc        net.PacketConn
	wg        sync.WaitGroup
	connMu    sync.Mutex
	conns     map[net.Conn]struct{}
	c         counters
	onMessage func([]byte) bool
}

func startServer(p protocol, path string, max int, mode os.FileMode, delay time.Duration, connLimit int, ack bool, onMessage func([]byte) bool) (*server, error) {
	_ = os.Remove(path)
	s := &server{proto: p, path: path, max: max, delay: delay, connLimit: connLimit, ack: ack, conns: make(map[net.Conn]struct{}), onMessage: onMessage}
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
		s.connMu.Lock()
		if len(s.conns) >= s.connLimit {
			s.c.connectionRejected.Add(1)
			s.connMu.Unlock()
			_ = c.Close()
			continue
		}
		s.conns[c] = struct{}{}
		s.c.active.Add(1)
		s.c.accepted.Add(1)
		s.connMu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { s.connMu.Lock(); delete(s.conns, c); s.c.active.Add(-1); s.connMu.Unlock(); _ = c.Close() }()
			s.readStream(c)
		}()
	}
}

func (s *server) readStream(c net.Conn) {
	if s.proto == line {
		r := bufio.NewReaderSize(c, s.max+1)
		for {
			b, err := r.ReadSlice('\n')
			s.observeBuffered(len(b))
			if errors.Is(err, bufio.ErrBufferFull) {
				s.c.oversized.Add(1)
				s.drainLine(r)
				s.respond(c, false, "oversized", b)
				continue
			}
			if err != nil {
				if len(b) > 0 {
					s.c.malformed.Add(1)
					s.respond(c, false, "partial", b)
				}
				return
			}
			if len(b)-1 > s.max {
				s.c.oversized.Add(1)
				s.respond(c, false, "oversized", b)
				continue
			}
			payload := b[:len(b)-1]
			s.respond(c, s.accept(payload), "invalid", payload)
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
			s.respond(c, false, "size", nil)
			return
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(c, b); err != nil {
			s.c.malformed.Add(1)
			s.respond(c, false, "partial", b)
			return
		}
		s.observeBuffered(len(b))
		s.respond(c, s.accept(b), "invalid", b)
	}
}

func (s *server) observeBuffered(n int) {
	for {
		old := s.c.maxBuffered.Load()
		if int64(n) <= old || s.c.maxBuffered.CompareAndSwap(old, int64(n)) {
			return
		}
	}
}
func (s *server) drainLine(r *bufio.Reader) {
	for {
		b, err := r.ReadSlice('\n')
		s.observeBuffered(len(b))
		if !errors.Is(err, bufio.ErrBufferFull) {
			return
		}
	}
}
func messageID(b []byte) string {
	for _, field := range strings.Fields(string(b)) {
		if strings.HasPrefix(field, "id=") {
			return strings.TrimPrefix(field, "id=")
		}
	}
	return "-"
}

func (s *server) respond(c net.Conn, ok bool, reason string, b []byte) {
	if !s.ack {
		return
	}
	_ = c.SetWriteDeadline(time.Now().Add(time.Second))
	fields := strings.Fields(string(b))
	command := ""
	publicationID := messageID(b)
	if len(fields) >= 2 {
		command = fields[1]
	}
	if strings.HasPrefix(command, "snapshot_") && len(fields) >= 3 {
		publicationID = fields[2]
	}
	if ok {
		switch command {
		case "snapshot_begin", "snapshot_part":
			_, _ = io.WriteString(c, "FRAME_ACCEPTED "+messageID(b)+"\n")
		case "snapshot_commit":
			_, _ = io.WriteString(c, "ACK "+publicationID+"\n")
		default:
			_, _ = io.WriteString(c, "ACK "+messageID(b)+"\n")
		}
	} else {
		_, _ = io.WriteString(c, "NACK "+publicationID+" "+reason+"\n")
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
		s.observeBuffered(n)
		if n > s.max+1 {
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
func (s *server) accept(b []byte) bool {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	if !strings.HasPrefix(string(b), "v1 ") {
		s.c.malformed.Add(1)
		return false
	}
	s.c.valid.Add(1)
	if s.onMessage != nil && !s.onMessage(b) {
		s.c.valid.Add(-1)
		s.c.malformed.Add(1)
		return false
	}
	return true
}
func (s *server) close() {
	if s.ln != nil {
		_ = s.ln.Close()
	}
	if s.pc != nil {
		_ = s.pc.Close()
	}
	s.connMu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.connMu.Unlock()
	s.wg.Wait()
	_ = os.Remove(s.path)
}

func dial(p protocol, path string) (net.Conn, error) {
	network := "unix"
	if p == datagram {
		network = "unixgram"
	}
	return net.DialTimeout(network, path, 500*time.Millisecond)
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
func sendAck(c net.Conn, p protocol, b []byte) (string, error) {
	if err := send(c, p, b); err != nil {
		return "", err
	}
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	r, err := bufio.NewReader(c).ReadString('\n')
	return strings.TrimSpace(r), err
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
func result(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}
func mapErr(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
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
		out.row("%s\t%s\t%s\t%s\t%s", p, name, expected, actual, result(ok))
	}
	for _, p := range []protocol{line, framed, datagram} {
		path := "/tmp/inv007-" + string(p) + ".sock"
		s, err := startServer(p, path, max, 0660, 0, defaultConnectionLimit, false, nil)
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
		_ = send(c, p, []byte("v1 "+strings.Repeat("x", max-3)))
		_ = c.Close()
		waitFor(func() bool { return s.c.valid.Load() == 2 }, time.Second)
		check(p, "maximum_payload_accepted", "2", strconv.FormatInt(s.c.valid.Load(), 10))
		c, _ = dial(p, path)
		oversizedBytes := max * 2
		if p == datagram {
			oversizedBytes = max + 1024
		}
		_ = send(c, p, []byte("v1 "+strings.Repeat("x", oversizedBytes)))
		_ = c.Close()
		waitFor(func() bool { return s.c.oversized.Load() >= 1 }, time.Second)
		check(p, "substantially_oversized_rejected", "1", strconv.FormatInt(s.c.oversized.Load(), 10))
		if p == line {
			check(p, "bounded_parser_buffer", "true", strconv.FormatBool(s.c.maxBuffered.Load() <= int64(max+1)))
		}

		if p != datagram {
			active, _ := dial(p, path)
			if p == line {
				_, _ = active.Write([]byte("v1 " + strings.Repeat("z", max-3)))
				time.Sleep(10 * time.Millisecond)
			}
			start := time.Now()
			done := make(chan struct{})
			go func() { s.close(); close(done) }()
			select {
			case <-done:
				check(p, "shutdown_active_client_bounded", "true", strconv.FormatBool(time.Since(start) < time.Second))
			case <-time.After(time.Second):
				check(p, "shutdown_active_client_bounded", "true", "false")
			}
			_ = active.SetReadDeadline(time.Now().Add(time.Second))
			one := make([]byte, 1)
			_, readErr := active.Read(one)
			check(p, "shutdown_breaks_old_connection", "error", mapErr(readErr))
			_ = active.Close()
		} else {
			s.close()
		}
		_, err = dial(p, path)
		check(p, "shutdown_refuses_new", "error", mapErr(err))

		type retryResult struct {
			attempts int
			c        net.Conn
			err      error
		}
		retryCh := make(chan retryResult, 1)
		go func() {
			rr := retryResult{}
			for i := 0; i < 50; i++ {
				rr.attempts++
				rr.c, rr.err = dial(p, path)
				if rr.err == nil {
					retryCh <- rr
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			retryCh <- rr
		}()
		time.Sleep(50 * time.Millisecond)
		s, err = startServer(p, path, max, 0660, 0, defaultConnectionLimit, false, nil)
		if err != nil {
			panic(err)
		}
		rr := <-retryCh
		check(p, "startup_retry", "ok", mapErr(rr.err))
		check(p, "startup_retry_bounded", "true", strconv.FormatBool(rr.attempts > 1 && rr.attempts < 50))
		if rr.c != nil {
			_ = rr.c.Close()
		}

		old, _ := dial(p, path)
		restartDone := make(chan struct{})
		go func() { s.close(); close(restartDone) }()
		if p != datagram {
			_ = old.SetReadDeadline(time.Now().Add(time.Second))
			_, readErr := old.Read(make([]byte, 1))
			check(p, "restart_old_connection_error", "error", mapErr(readErr))
		}
		<-restartDone
		_ = old.Close()
		s2, err := startServer(p, path, max, 0660, 0, defaultConnectionLimit, false, nil)
		if err != nil {
			panic(err)
		}
		fresh, err := dial(p, path)
		if err == nil {
			_ = send(fresh, p, []byte("v1 fresh=1"))
			_ = fresh.Close()
		}
		waitFor(func() bool { return s2.c.valid.Load() == 1 }, time.Second)
		check(p, "restart_reconnect_new_epoch", "1", strconv.FormatInt(s2.c.valid.Load(), 10))
		s2.close()
	}
	for _, p := range []protocol{line, framed} {
		path := "/tmp/inv007-ack.sock"
		s, err := startServer(p, path, max, 0660, 0, defaultConnectionLimit, true, nil)
		if err != nil {
			panic(err)
		}
		c, _ := dial(p, path)
		ack, err := sendAck(c, p, []byte("v1 id=42 metric=1"))
		check(p, "ack_after_accept", "ACK 42", ack)
		check(p, "ack_read", "ok", mapErr(err))
		_ = c.Close()
		c, _ = dial(p, path)
		nack, _ := sendAck(c, p, []byte("bad id=43"))
		check(p, "nack_invalid", "NACK 43 invalid", nack)
		_ = c.Close()
		s.close()
	}
	return failed
}

func percentile(v []float64, q float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sort.Float64s(v)
	return v[int(q*float64(len(v)-1))]
}
func rssKiB() int64 {
	b, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return -1
	}
	f := strings.Fields(string(b))
	if len(f) < 2 {
		return -1
	}
	pages, err := strconv.ParseInt(f[1], 10, 64)
	if err != nil {
		return -1
	}
	return pages * int64(os.Getpagesize()) / 1024
}

func memoryEvidence(dir string, max int) int {
	out := newTSV(dir+"/memory.tsv", "case\tmessage_bytes\tmessages\tmax_parser_buffer_bytes\trss_before_kib\trss_after_kib\trss_delta_kib\tallowed_delta_kib\tresult")
	defer out.close()
	runtime.GC()
	before := rssKiB()
	path := "/tmp/inv007-memory.sock"
	s, err := startServer(line, path, max, 0660, 0, defaultConnectionLimit, false, nil)
	if err != nil {
		panic(err)
	}
	payload := []byte("v1 " + strings.Repeat("m", max*2))
	wire := append(append([]byte{}, payload...), '\n')
	c, err := dial(line, path)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 16; i++ {
		if _, err = c.Write(wire); err != nil {
			break
		}
	}
	_ = c.Close()
	waitFor(func() bool { return s.c.oversized.Load() == 16 }, 3*time.Second)
	s.close()
	runtime.GC()
	after := rssKiB()
	delta := after - before
	allowed := int64(16 * 1024)
	ok := s.c.oversized.Load() == 16 && s.c.maxBuffered.Load() <= int64(max+1) && delta <= allowed
	out.row("bounded_line_reader\t%d\t16\t%d\t%d\t%d\t%d\t%d\t%s", len(payload), s.c.maxBuffered.Load(), before, after, delta, allowed, result(ok))
	if !ok {
		return 1
	}
	return 0
}

func performance(dir string, max, repetitions int) int {
	out := newTSV(dir+"/performance.tsv", "protocol\tproducers\tpayload_bytes\tmessages\trepetition\tdelivered\tdropped_or_failed\twall_ms\tmsg_per_second\tp50_us\tp95_us\tp99_us\tcpu_ms\tgo_runtime_sys_kib\trss_kib\tresult")
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
					s, err := startServer(p, path, max, 0660, 0, defaultConnectionLimit, false, func(b []byte) bool {
						parts := strings.SplitN(string(b), " ", 3)
						if len(parts) > 1 {
							if ns, e := strconv.ParseInt(parts[1], 10, 64); e == nil {
								latMu.Lock()
								lat = append(lat, float64(time.Now().UnixNano()-ns)/1000)
								latMu.Unlock()
							}
						}
						return true
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
								if e = send(c, p, []byte(fmt.Sprintf("v1 %d %s", time.Now().UnixNano(), padding))); e != nil {
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
					userUS := (after.Utime.Sec-before.Utime.Sec)*1e6 + int64(after.Utime.Usec-before.Utime.Usec)
					systemUS := (after.Stime.Sec-before.Stime.Sec)*1e6 + int64(after.Stime.Usec-before.Stime.Usec)
					var ms runtime.MemStats
					runtime.ReadMemStats(&ms)
					ok := delivered == expected
					if !ok {
						failed++
					}
					out.row("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%.3f\t%.1f\t%.3f\t%.3f\t%.3f\t%.3f\t%d\t%d\t%s", p, producers, size, expected, rep, delivered, expected-delivered, wall.Seconds()*1000, float64(delivered)/wall.Seconds(), percentile(lat, .5), percentile(lat, .95), percentile(lat, .99), float64(userUS+systemUS)/1000, ms.Sys/1024, rssKiB(), result(ok))
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
		out.row("%s\t%s\t%d\t%d\t%d\t%.3f\t%s", p, name, input, delivered, blocked, d.Seconds()*1000, result(ok))
	}
	for _, p := range []protocol{line, framed, datagram} {
		path := "/tmp/inv007-pressure.sock"
		s, err := startServer(p, path, max, 0660, 200*time.Microsecond, defaultConnectionLimit, false, nil)
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
		check(p, "slow_reader_backpressure", n, int(s.c.valid.Load()), int(sf.Load()), d, s.c.valid.Load()+sf.Load() == n)
	}
	path := "/tmp/inv007-limit.sock"
	s, err := startServer(line, path, max, 0660, 0, 8, false, nil)
	if err != nil {
		panic(err)
	}
	var conns []net.Conn
	for i := 0; i < 32; i++ {
		c, e := dial(line, path)
		if e == nil {
			conns = append(conns, c)
		}
	}
	waitFor(func() bool { return s.c.connectionRejected.Load() > 0 }, time.Second)
	rejected := int(s.c.connectionRejected.Load())
	for _, c := range conns {
		_ = c.Close()
	}
	waitFor(func() bool { return s.c.active.Load() == 0 }, time.Second)
	recovery, recErr := dial(line, path)
	if recErr == nil {
		_ = send(recovery, line, []byte("v1 recovered=1"))
		_ = recovery.Close()
	}
	waitFor(func() bool { return s.c.valid.Load() == 1 }, time.Second)
	check(line, "application_connection_limit_recovery", 32, 1, rejected, 0, rejected > 0 && s.c.valid.Load() == 1)
	s.close()

	var old syscall.Rlimit
	_ = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &old)
	lim := old
	if lim.Cur > 128 {
		lim.Cur = 128
	}
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim)
	s, err = startServer(line, path, max, 0660, 0, 256, false, nil)
	if err != nil {
		panic(err)
	}
	conns = nil
	rejected = 0
	start := time.Now()
	for i := 0; i < 256; i++ {
		c, e := dial(line, path)
		if e != nil {
			rejected++
		} else {
			conns = append(conns, c)
		}
	}
	for _, c := range conns {
		_ = c.Close()
	}
	s.close()
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &old)
	check(line, "system_fd_exhaustion", 256, len(conns), rejected, time.Since(start), rejected > 0)
	s, err = startServer(datagram, path, max, 0660, 0, defaultConnectionLimit, false, nil)
	if err != nil {
		panic(err)
	}
	c, err := dial(datagram, path)
	if err != nil {
		panic(err)
	}
	start = time.Now()
	for i := 0; i < 256; i++ {
		_ = send(c, datagram, []byte("v1 fd=1"))
	}
	_ = c.Close()
	waitFor(func() bool { return s.c.valid.Load() == 256 }, time.Second)
	check(datagram, "fd_model_no_per_producer_accept", 256, int(s.c.valid.Load()), int(s.c.accepted.Load()), time.Since(start), s.c.valid.Load() == 256 && s.c.accepted.Load() == 0)
	s.close()
	return failed
}

func snapshot(dir string, max int) int {
	out := newTSV(dir+"/snapshot.tsv", "case\tparts\tbytes\tcommitted_version\tframe_response\tcommit_response\tresult")
	defer out.close()
	failed := 0
	path := "/tmp/inv007-snapshot.sock"
	var mu sync.Mutex
	parts := map[int]string{}
	committed := ""
	pendingID := ""
	expectedParts := 0
	s, err := startServer(line, path, max, 0660, 0, defaultConnectionLimit, true, func(b []byte) bool {
		f := strings.Fields(string(b))
		if len(f) < 3 {
			return false
		}
		mu.Lock()
		defer mu.Unlock()
		switch f[1] {
		case "snapshot_begin":
			if len(f) < 4 {
				return false
			}
			expectedParts, _ = strconv.Atoi(f[3])
			if expectedParts <= 0 {
				return false
			}
			pendingID = f[2]
			parts = map[int]string{}
			return true
		case "snapshot_part":
			if len(f) >= 5 && f[2] == pendingID {
				i, _ := strconv.Atoi(f[3])
				if i < 0 || i >= expectedParts {
					return false
				}
				parts[i] = f[4]
				return true
			}
			return false
		case "snapshot_commit":
			if f[2] != pendingID || len(parts) != expectedParts {
				return false
			}
			committed = f[2]
			return true
		}
		return false
	})
	if err != nil {
		panic(err)
	}
	c, _ := dial(line, path)
	msgs := []string{"v1 snapshot_begin snap-1 3 id=begin-1", "v1 snapshot_part snap-1 0 " + strings.Repeat("a", 4000) + " id=part-0", "v1 snapshot_part snap-1 1 " + strings.Repeat("b", 4000) + " id=part-1", "v1 snapshot_part snap-1 2 " + strings.Repeat("c", 4000) + " id=part-2", "v1 snapshot_commit snap-1 id=commit-1"}
	responses := make([]string, 0, len(msgs))
	for index, m := range msgs {
		ack, e := sendAck(c, line, []byte(m))
		responses = append(responses, ack)
		expectedResponses := []string{"FRAME_ACCEPTED begin-1", "FRAME_ACCEPTED part-0", "FRAME_ACCEPTED part-1", "FRAME_ACCEPTED part-2", "ACK snap-1"}
		if e != nil || ack != expectedResponses[index] {
			failed++
		}
	}
	_ = c.Close()
	mu.Lock()
	ok := committed == "snap-1" && len(parts) == 3
	mu.Unlock()
	if !ok {
		failed++
	}
	out.row("multipart_atomic_snapshot\t3\t12000\t%s\t%s\t%s\t%s", committed, responses[1], responses[4], result(ok))
	c, _ = dial(line, path)
	beginResponse, _ := sendAck(c, line, []byte("v1 snapshot_begin snap-2 2 id=begin-2"))
	partResponse, _ := sendAck(c, line, []byte("v1 snapshot_part snap-2 0 partial id=part-2-0"))
	incompleteNack, _ := sendAck(c, line, []byte("v1 snapshot_commit snap-2 id=commit-2"))
	_ = c.Close()
	mu.Lock()
	incompleteRetained := committed == "snap-1" && beginResponse == "FRAME_ACCEPTED begin-2" && partResponse == "FRAME_ACCEPTED part-2-0" && strings.HasPrefix(incompleteNack, "NACK snap-2")
	mu.Unlock()
	if !incompleteRetained {
		failed++
	}
	out.row("incomplete_snapshot_not_committed\t1\t7\t%s\t%s\t%s\t%s", committed, partResponse, incompleteNack, result(incompleteRetained))
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
	failed := correctness(out, max) + memoryEvidence(out, max) + performance(out, max, reps) + pressure(out, max) + snapshot(out, max)
	s := newTSV(out+"/summary.tsv", "metric\tvalue")
	s.row("portable_assertions_failed\t%d", failed)
	s.row("max_payload_bytes\t%d", max)
	s.row("performance_repetitions\t%d", reps)
	s.close()
	if failed > 0 {
		os.Exit(1)
	}
}
