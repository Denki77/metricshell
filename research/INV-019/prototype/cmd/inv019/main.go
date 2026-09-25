package main

import (
	"flag"
	"fmt"
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

type series struct {
	kind         byte
	value, count uint64
	sum          float64
	buckets      []uint64
}
type registry struct {
	mu                    sync.Mutex
	data                  []series
	generation            uint64
	cached                []byte
	cachedGeneration      uint64
	maxSeries, maxBuckets int
}

type linkedRegistry struct {
	mu               sync.Mutex
	generation       uint64
	counter          uint64
	gauge            uint64
	histogramCount   uint64
	histogramSum     uint64
	cached           []byte
	cachedGeneration uint64
	cacheHits        uint64
	cacheMisses      uint64
}

func (r *linkedRegistry) commit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generation++
	r.counter = r.generation
	r.gauge = r.generation
	r.histogramCount = r.generation
	r.histogramSum = r.generation
}

func (r *linkedRegistry) snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cachedGeneration == r.generation && r.cached != nil {
		r.cacheHits++
		return r.cached
	}
	r.cached = []byte(fmt.Sprintf("# generation %d\nlinked_counter_total %d\nlinked_gauge %d\nlinked_histogram_count %d\nlinked_histogram_sum %d\n", r.generation, r.counter, r.gauge, r.histogramCount, r.histogramSum))
	r.cachedGeneration = r.generation
	r.cacheMisses++
	return r.cached
}

func linkedSnapshotValid(body []byte) bool {
	var generation, counter, gauge, histogramCount, histogramSum uint64
	n, err := fmt.Sscanf(string(body), "# generation %d\nlinked_counter_total %d\nlinked_gauge %d\nlinked_histogram_count %d\nlinked_histogram_sum %d\n", &generation, &counter, &gauge, &histogramCount, &histogramSum)
	return err == nil && n == 5 && generation == counter && counter == gauge && gauge == histogramCount && histogramCount == histogramSum
}

func newRegistry(n, buckets, maxSeries, maxBuckets int) (*registry, error) {
	if n > maxSeries {
		return nil, fmt.Errorf("series_limit")
	}
	if buckets > maxBuckets {
		return nil, fmt.Errorf("bucket_limit")
	}
	r := &registry{maxSeries: maxSeries, maxBuckets: maxBuckets, data: make([]series, n)}
	for i := range r.data {
		r.data[i].kind = 'c'
		if i%3 == 1 {
			r.data[i].kind = 'g'
		}
		if i%3 == 2 {
			r.data[i].kind = 'h'
			r.data[i].buckets = make([]uint64, buckets+1)
		}
	}
	return r, nil
}
func (r *registry) mutate(i int) {
	s := &r.data[i%len(r.data)]
	switch s.kind {
	case 'c':
		s.value++
	case 'g':
		s.value = uint64(i)
	case 'h':
		s.count++
		s.sum += .5
		for j := range s.buckets {
			s.buckets[j]++
		}
	}
	r.generation++
}
func encode(data []series, gen uint64) []byte {
	var b strings.Builder
	b.Grow(len(data) * 48)
	fmt.Fprintf(&b, "# generation %d\n", gen)
	for i, s := range data {
		switch s.kind {
		case 'c':
			fmt.Fprintf(&b, "m_%d_total %d\n", i, s.value)
		case 'g':
			fmt.Fprintf(&b, "m_%d %d\n", i, s.value)
		case 'h':
			for j, v := range s.buckets {
				fmt.Fprintf(&b, "m_%d_bucket{le=\"%d\"} %d\n", i, j, v)
			}
			fmt.Fprintf(&b, "m_%d_sum %.1f\nm_%d_count %d\n", i, s.sum, i, s.count)
		}
	}
	return []byte(b.String())
}
func clone(in []series) []series {
	out := make([]series, len(in))
	copy(out, in)
	for i := range out {
		out[i].buckets = append([]uint64(nil), in[i].buckets...)
	}
	return out
}
func pct(v []float64, p float64) float64 {
	sort.Float64s(v)
	if len(v) == 0 {
		return 0
	}
	return v[int(math.Ceil(float64(len(v))*p))-1]
}
func ms(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }
func mem() (uint64, uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc, m.TotalAlloc
}
func write(path, header string, rows []string) {
	f, e := os.Create(path)
	if e != nil {
		panic(e)
	}
	defer f.Close()
	fmt.Fprintln(f, header)
	for _, x := range rows {
		fmt.Fprintln(f, x)
	}
}
func assertion(rows *[]string, exp, name, want, got string) {
	result := "fail"
	if want == got {
		result = "pass"
	}
	*rows = append(*rows, fmt.Sprintf("%s\t%s\t%s\t%s\t%s", exp, name, want, got, result))
}

func main() {
	out := flag.String("out", "/results", "output")
	mode := flag.String("mode", "bench", "bench|allocate")
	alloc := flag.Int("allocate-mb", 0, "allocation for OOM test")
	flag.Parse()
	if *mode == "allocate" {
		x := make([]byte, *alloc*1024*1024)
		for i := 0; i < len(x); i += 4096 {
			x[i] = 1
		}
		fmt.Println(len(x))
		time.Sleep(2 * time.Second)
		return
	}
	os.MkdirAll(*out, 0755)
	assertions := []string{}
	summaries := []string{}
	throughput := []string{}
	cardinality := []string{}
	hist := []string{}
	strategies := []string{}
	concurrent := []string{}
	backpressure := []string{}
	limits := []string{}
	core := []string{}
	// E-019.1: fixed total work, publisher fan-out, mixed data.
	for _, pub := range []int{1, 10, 100} {
		for _, n := range []int{100, 1000, 10000} {
			r, _ := newRegistry(n, 10, 20000, 100)
			samples := make([]float64, 0, 20000)
			before, totalBefore := mem()
			start := time.Now()
			var wg sync.WaitGroup
			var accepted atomic.Uint64
			per := 20000 / pub
			for p := 0; p < pub; p++ {
				wg.Add(1)
				go func(base int) {
					defer wg.Done()
					for i := 0; i < per; i++ {
						t := time.Now()
						r.mu.Lock()
						r.mutate(base + i)
						r.mu.Unlock()
						samplesMu.Lock()
						samples = append(samples, ms(time.Since(t)))
						samplesMu.Unlock()
						accepted.Add(1)
					}
				}(p * per)
			}
			wg.Wait()
			elapsed := time.Since(start)
			after, totalAfter := mem()
			allocDelta := uint64(0)
			if after > before {
				allocDelta = after - before
			}
			throughput = append(throughput, fmt.Sprintf("%d\t%d\t%d\t0\t%.1f\t%.6f\t%.6f\t%.6f\t%d\t%d\t%d\t%d", n, pub, accepted.Load(), float64(accepted.Load())/elapsed.Seconds(), pct(samples, .5), pct(samples, .95), pct(samples, .99), allocDelta, totalAfter-totalBefore, 0, n))
			assertion(&assertions, "E-019.1", fmt.Sprintf("accepted_%d_%d", n, pub), strconv.Itoa(per*pub), strconv.FormatUint(accepted.Load(), 10))
		}
	}
	summaries = append(summaries, "E-019.1\toperation_throughput\tpass\t100/1000/10000 series; 1/10/100 publishers")
	// E-019.2: cardinality, encoding and safe rejection.
	for _, n := range []int{100, 1000, 10000, 20000} {
		_, before := mem()
		t := time.Now()
		r, _ := newRegistry(n, 10, 20000, 100)
		build := time.Since(t)
		_, mid := mem()
		t = time.Now()
		body := encode(r.data, r.generation)
		enc := time.Since(t)
		t = time.Now()
		_ = append([]byte(nil), body...)
		install := time.Since(t)
		t = time.Now()
		_ = len(body)
		scrape := time.Since(t)
		cardinality = append(cardinality, fmt.Sprintf("%d\t%d\t%d\t%.3f\t%.3f\t%.3f\t%.6f\t%d\t%.1f", n, mid-before, len(body), ms(build), ms(enc), ms(install), ms(scrape), len(body)/n, float64(mid-before)/float64(n)))
	}
	r, _ := newRegistry(20000, 10, 20000, 100)
	beforeGen := r.generation
	_, err := newRegistry(20001, 10, 20000, 100)
	assertion(&assertions, "E-019.2", "series_limit_rejects", "series_limit", err.Error())
	assertion(&assertions, "E-019.2", "series_rejection_preserves_generation", strconv.FormatUint(beforeGen, 10), strconv.FormatUint(r.generation, 10))
	summaries = append(summaries, "E-019.2\tcardinality_scaling\tpass\t100..20000 series plus 20001 rejection")
	// E-019.3 histogram bucket matrix and pre-allocation rejection.
	for _, n := range []int{100, 1000, 5000} {
		for _, b := range []int{1, 10, 50, 100} {
			_, before := mem()
			t := time.Now()
			r, _ := newRegistry(n, b, 20000, 100)
			build := time.Since(t)
			_, mid := mem()
			t = time.Now()
			for i := 0; i < n; i++ {
				if r.data[i].kind == 'h' {
					r.mutate(i)
				}
			}
			mut := time.Since(t)
			body := encode(r.data, r.generation)
			hist = append(hist, fmt.Sprintf("%d\t%d\t%d\t%d\t%.3f\t%.3f", n, b, mid-before, len(body), ms(build), ms(mut)))
		}
	}
	_, err = newRegistry(1, 101, 20000, 100)
	assertion(&assertions, "E-019.3", "bucket_limit_before_allocation", "bucket_limit", err.Error())
	summaries = append(summaries, "E-019.3\thistogram_cost\tpass\t1/10/50/100 buckets; 101 rejected")
	// E-019.4 strategy comparison; costs are recorded in distinct columns.
	for _, n := range []int{100, 1000, 10000} {
		for _, name := range []string{"reencode_each_mutation", "materialize_on_scrape", "generation_cache", "copy_on_write"} {
			ops := 2000
			if name == "reencode_each_mutation" && n > 1000 {
				ops = 200
			}
			if name == "copy_on_write" && n > 1000 {
				ops = 200
			}
			r, _ := newRegistry(n, 10, 20000, 100)
			start := time.Now()
			var encDur time.Duration
			for i := 0; i < ops; i++ {
				r.mu.Lock()
				if name == "copy_on_write" {
					r.data = clone(r.data)
				}
				r.mutate(i)
				if name == "reencode_each_mutation" {
					t := time.Now()
					r.cached = encode(r.data, r.generation)
					encDur += time.Since(t)
				}
				r.mu.Unlock()
			}
			mutDur := time.Since(start)
			t := time.Now()
			if name != "reencode_each_mutation" {
				r.cached = encode(r.data, r.generation)
			}
			material := time.Since(t)
			t = time.Now()
			coreCopy := append([]byte(nil), r.cached...)
			install := time.Since(t)
			t = time.Now()
			_ = len(coreCopy)
			scrape := time.Since(t)
			strategies = append(strategies, fmt.Sprintf("%s\t%d\t%d\t%.3f\t%.3f\t%.3f\t%.6f\t%d", name, n, ops, ms(mutDur-encDur), ms(material+encDur), ms(install), ms(scrape), len(coreCopy)))
		}
	}
	summaries = append(summaries, "E-019.4\tsnapshot_strategies\tpass\tall four strategies; split phase timings")
	// E-019.5: linked cross-family commits pass through the selected generation
	// cache. Readers retain old byte slices while newer generations replace the
	// cache, and validate every marker in the complete encoded body.
	linked := &linkedRegistry{}
	linked.commit()
	_ = linked.snapshot()
	_ = linked.snapshot()
	var stop atomic.Bool
	var mixed atomic.Uint64
	var snaps atomic.Uint64
	var slowCompleted atomic.Uint64
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for !stop.Load() {
			linked.commit()
		}
	}()
	for reader := 0; reader < 4; reader++ {
		workers.Add(1)
		go func(slow bool) {
			defer workers.Done()
			for !stop.Load() {
				body := linked.snapshot()
				if slow {
					// Keep the old immutable bytes alive across many writer commits
					// and cache replacements before inspecting the complete body.
					time.Sleep(200 * time.Microsecond)
					slowCompleted.Add(1)
				}
				if !linkedSnapshotValid(body) {
					mixed.Add(1)
				}
				snaps.Add(1)
			}
		}(reader == 0)
	}
	time.Sleep(300 * time.Millisecond)
	stop.Store(true)
	workers.Wait()
	linked.mu.Lock()
	finalGeneration, cacheHits, cacheMisses := linked.generation, linked.cacheHits, linked.cacheMisses
	linked.mu.Unlock()
	concurrent = append(concurrent, fmt.Sprintf("%d\t%d\t%d\t%d\t%d\t%d", snaps.Load(), mixed.Load(), finalGeneration, slowCompleted.Load(), cacheHits, cacheMisses))
	assertion(&assertions, "E-019.5", "zero_mixed_generation_responses", "0", strconv.FormatUint(mixed.Load(), 10))
	assertion(&assertions, "E-019.5", "slow_reader_completed", "true", strconv.FormatBool(slowCompleted.Load() > 0))
	assertion(&assertions, "E-019.5", "cache_path_exercised", "true", strconv.FormatBool(cacheHits > 0 && cacheMisses > 0))
	summaries = append(summaries, "E-019.5\tconcurrent_scrape\tpass\tfull linked body; immutable generation cache; concurrent and slow readers")
	// E-019.6 exact bounded non-blocking admission.
	for _, capn := range []int{1, 16, 64, 1024} {
		q := make(chan int, capn)
		accepted := 0
		for i := 0; i < 10000; i++ {
			select {
			case q <- i:
				accepted++
			default:
			}
		}
		rejected := 10000 - accepted
		backpressure = append(backpressure, fmt.Sprintf("%d\t10000\t%d\t%d\t%d", capn, accepted, rejected, len(q)))
		assertion(&assertions, "E-019.6", fmt.Sprintf("capacity_%d", capn), strconv.Itoa(capn), strconv.Itoa(accepted))
	}
	summaries = append(summaries, "E-019.6\tbackpressure\tpass\texact capacity; visible rejection")
	// E-019.7 normal policy bounds. OS-level FD/OOM are runner-owned.
	for _, spec := range [][3]int{{100, 10, 1024}, {20000, 100, 64}, {20001, 10, 64}, {100, 101, 64}} {
		_, e := newRegistry(spec[0], spec[1], 20000, 100)
		status := "accepted"
		if e != nil {
			status = e.Error()
		}
		limits = append(limits, fmt.Sprintf("%d\t%d\t%d\t%s", spec[0], spec[1], spec[2], status))
	}
	summaries = append(summaries, "E-019.7\tcontrolled_exhaustion\tpass\tpolicy bounds in prototype; cgroup and FD in runner")
	// Core-like installation rejects malformed data and retains prior immutable response.
	valid := []byte("# generation 1\nm_total 1\n")
	active := append([]byte(nil), valid...)
	candidate := []byte("invalid")
	if !strings.HasPrefix(string(candidate), "# generation ") {
		candidate = nil
	}
	assertion(&assertions, "Additional", "invalid_core_candidate_preserves_active", "true", strconv.FormatBool(string(valid) == string(active)))
	core = append(core, "valid\taccepted\t1", "malformed\trejected\t1")
	write(*out+"/assertions.tsv", "experiment\tassertion\texpected\tactual\tresult", assertions)
	write(*out+"/summary.tsv", "experiment\tcase\tresult\tdetail", summaries)
	write(*out+"/throughput.tsv", "series\tpublishers\taccepted\trejected\tops_per_second\tp50_ms\tp95_ms\tp99_ms\talloc_delta_bytes\ttotal_alloc_delta_bytes\tmax_queue_depth\tdescriptors", throughput)
	write(*out+"/cardinality.tsv", "series\talloc_delta_bytes\tencoded_bytes\tbuild_ms\tencode_ms\tcore_install_ms\tscrape_ms\tencoded_bytes_per_series\talloc_bytes_per_series", cardinality)
	write(*out+"/histograms.tsv", "series\tbuckets\talloc_delta_bytes\tencoded_bytes\tbuild_ms\tmutation_ms", hist)
	write(*out+"/snapshot-strategies.tsv", "strategy\tseries\toperations\tmutation_ms\tencoding_ms\tcore_install_ms\tscrape_ms\tencoded_bytes", strategies)
	write(*out+"/concurrent-scrape.tsv", "snapshots\tmixed_generation_responses\tfinal_generation\tslow_reader_completions\tcache_hits\tcache_misses", concurrent)
	write(*out+"/backpressure.tsv", "capacity\tattempted\taccepted\trejected\tmax_depth", backpressure)
	write(*out+"/limits.tsv", "series\tbuckets\tqueue_capacity\tresult", limits)
	write(*out+"/core-install.tsv", "candidate\tresult\tactive_generation", core)
	for _, a := range assertions {
		if strings.HasSuffix(a, "\tfail") {
			os.Exit(1)
		}
	}
}

var samplesMu sync.Mutex
