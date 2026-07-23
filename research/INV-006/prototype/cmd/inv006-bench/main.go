package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
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
)

type config struct {
	strategy, root, output           string
	interval, duration, initialPause time.Duration
	updates, fileBytes               int
}

type snapshot struct {
	seq int64
	ts  int64
	sum string
}

type observer struct {
	cfg        config
	path       string
	stop       chan struct{}
	done       chan struct{}
	seen       chan snapshot
	invalid    atomic.Int64
	overflow   atomic.Int64
	dropEvents atomic.Bool
	dropped    atomic.Int64
	reads      atomic.Int64
	parseError atomic.Int64
	lastMu     sync.Mutex
	last       snapshot
}

func readSnapshot(path string) (snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return snapshot{}, err
	}
	parts := strings.SplitN(string(b), "\n", 3)
	if len(parts) < 2 {
		return snapshot{}, errors.New("invalid record")
	}
	seq, e1 := strconv.ParseInt(strings.TrimPrefix(parts[0], "seq="), 10, 64)
	ts, e2 := strconv.ParseInt(strings.TrimPrefix(parts[1], "ts_ns="), 10, 64)
	if e1 != nil || e2 != nil {
		return snapshot{}, errors.New("invalid record")
	}
	h := sha256.Sum256(b)
	return snapshot{seq: seq, ts: ts, sum: hex.EncodeToString(h[:])}, nil
}

func (o *observer) reconcile() {
	s, err := readSnapshot(o.path)
	if err != nil {
		if !os.IsNotExist(err) {
			o.parseError.Add(1)
		}
		return
	}
	o.reads.Add(1)
	o.lastMu.Lock()
	changed := s.sum != o.last.sum
	if changed {
		o.last = s
	}
	o.lastMu.Unlock()
	if changed {
		select {
		case o.seen <- s:
		default:
		}
	}
}

func (o *observer) run() {
	defer close(o.done)
	o.reconcile()
	if o.cfg.strategy == "poll" {
		t := time.NewTicker(o.cfg.interval)
		defer t.Stop()
		for {
			select {
			case <-o.stop:
				return
			case <-t.C:
				o.reconcile()
			}
		}
	}

	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		panic(err)
	}
	defer syscall.Close(fd)
	parent := filepath.Dir(o.cfg.root)
	add := func() {
		_, _ = syscall.InotifyAddWatch(fd, parent, syscall.IN_CREATE|syscall.IN_MOVED_TO)
		_, _ = syscall.InotifyAddWatch(fd, o.cfg.root, syscall.IN_CREATE|syscall.IN_CLOSE_WRITE|syscall.IN_MOVED_TO|syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF)
	}
	add()
	if o.cfg.initialPause > 0 {
		time.Sleep(o.cfg.initialPause)
	}
	nextReconcile := time.Now().Add(o.cfg.interval)
	buf := make([]byte, 64*1024)
	for {
		select {
		case <-o.stop:
			return
		default:
		}
		var readfds syscall.FdSet
		readfds.Bits[fd/64] |= 1 << (uint(fd) % 64)
		timeout := syscall.Timeval{Usec: 50_000}
		ready, _ := syscall.Select(fd+1, &readfds, nil, nil, &timeout)
		if o.cfg.strategy == "hybrid" && !time.Now().Before(nextReconcile) {
			add()
			o.reconcile()
			nextReconcile = time.Now().Add(o.cfg.interval)
		}
		if ready > 0 {
			n, e := syscall.Read(fd, buf)
			if e == syscall.EAGAIN || n == 0 {
				continue
			}
			if e != nil {
				time.Sleep(time.Millisecond)
				continue
			}
			for off := 0; off+syscall.SizeofInotifyEvent <= n; {
				ev := (*syscall.InotifyEvent)(unsafePointer(&buf[off]))
				if ev.Mask&syscall.IN_Q_OVERFLOW != 0 {
					o.overflow.Add(1)
				}
				if ev.Mask&(syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF|syscall.IN_IGNORED) != 0 {
					o.invalid.Add(1)
				}
				off += syscall.SizeofInotifyEvent + int(ev.Len)
			}
			if o.dropEvents.Load() {
				o.dropped.Add(1)
				continue
			}
			add()
			o.reconcile()
		}
	}
}

// Kept in one place so the benchmark has no external module dependency.
func unsafePointer(p *byte) unsafe.Pointer { return unsafe.Pointer(p) }

func startObserver(c config) *observer {
	o := &observer{cfg: c, path: filepath.Join(c.root, "metrics"), stop: make(chan struct{}), done: make(chan struct{}), seen: make(chan snapshot, 65536)}
	go o.run()
	return o
}

func (o *observer) close() {
	close(o.stop)
	<-o.done
}

func atomicWrite(path string, seq int64, size int, valid bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	var body string
	if valid {
		head := fmt.Sprintf("seq=%d\nts_ns=%d\n", seq, time.Now().UnixNano())
		if size < len(head) {
			size = len(head)
		}
		body = head + strings.Repeat("x", size-len(head))
	} else {
		body = "invalid\n"
	}
	if err := os.WriteFile(tmp, []byte(body), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func waitSeq(o *observer, seq int64, timeout time.Duration) (time.Duration, bool) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case s := <-o.seen:
			if s.seq == seq {
				return time.Duration(time.Now().UnixNano() - s.ts), true
			}
		case <-deadline.C:
			return 0, false
		}
	}
}

func correctness(c config) {
	_ = os.RemoveAll(c.root)
	_ = os.MkdirAll(c.root, 0755)
	path := filepath.Join(c.root, "metrics")
	out, err := os.Create(c.output)
	if err != nil {
		panic(err)
	}
	defer out.Close()
	fmt.Fprintln(out, "case\texpected\tactual\tresult")
	row := func(name, expected, actual string, pass bool) {
		result := "fail"
		if pass {
			result = "pass"
		}
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", name, expected, actual, result)
	}

	_ = atomicWrite(path, 1, 128, true)
	o := startObserver(c)
	_, ok := waitSeq(o, 1, 2*time.Second)
	row("initial_file", "observed", boolWord(ok), ok)
	_ = os.Remove(path)
	time.Sleep(2*c.interval + 20*time.Millisecond)
	_ = atomicWrite(path, 2, 128, true)
	_, ok = waitSeq(o, 2, 2*time.Second)
	row("initially_absent_then_created", "observed", boolWord(ok), ok)
	_ = atomicWrite(path, 3, 128, true)
	_, ok = waitSeq(o, 3, 2*time.Second)
	row("atomic_rename", "observed", boolWord(ok), ok)
	for i := int64(4); i <= 103; i++ {
		_ = atomicWrite(path, i, 128, true)
	}
	_, ok = waitSeq(o, 103, 3*time.Second)
	row("repeated_replacement_100", "final_observed", boolWord(ok), ok)
	_ = os.WriteFile(path+".tmp", []byte("partial"), 0644)
	time.Sleep(2*c.interval + 20*time.Millisecond)
	o.lastMu.Lock()
	last := o.last.seq
	o.lastMu.Unlock()
	row("writer_crash_before_rename", "last_valid_103", fmt.Sprint(last), last == 103)
	_ = atomicWrite(path, 104, 128, false)
	time.Sleep(2*c.interval + 20*time.Millisecond)
	o.lastMu.Lock()
	last = o.last.seq
	o.lastMu.Unlock()
	row("invalid_update_retains_last_valid", "103", fmt.Sprint(last), last == 103)
	_ = os.Remove(path)
	time.Sleep(2*c.interval + 20*time.Millisecond)
	o.lastMu.Lock()
	last = o.last.seq
	o.lastMu.Unlock()
	row("file_deletion_retains_last_valid", "103", fmt.Sprint(last), last == 103)
	_ = os.RemoveAll(c.root)
	_ = os.MkdirAll(c.root, 0755)
	_ = atomicWrite(path, 105, 128, true)
	_, ok = waitSeq(o, 105, 4*time.Second)
	row("directory_recreation", "observed", boolWord(ok), ok)
	o.close()

	o2 := startObserver(c)
	_, ok = waitSeq(o2, 105, 2*time.Second)
	row("metricshell_restart", "initial_state_observed", boolWord(ok), ok)
	o2.close()
}

func performance(c config) {
	_ = os.RemoveAll(c.root)
	_ = os.MkdirAll(c.root, 0755)
	path := filepath.Join(c.root, "metrics")
	o := startObserver(c)
	lat := make([]float64, 0, c.updates)
	missed := 0
	startCPU := cpuTime()
	startWall := time.Now()
	for i := 1; i <= c.updates; i++ {
		_ = atomicWrite(path, int64(i), c.fileBytes, true)
		d, ok := waitSeq(o, int64(i), 3*time.Second)
		if ok {
			lat = append(lat, float64(d.Microseconds())/1000)
		} else {
			missed++
		}
	}
	wall := time.Since(startWall)
	cpu := cpuTime() - startCPU
	o.close()
	sort.Float64s(lat)
	out, err := os.Create(c.output)
	if err != nil {
		panic(err)
	}
	defer out.Close()
	fmt.Fprintln(out, "updates\tobserved\tmissed\tp50_ms\tp95_ms\tp99_ms\tmax_ms\twall_ms\tcpu_ms\tcpu_percent\treads\tparse_errors\twatch_invalidations\tqueue_overflows")
	fmt.Fprintf(out, "%d\t%d\t%d\t%.3f\t%.3f\t%.3f\t%.3f\t%.3f\t%.3f\t%.3f\t%d\t%d\t%d\t%d\n",
		c.updates, len(lat), missed, pct(lat, .50), pct(lat, .95), pct(lat, .99), pct(lat, 1), float64(wall.Microseconds())/1000,
		float64(cpu.Microseconds())/1000, 100*float64(cpu)/float64(wall), o.reads.Load(), o.parseError.Load(), o.invalid.Load(), o.overflow.Load())
}

func idle(c config) {
	_ = os.RemoveAll(c.root)
	_ = os.MkdirAll(c.root, 0755)
	_ = atomicWrite(filepath.Join(c.root, "metrics"), 1, c.fileBytes, true)
	startCPU := cpuTime()
	startWall := time.Now()
	o := startObserver(c)
	time.Sleep(c.duration)
	o.close()
	wall := time.Since(startWall)
	cpu := cpuTime() - startCPU
	out, _ := os.Create(c.output)
	defer out.Close()
	fmt.Fprintln(out, "duration_ms\tcpu_ms\tcpu_percent\treads\tparse_errors")
	fmt.Fprintf(out, "%.3f\t%.3f\t%.3f\t%d\t%d\n", float64(wall.Microseconds())/1000, float64(cpu.Microseconds())/1000, 100*float64(cpu)/float64(wall), o.reads.Load(), o.parseError.Load())
}

func burst(c config) {
	_ = os.RemoveAll(c.root)
	_ = os.MkdirAll(c.root, 0755)
	path := filepath.Join(c.root, "metrics")
	o := startObserver(c)
	if c.initialPause > 0 {
		time.Sleep(50 * time.Millisecond)
	}
	startCPU := cpuTime()
	startWall := time.Now()
	for i := 1; i <= c.updates; i++ {
		_ = atomicWrite(path, int64(i), c.fileBytes, true)
	}
	produceElapsed := time.Since(startWall)
	_, finalObserved := waitSeq(o, int64(c.updates), 5*time.Second)
	totalElapsed := time.Since(startWall)
	cpu := cpuTime() - startCPU
	o.close()
	out, _ := os.Create(c.output)
	defer out.Close()
	fmt.Fprintln(out, "updates\tfinal_observed\tproduce_ms\ttotal_ms\tcpu_ms\treads\tparse_errors\twatch_invalidations\tqueue_overflows\tresult")
	result := "fail"
	if finalObserved {
		result = "pass"
	}
	fmt.Fprintf(out, "%d\t%t\t%.3f\t%.3f\t%.3f\t%d\t%d\t%d\t%d\t%s\n", c.updates, finalObserved,
		float64(produceElapsed.Microseconds())/1000, float64(totalElapsed.Microseconds())/1000,
		float64(cpu.Microseconds())/1000, o.reads.Load(), o.parseError.Load(), o.invalid.Load(), o.overflow.Load(), result)
}

func lostEvent(c config) {
	_ = os.RemoveAll(c.root)
	_ = os.MkdirAll(c.root, 0755)
	path := filepath.Join(c.root, "metrics")
	_ = atomicWrite(path, 1, c.fileBytes, true)
	o := startObserver(c)
	_, initialObserved := waitSeq(o, 1, 2*time.Second)

	o.dropEvents.Store(true)
	_ = atomicWrite(path, 2, c.fileBytes, true)
	deadline := time.Now().Add(2 * time.Second)
	var lastDropped int64
	quietSince := time.Now()
	for time.Now().Before(deadline) {
		current := o.dropped.Load()
		if current != lastDropped {
			lastDropped = current
			quietSince = time.Now()
		}
		if current > 0 && time.Since(quietSince) >= 100*time.Millisecond {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	eventDropped := o.dropped.Load() > 0
	o.dropEvents.Store(false)
	_, finalObserved := waitSeq(o, 2, 2*c.interval+250*time.Millisecond)
	o.close()

	expectedObserved := c.strategy == "hybrid"
	result := "fail"
	if initialObserved && eventDropped && finalObserved == expectedObserved {
		result = "pass"
	}
	out, _ := os.Create(c.output)
	defer out.Close()
	fmt.Fprintln(out, "strategy\tinitial_observed\tevent_dropped\tfinal_observed\texpected_final_observed\tresult")
	fmt.Fprintf(out, "%s\t%t\t%t\t%t\t%t\t%s\n", c.strategy, initialObserved, eventDropped, finalObserved, expectedObserved, result)
}

func cpuTime() time.Duration {
	var r syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &r)
	return time.Duration(r.Utime.Sec+r.Stime.Sec)*time.Second + time.Duration(r.Utime.Usec+r.Stime.Usec)*time.Microsecond
}
func pct(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	i := int(float64(len(v)-1) * p)
	return v[i]
}
func boolWord(v bool) string {
	if v {
		return "observed"
	}
	return "missing"
}

func main() {
	var c config
	mode := flag.String("mode", "correctness", "correctness, performance, idle, burst, or lost-event")
	flag.StringVar(&c.strategy, "strategy", "hybrid", "poll, inotify, or hybrid")
	flag.StringVar(&c.root, "root", "/data/watch", "watched directory")
	flag.StringVar(&c.output, "output", "/results/result.tsv", "result TSV")
	flag.DurationVar(&c.interval, "interval", time.Second, "poll/reconciliation interval")
	flag.DurationVar(&c.duration, "duration", 10*time.Second, "idle measurement duration")
	flag.DurationVar(&c.initialPause, "initial-read-pause", 0, "pause inotify reads after installing watches")
	flag.IntVar(&c.updates, "updates", 1000, "performance updates")
	flag.IntVar(&c.fileBytes, "file-bytes", 4096, "file size")
	flag.Parse()
	if c.strategy != "poll" && c.strategy != "inotify" && c.strategy != "hybrid" {
		panic("invalid strategy")
	}
	if err := os.MkdirAll(filepath.Dir(c.output), 0755); err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stderr, "mode=%s strategy=%s go=%s arch=%s\n", *mode, c.strategy, runtime.Version(), runtime.GOARCH)
	switch *mode {
	case "correctness":
		correctness(c)
	case "performance":
		performance(c)
	case "idle":
		idle(c)
	case "burst":
		burst(c)
	case "lost-event":
		lostEvent(c)
	default:
		panic("invalid mode")
	}
}
