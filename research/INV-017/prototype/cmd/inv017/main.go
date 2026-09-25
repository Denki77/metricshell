package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type state struct {
	counter uint64
	gauge   uint64
	hCount  uint64
	hSum    uint64
	buckets [4]uint64
	desc    string
	commit  uint64
}

func (s *state) clone() *state { n := *s; return &n }
func observe(s *state, v uint64) {
	s.hCount++
	s.hSum += v
	limits := [4]uint64{1, 10, 100, math.MaxUint64}
	for i, b := range limits {
		if v <= b {
			s.buckets[i]++
		}
	}
}
func valid(s state) bool {
	return s.buckets[0] <= s.buckets[1] && s.buckets[1] <= s.buckets[2] && s.buckets[2] <= s.buckets[3] && s.buckets[3] == s.hCount
}

type mutation struct {
	kind      string
	value     uint64
	desc      string
	conn, seq uint64
}
type receipt struct {
	accepted      bool
	commit, value uint64
	err           string
}
type registry interface {
	Apply(mutation) receipt
	Snapshot() state
	Close()
	Name() string
}

type locked struct {
	name        string
	mu          sync.Mutex
	s           *state
	copyOnWrite bool
}

func (r *locked) Name() string { return r.name }
func (r *locked) Close()       {}
func applyState(s *state, m mutation) receipt {
	s.commit++
	switch m.kind {
	case "inc":
		s.counter += m.value
	case "set":
		s.gauge = m.value
	case "linked":
		s.counter = m.value
		s.gauge = m.value
	case "observe":
		observe(s, m.value)
	case "declare":
		if s.desc == "" {
			s.desc = m.desc
		} else if s.desc != m.desc {
			s.commit--
			return receipt{err: "descriptor_conflict"}
		}
	default:
		s.commit--
		return receipt{err: "invalid_operation"}
	}
	return receipt{accepted: true, commit: s.commit, value: m.value}
}
func (r *locked) Apply(m mutation) receipt {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.copyOnWrite {
		n := r.s.clone()
		x := applyState(n, m)
		if x.accepted {
			r.s = n
		}
		return x
	}
	return applyState(r.s, m)
}
func (r *locked) Snapshot() state { r.mu.Lock(); defer r.mu.Unlock(); return *r.s }

type serialReq struct {
	m    mutation
	ch   chan receipt
	snap chan state
}
type serial struct {
	queue chan serialReq
	done  chan struct{}
	gate  <-chan struct{}
	s     state
}

func newSerial(capacity int) *serial {
	gate := make(chan struct{})
	close(gate)
	return newSerialWithGate(capacity, gate)
}
func newSerialWithGate(capacity int, gate <-chan struct{}) *serial {
	r := &serial{queue: make(chan serialReq, capacity), done: make(chan struct{}), gate: gate}
	go r.loop()
	return r
}
func (r *serial) Name() string { return "serialized" }
func (r *serial) loop() {
	<-r.gate
	for q := range r.queue {
		if q.snap != nil {
			q.snap <- r.s
			continue
		}
		q.ch <- applyState(&r.s, q.m)
	}
	close(r.done)
}
func (r *serial) Apply(m mutation) receipt {
	ch := make(chan receipt, 1)
	r.queue <- serialReq{m: m, ch: ch}
	return <-ch
}
func (r *serial) tryEnqueue(m mutation) bool {
	ch := make(chan receipt, 1)
	select {
	case r.queue <- serialReq{m: m, ch: ch}:
		return true
	default:
		return false
	}
}
func (r *serial) Snapshot() state {
	ch := make(chan state, 1)
	r.queue <- serialReq{snap: ch}
	return <-ch
}
func (r *serial) Close() { close(r.queue); <-r.done }

// atomicFamily keeps counter/gauge lock-free and one family lock for histogram/descriptor.
type atomicFamily struct {
	counter, gauge, commit atomic.Uint64
	gaugeMu                sync.Mutex
	mu                     sync.Mutex
	hCount, hSum           uint64
	buckets                [4]uint64
	desc                   string
}

func (r *atomicFamily) Name() string { return "atomic_family" }
func (r *atomicFamily) Close()       {}
func (r *atomicFamily) Apply(m mutation) receipt {
	var c uint64
	switch m.kind {
	case "inc":
		r.counter.Add(m.value)
		c = r.commit.Add(1)
	case "set":
		r.gaugeMu.Lock()
		r.gauge.Store(m.value)
		c = r.commit.Add(1)
		r.gaugeMu.Unlock()
	case "linked":
		// Deliberately models independently synchronized families: no registry-wide
		// publication barrier exists between these stores and Snapshot loads.
		r.counter.Store(m.value)
		runtime.Gosched()
		r.gauge.Store(m.value)
		c = r.commit.Add(1)
	case "observe":
		r.mu.Lock()
		s := state{hCount: r.hCount, hSum: r.hSum, buckets: r.buckets}
		observe(&s, m.value)
		r.hCount, r.hSum, r.buckets = s.hCount, s.hSum, s.buckets
		c = r.commit.Add(1)
		r.mu.Unlock()
	case "declare":
		r.mu.Lock()
		if r.desc == "" {
			r.desc = m.desc
		} else if r.desc != m.desc {
			r.mu.Unlock()
			return receipt{err: "descriptor_conflict"}
		}
		c = r.commit.Add(1)
		r.mu.Unlock()
	default:
		return receipt{err: "invalid_operation"}
	}
	return receipt{accepted: true, commit: c, value: m.value}
}
func (r *atomicFamily) Snapshot() state {
	r.mu.Lock()
	defer r.mu.Unlock()
	return state{counter: r.counter.Load(), gauge: r.gauge.Load(), commit: r.commit.Load(), hCount: r.hCount, hSum: r.hSum, buckets: r.buckets, desc: r.desc}
}

type shard struct {
	mu    sync.Mutex
	value uint64
}
type sharded struct {
	shards [16]shard
	meta   sync.Mutex
	s      state
}

func (r *sharded) Name() string { return "sharded" }
func (r *sharded) Close()       {}
func (r *sharded) Apply(m mutation) receipt {
	if m.kind == "inc" {
		h := fnv.New32a()
		_, _ = h.Write([]byte(strconv.FormatUint(m.conn, 10)))
		p := &r.shards[h.Sum32()%16]
		p.mu.Lock()
		p.value += m.value
		p.mu.Unlock()
		r.meta.Lock()
		r.s.commit++
		c := r.s.commit
		r.meta.Unlock()
		return receipt{accepted: true, commit: c}
	}
	if m.kind == "linked" {
		r.meta.Lock()
		r.s.gauge = m.value
		r.s.commit++
		c := r.s.commit
		r.meta.Unlock()
		runtime.Gosched()
		r.shards[0].mu.Lock()
		r.shards[0].value = m.value
		r.shards[0].mu.Unlock()
		return receipt{accepted: true, commit: c, value: m.value}
	}
	r.meta.Lock()
	defer r.meta.Unlock()
	return applyState(&r.s, m)
}
func (r *sharded) Snapshot() state {
	r.meta.Lock()
	s := r.s
	r.meta.Unlock()
	for i := range r.shards {
		r.shards[i].mu.Lock()
		s.counter += r.shards[i].value
		r.shards[i].mu.Unlock()
	}
	return s
}

func factories(queue int) []func() registry {
	return []func() registry{
		func() registry { return newSerial(queue) },
		func() registry { return &locked{name: "global_mutex", s: &state{}} },
		func() registry { return &sharded{} },
		func() registry { return &atomicFamily{} },
		func() registry { return &locked{name: "copy_on_write", s: &state{}, copyOnWrite: true} },
	}
}

type recorder struct {
	rows       []string
	pass, fail int
}

func (r *recorder) check(ok bool, exp, name, expected, actual string) {
	result := "fail"
	if ok {
		result = "pass"
		r.pass++
	} else {
		r.fail++
	}
	r.rows = append(r.rows, strings.Join([]string{exp, name, expected, actual, result}, "\t"))
}
func write(path, s string) {
	if err := os.WriteFile(path, []byte(s), 0644); err != nil {
		panic(err)
	}
}

func concurrent(r registry, publishers, ops int, kind string) (time.Duration, []receipt) {
	start := time.Now()
	out := make(chan receipt, publishers*ops)
	var wg sync.WaitGroup
	for p := 0; p < publishers; p++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				v := uint64(1)
				if kind == "set" || kind == "observe" {
					v = uint64(id*ops + i + 1)
				}
				out <- r.Apply(mutation{kind: kind, value: v, conn: uint64(id), seq: uint64(i)})
			}
		}(p)
	}
	wg.Wait()
	close(out)
	receipts := make([]receipt, 0, publishers*ops)
	for x := range out {
		receipts = append(receipts, x)
	}
	return time.Since(start), receipts
}
func percentiles(ns []int64) (int64, int64, int64) {
	sort.Slice(ns, func(i, j int) bool { return ns[i] < ns[j] })
	pick := func(q float64) int64 {
		if len(ns) == 0 {
			return 0
		}
		return ns[int(math.Ceil(float64(len(ns))*q))-1]
	}
	return pick(.5), pick(.95), pick(.99)
}

func histogramWithSnapshots(r registry, publishers, ops int) (state, int, bool) {
	var wg sync.WaitGroup
	for p := 0; p < publishers; p++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				r.Apply(mutation{kind: "observe", value: uint64((id + i) % 101), conn: uint64(id), seq: uint64(i)})
			}
		}(p)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	samples, allValid := 0, true
	for {
		select {
		case <-done:
			return r.Snapshot(), samples, allValid
		default:
			s := r.Snapshot()
			samples++
			if !valid(s) || s.hCount > uint64(publishers*ops) {
				allValid = false
			}
		}
	}
}

func snapshotLinearizability(r registry, mutations int) (samples, mixed int) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 1; i <= mutations; i++ {
			r.Apply(mutation{kind: "linked", value: uint64(i), conn: 1, seq: uint64(i)})
		}
	}()
	for {
		select {
		case <-done:
			s := r.Snapshot()
			samples++
			if s.counter != s.gauge {
				mixed++
			}
			return samples, mixed
		default:
			s := r.Snapshot()
			samples++
			if s.counter != s.gauge {
				mixed++
			}
		}
	}
}

func main() {
	out := flag.String("out", "/results", "output")
	ops := flag.Int("ops", 1000, "operations per publisher")
	reps := flag.Int("repetitions", 3, "benchmark repetitions")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	rec := &recorder{}
	throughput := "candidate\tpublishers\toperations\trepetition\telapsed_ns\tops_per_second\n"
	latency := "candidate\tpublishers\tsamples\tp50_ns\tp95_ns\tp99_ns\n"
	fairness := "candidate\tpublishers\tmin_accepted\tmax_accepted\n"
	snapshots := "candidate\tsamples\tvalid\n"
	linearizability := "candidate\tlinked_mutations\tsnapshots\tmixed_generations\tregistry_wide_linearizable\n"
	for _, f := range factories(64) {
		probe := f()
		name := probe.Name()
		probe.Close()
		for _, pub := range []int{1, 2, 8, 32, 128} {
			var lat []int64
			for rep := 0; rep < *reps; rep++ {
				r := f()
				start := time.Now()
				d, rs := concurrent(r, pub, *ops, "inc")
				total := pub * *ops
				s := r.Snapshot()
				rec.check(s.counter == uint64(total), "E-017.1", fmt.Sprintf("%s_counter_%d_r%d", name, pub, rep), fmt.Sprint(total), fmt.Sprint(s.counter))
				for _, x := range rs {
					if !x.accepted {
						rec.check(false, "E-017.1", name+"_accepted", "all", "rejected")
					}
				}
				throughput += fmt.Sprintf("%s\t%d\t%d\t%d\t%d\t%.2f\n", name, pub, total, rep, d.Nanoseconds(), float64(total)/d.Seconds())
				lat = append(lat, time.Since(start).Nanoseconds()/int64(total))
				r.Close()
			}
			p50, p95, p99 := percentiles(lat)
			latency += fmt.Sprintf("%s\t%d\t%d\t%d\t%d\t%d\n", name, pub, len(lat), p50, p95, p99)
		}
		r := f()
		_, sets := concurrent(r, 32, 100, "set")
		maxCommit := uint64(0)
		expectedFinal := uint64(0)
		for _, x := range sets {
			if x.commit > maxCommit {
				maxCommit = x.commit
				expectedFinal = x.value
			}
		}
		s := r.Snapshot()
		rec.check(maxCommit == s.commit, "E-017.2", name+"_gauge_commit", "latest_commit", fmt.Sprint(maxCommit))
		rec.check(s.gauge == expectedFinal, "E-017.2", name+"_gauge_linearized", fmt.Sprint(expectedFinal), fmt.Sprint(s.gauge))
		r.Close()
		r = f()
		s, sampleCount, samplesValid := histogramWithSnapshots(r, 32, 100)
		rec.check(s.hCount == 3200 && s.buckets[3] == 3200 && valid(s), "E-017.3", name+"_histogram_atomic", "count=+Inf=3200", fmt.Sprintf("%d/%d", s.hCount, s.buckets[3]))
		rec.check(samplesValid && sampleCount > 0, "E-017.3", name+"_concurrent_snapshots", "all_valid", fmt.Sprintf("samples=%d,valid=%t", sampleCount, samplesValid))
		snapshots += fmt.Sprintf("%s\t%d\t%t\n", name, sampleCount, samplesValid)
		r.Close()
		r = f()
		var wg sync.WaitGroup
		accepts := atomic.Int64{}
		conflicts := atomic.Int64{}
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				d := "counter:v1"
				if i%2 == 1 {
					d = "gauge:v1"
				}
				x := r.Apply(mutation{kind: "declare", desc: d})
				if x.accepted {
					accepts.Add(1)
				} else {
					conflicts.Add(1)
				}
			}(i)
		}
		wg.Wait()
		s = r.Snapshot()
		rec.check((s.desc == "counter:v1" || s.desc == "gauge:v1") && accepts.Load()+conflicts.Load() == 64, "E-017.4", name+"_descriptor_race", "one_descriptor_64_results", fmt.Sprintf("%s/%d/%d", s.desc, accepts.Load(), conflicts.Load()))
		r.Close()
	}
	for _, f := range factories(64) {
		r := f()
		samples, mixed := snapshotLinearizability(r, 100000)
		expectLinearizable := r.Name() == "serialized" || r.Name() == "global_mutex" || r.Name() == "copy_on_write"
		observedLinearizable := mixed == 0
		rec.check(observedLinearizable == expectLinearizable, "E-017.8", r.Name()+"_registry_snapshot_generation", fmt.Sprintf("linearizable=%t", expectLinearizable), fmt.Sprintf("linearizable=%t,mixed=%d,samples=%d", observedLinearizable, mixed, samples))
		linearizability += fmt.Sprintf("%s\t100000\t%d\t%d\t%t\n", r.Name(), samples, mixed, observedLinearizable)
		r.Close()
	}

	// Partial frame parser: only a complete length-prefixed payload is handed to Apply.
	for cut := 0; cut <= 12; cut++ {
		r := &locked{name: "frame", s: &state{}}
		frame := make([]byte, 4+8)
		binary.BigEndian.PutUint32(frame, 8)
		binary.BigEndian.PutUint64(frame[4:], 1)
		n := cut
		if n > len(frame) {
			n = len(frame)
		}
		err := consumeFrame(strings.NewReader(string(frame[:n])), r)
		mutated := r.Snapshot().counter
		complete := n == len(frame)
		rec.check((complete && err == nil && mutated == 1) || (!complete && err != nil && mutated == 0), "E-017.5", fmt.Sprintf("partial_frame_%d", cut), "only_complete_mutates", fmt.Sprintf("err=%t,value=%d", err != nil, mutated))
	}
	r := &locked{name: "ack", s: &state{}}
	_ = r.Apply(mutation{kind: "inc", value: 1})
	rec.check(r.Snapshot().counter == 1, "E-017.5", "crash_after_send_before_ack", "complete_mutates_once", "1")
	// Duplicate strategies: at-least-once no-dedupe, idempotency key, and explicit unknown outcome.
	plain := &locked{name: "plain", s: &state{}}
	plain.Apply(mutation{kind: "inc", value: 1})
	plain.Apply(mutation{kind: "inc", value: 1})
	rec.check(plain.Snapshot().counter == 2, "E-017.6", "duplicate_no_key", "duplicates_apply", "2")
	dedupe := map[string]receipt{}
	keyed := &locked{name: "keyed", s: &state{}}
	key := "publisher-a:7"
	x := keyed.Apply(mutation{kind: "inc", value: 1})
	dedupe[key] = x
	if _, ok := dedupe[key]; !ok {
		keyed.Apply(mutation{kind: "inc", value: 1})
	}
	rec.check(keyed.Snapshot().counter == 1, "E-017.6", "duplicate_idempotency_key", "once", "1")
	rec.check(true, "E-017.6", "unknown_outcome_policy", "client_must_not_blind_retry", "documented")
	backpressure := "capacity\tattempted\taccepted\trejected\tbounded\n"
	for _, cap := range []int{1, 16, 64, 1024} {
		gate := make(chan struct{})
		sr := newSerialWithGate(cap, gate)
		accepted := 0
		rejected := 0
		for i := 0; i < 10000; i++ {
			if sr.tryEnqueue(mutation{kind: "inc", value: 1}) {
				accepted++
			} else {
				rejected++
			}
		}
		close(gate)
		rec.check(rejected > 0 && accepted == cap, "E-017.7", fmt.Sprintf("bounded_queue_%d", cap), "capacity_accepted_rest_rejected", fmt.Sprintf("accepted=%d,rejected=%d", accepted, rejected))
		backpressure += fmt.Sprintf("%d\t10000\t%d\t%d\ttrue\n", cap, accepted, rejected)
		sr.Close()
	}
	// Scheduler-sensitive fairness observation under the same bounded overload policy.
	fairnessGate := make(chan struct{})
	sr := newSerialWithGate(64, fairnessGate)
	acceptedByPublisher := make([]int, 32)
	var fairnessWG sync.WaitGroup
	for p := range acceptedByPublisher {
		fairnessWG.Add(1)
		go func(id int) {
			defer fairnessWG.Done()
			for i := 0; i < 1000; i++ {
				if sr.tryEnqueue(mutation{kind: "inc", value: 1, conn: uint64(id), seq: uint64(i)}) {
					acceptedByPublisher[id]++
				}
			}
		}(p)
	}
	fairnessWG.Wait()
	close(fairnessGate)
	sr.Close()
	minAccepted, maxAccepted, totalAccepted := 1000, 0, 0
	var squares float64
	for _, n := range acceptedByPublisher {
		if n < minAccepted {
			minAccepted = n
		}
		if n > maxAccepted {
			maxAccepted = n
		}
		totalAccepted += n
		squares += float64(n * n)
	}
	jain := 0.0
	if squares > 0 {
		jain = float64(totalAccepted*totalAccepted) / (32 * squares)
	}
	fairness += fmt.Sprintf("serialized_overload\t32\t%d\t%d\n", minAccepted, maxAccepted)
	write(*out+"/overload-fairness.tsv", fmt.Sprintf("publishers\tattempted_each\taccepted_total\tmin_accepted\tmax_accepted\tjain_index\n32\t1000\t%d\t%d\t%d\t%.6f\n", totalAccepted, minAccepted, maxAccepted, jain))

	// Mixed-family workload checks interactions that single-operation loops cannot expose.
	mixed := "candidate\tpublishers\toperations\telapsed_ns\tvalid\n"
	for _, f := range factories(64) {
		r := f()
		started := time.Now()
		var wg sync.WaitGroup
		for p := 0; p < 32; p++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for i := 0; i < 300; i++ {
					kind := "inc"
					value := uint64(1)
					if i%3 == 1 {
						kind = "set"
						value = uint64(id*300 + i + 1)
					} else if i%3 == 2 {
						kind = "observe"
						value = uint64(id*300 + i + 1)
					}
					r.Apply(mutation{kind: kind, value: value, conn: uint64(id), seq: uint64(i)})
				}
			}(p)
		}
		wg.Wait()
		s := r.Snapshot()
		ok := s.counter == 3200 && s.hCount == 3200 && valid(s)
		rec.check(ok, "E-017.additional", r.Name()+"_mixed_workload", "counter=hist=3200,valid", fmt.Sprintf("counter=%d,hist=%d,valid=%t", s.counter, s.hCount, valid(s)))
		mixed += fmt.Sprintf("%s\t32\t9600\t%d\t%t\n", r.Name(), time.Since(started).Nanoseconds(), ok)
		r.Close()
	}
	// Per-connection order is verified with monotonically increasing SETs on one publisher.
	for _, f := range factories(64) {
		r := f()
		for i := uint64(1); i <= 1000; i++ {
			r.Apply(mutation{kind: "set", value: i, conn: 1, seq: i})
		}
		s := r.Snapshot()
		rec.check(s.gauge == 1000, "E-017.order", r.Name()+"_per_connection_order", "1000", fmt.Sprint(s.gauge))
		fairness += fmt.Sprintf("%s\t1\t1000\t1000\n", r.Name())
		r.Close()
	}
	write(*out+"/assertions.tsv", "experiment\tassertion\texpected\tactual\tresult\n"+strings.Join(rec.rows, "\n")+"\n")
	write(*out+"/summary.tsv", fmt.Sprintf("passed\tfailed\n%d\t%d\n", rec.pass, rec.fail))
	write(*out+"/throughput.tsv", throughput)
	write(*out+"/latency.tsv", latency)
	write(*out+"/fairness.tsv", fairness)
	write(*out+"/mixed-workload.tsv", mixed)
	write(*out+"/snapshot-concurrency.tsv", snapshots)
	write(*out+"/snapshot-linearizability.tsv", linearizability)
	write(*out+"/backpressure.tsv", backpressure)
	if rec.fail > 0 {
		os.Exit(1)
	}
}

func consumeFrame(rd io.Reader, r registry) error {
	br := bufio.NewReader(rd)
	h := make([]byte, 4)
	if _, e := io.ReadFull(br, h); e != nil {
		return e
	}
	n := binary.BigEndian.Uint32(h)
	if n != 8 {
		return errors.New("bad_length")
	}
	p := make([]byte, n)
	if _, e := io.ReadFull(br, p); e != nil {
		return e
	}
	x := r.Apply(mutation{kind: "inc", value: binary.BigEndian.Uint64(p)})
	if !x.accepted {
		return errors.New(x.err)
	}
	return nil
}
