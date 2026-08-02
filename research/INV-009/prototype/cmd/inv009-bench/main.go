package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	headerSize = 64
	version    = 1
)

var magic = [8]byte{'M', 'S', 'H', 'M', 'R', 'I', 'N', 'G'}

type ring struct {
	b         []byte
	f         *os.File
	slots     uint64
	slotBytes int
	seq       atomic.Uint64
	dropped   atomic.Uint64
}

func openRing(path string, slots, payload int, reset bool) (*ring, error) {
	if slots < 2 {
		return nil, fmt.Errorf("slots must be >= 2")
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	slotBytes := 16 + payload
	size := headerSize + slots*slotBytes
	if reset {
		if e = f.Truncate(int64(size)); e != nil {
			f.Close()
			return nil, e
		}
	}
	st, e := f.Stat()
	if e != nil || st.Size() != int64(size) {
		f.Close()
		return nil, fmt.Errorf("size mismatch")
	}
	b, e := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if e != nil {
		f.Close()
		return nil, e
	}
	r := &ring{b: b, f: f, slots: uint64(slots), slotBytes: slotBytes}
	if reset {
		copy(b[:8], magic[:])
		binary.LittleEndian.PutUint32(b[8:12], version)
		binary.LittleEndian.PutUint64(b[16:24], uint64(slots))
	} else if string(b[:8]) != string(magic[:]) || binary.LittleEndian.Uint32(b[8:12]) != version {
		r.close()
		return nil, fmt.Errorf("schema mismatch")
	} else {
		r.seq.Store(binary.LittleEndian.Uint64(b[24:32]))
	}
	return r, nil
}
func (r *ring) close() {
	binary.LittleEndian.PutUint64(r.b[24:32], r.seq.Load())
	_, _, _ = syscall.Syscall(syscall.SYS_MSYNC, uintptr(unsafe.Pointer(&r.b[0])), uintptr(len(r.b)), uintptr(syscall.MS_SYNC))
	_ = syscall.Munmap(r.b)
	_ = r.f.Close()
}
func ptr(b *byte) *uint64 { return (*uint64)(unsafe.Pointer(b)) }
func (r *ring) reserve() uint64 {
	n := r.seq.Add(1)
	if n > r.slots {
		r.dropped.Add(1)
	}
	return n
}
func (r *ring) off(n uint64) int { return headerSize + int((n-1)%r.slots)*r.slotBytes }
func (r *ring) publish(n uint64, body []byte) {
	off := r.off(n)
	atomic.StoreUint64(ptr(&r.b[off]), 0)
	atomic.StoreUint64(ptr(&r.b[off+8]), 0)
	copy(r.b[off+16:off+r.slotBytes], body)
	atomic.StoreUint64(ptr(&r.b[off]), n)
}

func snapshot(size, publisher int) []byte {
	prefix := []byte(fmt.Sprintf("# TYPE inv009_value gauge\ninv009_value{publisher=\"%d\"} %d\n", publisher, publisher))
	if size < len(prefix)+2 {
		size = len(prefix) + 2
	}
	b := make([]byte, size)
	copy(b, prefix)
	for i := len(prefix); i < len(b)-1; i++ {
		b[i] = '#'
	}
	b[len(b)-1] = '\n'
	return b
}
func validSnapshot(b []byte) bool {
	return bytes.HasPrefix(b, []byte("# TYPE inv009_value gauge\n")) && bytes.Contains(b, []byte("inv009_value{publisher=\"")) && len(b) > 0 && b[len(b)-1] == '\n'
}
func percentile(v []int64, p float64) int64 {
	if len(v) == 0 {
		return 0
	}
	x := append([]int64(nil), v...)
	sort.Slice(x, func(i, j int) bool { return x[i] < x[j] })
	return x[int(float64(len(x)-1)*p)]
}
func usage() (int64, int64) {
	var u syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &u)
	return (u.Utime.Sec+u.Stime.Sec)*1e9 + int64(u.Utime.Usec+u.Stime.Usec)*1e3, u.Maxrss
}

type activeState struct{ body []byte }
type activeCheck struct {
	active     atomic.Pointer[activeState]
	stop       chan struct{}
	allowed    map[string]struct{}
	reads, bad atomic.Uint64
}

func newActiveCheck(size, publishers int) *activeCheck {
	a := &activeCheck{stop: make(chan struct{}), allowed: make(map[string]struct{})}
	initial := snapshot(size, 99)
	a.allowed[string(initial)] = struct{}{}
	for p := 0; p < publishers; p++ {
		a.allowed[string(snapshot(size, p))] = struct{}{}
	}
	a.active.Store(&activeState{body: initial})
	go func() {
		for {
			select {
			case <-a.stop:
				return
			default:
				s := a.active.Load()
				a.reads.Add(1)
				allowed := false
				if s != nil {
					_, allowed = a.allowed[string(s.body)]
				}
				if !allowed {
					a.bad.Add(1)
				}
				runtime.Gosched()
			}
		}
	}()
	return a
}
func (a *activeCheck) install(body []byte) {
	copyBody := append([]byte(nil), body...)
	a.active.Store(&activeState{body: copyBody})
}
func (a *activeCheck) close() { close(a.stop) }

type result struct {
	transport               string
	publishers, size, count int
	elapsed                 time.Duration
	publisherLat, e2eLat    []int64
	accepted                uint64
	errors                  int
	cpuNS, maxRSSKB         int64
	readerReads, readerBad  uint64
}

func benchMmap(transport, path string, publishers, size, each int) result {
	total := publishers * each
	r, _ := openRing(path, total+1, size, true)
	defer r.close()
	active := newActiveCheck(size, publishers)
	defer active.close()
	pubCh := make(chan int64, total)
	e2eCh := make(chan int64, total)
	var accepted atomic.Uint64
	consumerDone := make(chan struct{})
	commits := make(chan uint64, total)
	var publishMu sync.Mutex
	go func() {
		defer close(consumerDone)
		for i := 0; i < total; i++ {
			n := <-commits
			off := r.off(n)
			if atomic.LoadUint64(ptr(&r.b[off])) != n {
				continue
			}
			body := append([]byte(nil), r.b[off+16:off+r.slotBytes]...)
			if validSnapshot(body) {
				active.install(body)
				accepted.Add(1)
			}
			atomic.StoreUint64(ptr(&r.b[off+8]), n)
		}
	}()
	cpu0, _ := usage()
	start := time.Now()
	var wg sync.WaitGroup
	for p := 0; p < publishers; p++ {
		body := snapshot(size, p)
		wg.Add(1)
		go func(body []byte) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				began := time.Now()
				publishMu.Lock()
				n := r.reserve()
				r.publish(n, body)
				commits <- n
				publishMu.Unlock()
				pubCh <- time.Since(began).Nanoseconds()
				off := r.off(n)
				for atomic.LoadUint64(ptr(&r.b[off+8])) != n {
					runtime.Gosched()
				}
				e2eCh <- time.Since(began).Nanoseconds()
			}
		}(body)
	}
	wg.Wait()
	<-consumerDone
	elapsed := time.Since(start)
	cpu1, rss := usage()
	close(pubCh)
	close(e2eCh)
	rr := result{transport: transport, publishers: publishers, size: size, count: total, elapsed: elapsed, accepted: accepted.Load(), cpuNS: cpu1 - cpu0, maxRSSKB: rss, readerReads: active.reads.Load(), readerBad: active.bad.Load()}
	for n := range pubCh {
		rr.publisherLat = append(rr.publisherLat, n)
	}
	for n := range e2eCh {
		rr.e2eLat = append(rr.e2eLat, n)
	}
	rr.errors = total - int(rr.accepted) + int(rr.readerBad)
	return rr
}

func benchSocket(path string, publishers, size, each int) result {
	_ = os.Remove(path)
	ln, e := net.Listen("unix", path)
	if e != nil {
		return result{errors: 1}
	}
	defer ln.Close()
	total := publishers * each
	active := newActiveCheck(size, publishers)
	defer active.close()
	var accepted atomic.Uint64
	var installMu sync.Mutex
	var server sync.WaitGroup
	server.Add(1)
	go func() {
		defer server.Done()
		var clients sync.WaitGroup
		for i := 0; i < publishers; i++ {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			clients.Add(1)
			go func(c net.Conn) {
				defer clients.Done()
				defer c.Close()
				br := bufio.NewReader(c)
				ack := make([]byte, 8)
				for {
					seq, err := binary.ReadUvarint(br)
					if err != nil {
						return
					}
					n, err := binary.ReadUvarint(br)
					if err != nil || n > 1<<20 {
						return
					}
					body := make([]byte, n)
					if _, err = io.ReadFull(br, body); err != nil {
						return
					}
					installMu.Lock()
					ok := validSnapshot(body)
					if ok {
						active.install(body)
						accepted.Add(1)
					}
					installMu.Unlock()
					binary.LittleEndian.PutUint64(ack, seq)
					if _, err = c.Write(ack); err != nil {
						return
					}
				}
			}(c)
		}
		clients.Wait()
	}()
	pubCh := make(chan int64, total)
	e2eCh := make(chan int64, total)
	cpu0, _ := usage()
	start := time.Now()
	var wg sync.WaitGroup
	var seq atomic.Uint64
	var errs atomic.Int64
	for p := 0; p < publishers; p++ {
		body := snapshot(size, p)
		wg.Add(1)
		go func(body []byte) {
			defer wg.Done()
			c, err := net.Dial("unix", path)
			if err != nil {
				errs.Add(1)
				return
			}
			defer c.Close()
			ack := make([]byte, 8)
			h := make([]byte, 2*binary.MaxVarintLen64)
			for i := 0; i < each; i++ {
				began := time.Now()
				s := seq.Add(1)
				n := binary.PutUvarint(h, s)
				n += binary.PutUvarint(h[n:], uint64(len(body)))
				if _, err = c.Write(h[:n]); err == nil {
					_, err = c.Write(body)
				}
				pubCh <- time.Since(began).Nanoseconds()
				if err == nil {
					_, err = io.ReadFull(c, ack)
				}
				if err != nil || binary.LittleEndian.Uint64(ack) != s {
					errs.Add(1)
				}
				e2eCh <- time.Since(began).Nanoseconds()
			}
		}(body)
	}
	wg.Wait()
	_ = ln.Close()
	server.Wait()
	elapsed := time.Since(start)
	cpu1, rss := usage()
	close(pubCh)
	close(e2eCh)
	rr := result{transport: "unix_socket", publishers: publishers, size: size, count: total, elapsed: elapsed, accepted: accepted.Load(), errors: int(errs.Load()), cpuNS: cpu1 - cpu0, maxRSSKB: rss, readerReads: active.reads.Load(), readerBad: active.bad.Load()}
	for n := range pubCh {
		rr.publisherLat = append(rr.publisherLat, n)
	}
	for n := range e2eCh {
		rr.e2eLat = append(rr.e2eLat, n)
	}
	rr.errors += total - int(rr.accepted) + int(rr.readerBad)
	return rr
}

func writePublisherResults(path string, rows []result) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	fmt.Fprintln(f, "transport\tconcurrent_publishers\tsnapshot_bytes\tpublications\tpublisher_publications_per_second\tpublisher_commit_p50_ns\tpublisher_commit_p95_ns\tpublisher_commit_p99_ns")
	for _, r := range rows {
		var total int64
		for _, n := range r.publisherLat {
			total += n
		}
		rate := float64(0)
		if total > 0 {
			rate = float64(len(r.publisherLat)) * 1e9 / float64(total)
		}
		fmt.Fprintf(f, "%s\t%d\t%d\t%d\t%.0f\t%d\t%d\t%d\n", r.transport, r.publishers, r.size, r.count, rate, percentile(r.publisherLat, .5), percentile(r.publisherLat, .95), percentile(r.publisherLat, .99))
	}
	return nil
}
func writeAcceptanceResults(path string, rows []result) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	fmt.Fprintln(f, "transport\tconcurrent_publishers\tsnapshot_bytes\tpublications\telapsed_ns\taccepted_snapshots_per_second\tend_to_end_p50_ns\tend_to_end_p95_ns\tend_to_end_p99_ns\terrors\taccepted_snapshots\tactive_reader_reads\tactive_reader_bad_reads")
	for _, r := range rows {
		rate := float64(r.accepted) / r.elapsed.Seconds()
		fmt.Fprintf(f, "%s\t%d\t%d\t%d\t%d\t%.0f\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", r.transport, r.publishers, r.size, r.count, r.elapsed.Nanoseconds(), rate, percentile(r.e2eLat, .5), percentile(r.e2eLat, .95), percentile(r.e2eLat, .99), r.errors, r.accepted, r.readerReads, r.readerBad)
	}
	return nil
}
func writeResources(path string, rows []result) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	fmt.Fprintln(f, "transport\tconcurrent_publishers\tsnapshot_bytes\taccepted_snapshots\tcpu_ns\tcpu_ns_per_accepted_snapshot\tmax_rss_kb")
	for _, r := range rows {
		cpuPer := int64(0)
		if r.accepted > 0 {
			cpuPer = r.cpuNS / int64(r.accepted)
		}
		fmt.Fprintf(f, "%s\t%d\t%d\t%d\t%d\t%d\t%d\n", r.transport, r.publishers, r.size, r.accepted, r.cpuNS, cpuPer, r.maxRSSKB)
	}
	return nil
}
func assertion(f *os.File, name, expected, actual string) {
	res := "fail"
	if expected == actual {
		res = "pass"
	}
	fmt.Fprintf(f, "%s\t%s\t%s\t%s\n", name, expected, actual, res)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "crash-write" {
		r, e := openRing(os.Args[2], 8, 128, false)
		if e != nil {
			os.Exit(2)
		}
		copy(r.b[headerSize+16:headerSize+32], []byte("partial-snapshot"))
		os.Exit(99)
	}
	if len(os.Args) > 1 && os.Args[1] == "allocate" {
		n, _ := strconv.Atoi(os.Args[2])
		b := make([]byte, n)
		for i := range b {
			b[i] = 1
		}
		fmt.Println(len(b))
		return
	}
	out := "/out"
	_ = os.MkdirAll(out, 0755)
	tmp, _ := os.MkdirTemp("", "inv009-")
	defer os.RemoveAll(tmp)
	var rows []result
	for _, size := range []int{128, 4096, 65536} {
		for _, publishers := range []int{1, 8} {
			each := 500
			if size >= 4096 {
				each = 100
			}
			if size >= 65536 {
				each = 20
			}
			rows = append(rows, benchMmap("mmap_file", filepath.Join(tmp, "ring"), publishers, size, each))
			rows = append(rows, benchMmap("mmap_tmpfs", filepath.Join("/dev/shm", fmt.Sprintf("inv009-%d", os.Getpid())), publishers, size, each))
			rows = append(rows, benchSocket(filepath.Join(tmp, "sock"), publishers, size, each))
		}
	}
	_ = writePublisherResults(filepath.Join(out, "publisher-performance.tsv"), rows)
	_ = writeAcceptanceResults(filepath.Join(out, "acceptance-performance.tsv"), rows)
	_ = writeResources(filepath.Join(out, "resources.tsv"), rows)
	af, _ := os.Create(filepath.Join(out, "assertions.tsv"))
	fmt.Fprintln(af, "assertion\texpected\tactual\tresult")
	for _, r := range rows {
		base := fmt.Sprintf("%s_%d_%d", r.transport, r.publishers, r.size)
		assertion(af, base+"_no_errors", "0", strconv.Itoa(r.errors))
		assertion(af, base+"_accepted", strconv.Itoa(r.count), strconv.FormatUint(r.accepted, 10))
		assertion(af, base+"_active_reader_bad_reads", "0", strconv.FormatUint(r.readerBad, 10))
		positive := r.readerReads > 0
		assertion(af, base+"_active_reader_observed", "true", strconv.FormatBool(positive))
	}
	r, _ := openRing(filepath.Join(tmp, "overflow"), 64, 128, true)
	body := snapshot(128, 1)
	for i := 0; i < 1000; i++ {
		n := r.reserve()
		r.publish(n, body)
	}
	assertion(af, "overflow_accounting", "936", strconv.FormatUint(r.dropped.Load(), 10))
	r.close()
	r, _ = openRing(filepath.Join(tmp, "restart"), 128, 128, true)
	for i := 0; i < 100; i++ {
		n := r.reserve()
		r.publish(n, body)
	}
	r.close()
	r, e := openRing(filepath.Join(tmp, "restart"), 128, 128, false)
	if e == nil {
		assertion(af, "restart_sequence_recovery", "100", strconv.FormatUint(r.seq.Load(), 10))
		r.close()
	} else {
		assertion(af, "restart_sequence_recovery", "open", e.Error())
	}
	r, _ = openRing(filepath.Join(tmp, "schema"), 8, 128, true)
	binary.LittleEndian.PutUint32(r.b[8:12], 99)
	r.close()
	_, e = openRing(filepath.Join(tmp, "schema"), 8, 128, false)
	assertion(af, "schema_mismatch_rejected", "true", strconv.FormatBool(e != nil))
	crashPath := filepath.Join(tmp, "crash")
	r, _ = openRing(crashPath, 8, 128, true)
	r.close()
	self, _ := os.Executable()
	cmd := exec.Command(self, "crash-write", crashPath)
	e = cmd.Run()
	exitCode := -1
	if ee, ok := e.(*exec.ExitError); ok {
		exitCode = ee.ExitCode()
	}
	assertion(af, "writer_crash_exit", "99", strconv.Itoa(exitCode))
	r, e = openRing(crashPath, 8, 128, false)
	if e == nil {
		assertion(af, "crash_torn_candidate_uncommitted", "0", strconv.FormatUint(atomic.LoadUint64(ptr(&r.b[headerSize])), 10))
		r.close()
	} else {
		assertion(af, "crash_reopen", "success", e.Error())
	}
	st, _ := os.Stat(filepath.Join(tmp, "ring"))
	assertion(af, "mapping_permissions", "0600", fmt.Sprintf("%04o", st.Mode().Perm()))
	af.Close()
	ef, _ := os.Create(filepath.Join(out, "container-environment.tsv"))
	fmt.Fprintln(ef, "key\tvalue")
	fmt.Fprintf(ef, "go_version\t%s\narchitecture\t%s\nos\t%s\ncpu_count\t%d\n", runtime.Version(), runtime.GOARCH, runtime.GOOS, runtime.NumCPU())
	ef.Close()
	fmt.Printf("performance_rows=%d\n", len(rows))
}
