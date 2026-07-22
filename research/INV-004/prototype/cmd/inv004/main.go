package main

import (
	"flag"
	"fmt"
	"runtime"
	"sort"
	"time"
)

type sample struct {
	typ   string
	value float64
	hist  *histogram
}
type histogram struct {
	bounds  []float64
	buckets []uint64
	count   uint64
	sum     float64
}
type registry map[string]sample
type ownerState struct {
	epoch, seq uint64
	values     registry
}
type operationState struct {
	epoch         uint64
	lastSeq       uint64
	incomplete    bool
	authoritative bool
}

func copyRegistry(in registry) registry {
	out := registry{}
	for k, v := range in {
		if v.hist != nil {
			v.hist = &histogram{append([]float64(nil), v.hist.bounds...), append([]uint64(nil), v.hist.buckets...), v.hist.count, v.hist.sum}
		}
		out[k] = v
	}
	return out
}

func snapshot(dst map[string]ownerState, operations map[string]operationState, owner string, epoch, seq uint64, values registry) bool {
	old, exists := dst[owner]
	if exists && (epoch < old.epoch || (epoch == old.epoch && seq <= old.seq)) {
		return false
	}
	for _, next := range values {
		if next.typ == "histogram" && !validHistogram(next.hist) {
			return false
		}
	}
	if exists && epoch == old.epoch {
		for key, next := range values {
			previous, present := old.values[key]
			if !present {
				continue
			}
			if previous.typ != next.typ || (next.typ == "counter" && next.value < previous.value) ||
				(next.typ == "histogram" && !histogramProgresses(previous.hist, next.hist)) {
				return false
			}
		}
	}
	dst[owner] = ownerState{epoch, seq, copyRegistry(values)}
	operations[owner] = operationState{epoch: epoch, lastSeq: seq, incomplete: false, authoritative: true}
	return true
}

func validHistogram(h *histogram) bool {
	if h == nil || len(h.bounds) == 0 || len(h.bounds) != len(h.buckets) || h.count != h.buckets[len(h.buckets)-1] {
		return false
	}
	for i := range h.bounds {
		if i > 0 && (h.bounds[i] <= h.bounds[i-1] || h.buckets[i] < h.buckets[i-1]) {
			return false
		}
	}
	return true
}

func sameBounds(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func histogramProgresses(old, next *histogram) bool {
	if !validHistogram(old) || !validHistogram(next) || !sameBounds(old.bounds, next.bounds) || next.count < old.count {
		return false
	}
	for i := range old.buckets {
		if next.buckets[i] < old.buckets[i] {
			return false
		}
	}
	return true
}

func operation(dst registry, owners map[string]operationState, owner string, epoch, seq uint64, key, typ string, delta float64) bool {
	state, exists := owners[owner]
	if !exists || epoch > state.epoch {
		owners[owner] = operationState{epoch: epoch, incomplete: true, authoritative: false}
		return false
	}
	if epoch < state.epoch || !state.authoritative || seq <= state.lastSeq {
		return false
	}
	v := dst[key]
	if v.typ != "" && v.typ != typ {
		return false
	}
	if state.lastSeq > 0 && seq > state.lastSeq+1 {
		state.incomplete = true
	}
	state.lastSeq = seq
	owners[owner] = state
	v.typ = typ
	if typ == "counter" {
		v.value += delta
	} else {
		v.value = delta
	}
	dst[key] = v
	return true
}

func initialSnapshot(owners map[string]operationState, owner string, epoch uint64) {
	snapshot(map[string]ownerState{}, owners, owner, epoch, 0, registry{})
}

func aggregateHistogram(states map[string]ownerState, key string) (histogram, bool) {
	var result histogram
	found := false
	for _, owner := range states {
		v, exists := owner.values[key]
		if !exists {
			continue
		}
		if v.typ != "histogram" || !validHistogram(v.hist) {
			return histogram{}, false
		}
		if !found {
			result = histogram{append([]float64(nil), v.hist.bounds...), append([]uint64(nil), v.hist.buckets...), v.hist.count, v.hist.sum}
			found = true
			continue
		}
		if !sameBounds(result.bounds, v.hist.bounds) {
			return histogram{}, false
		}
		for i := range result.buckets {
			result.buckets[i] += v.hist.buckets[i]
		}
		result.count += v.hist.count
		result.sum += v.hist.sum
	}
	return result, found && validHistogram(&result)
}

func aggregate(states map[string]ownerState, key, gaugePolicy string) (float64, bool) {
	var total float64
	var first sample
	count := 0
	for _, owner := range states {
		v, exists := owner.values[key]
		if !exists {
			continue
		}
		if count == 0 {
			first = v
		} else if first.typ != v.typ {
			return 0, false
		}
		if v.typ == "histogram" {
			return 0, false
		}
		count++
		if v.typ == "gauge" {
			switch gaugePolicy {
			case "sum":
				total += v.value
			case "max":
				if count == 1 || v.value > total {
					total = v.value
				}
			default:
				if count > 1 {
					return 0, false
				}
				total = v.value
			}
		} else {
			total += v.value
		}
	}
	return total, count > 0
}

func row(name, candidate, topic string, expected, actual any, pass bool) {
	result := "fail"
	if pass {
		result = "pass"
	}
	fmt.Printf("%s\t%s\t%s\t%v\t%v\t%s\n", name, candidate, topic, expected, actual, result)
}

func scenarios() {
	fmt.Println("scenario\tcandidate\ttopic\texpected\tactual\tresult")
	states := map[string]ownerState{}
	opStates := map[string]operationState{}
	snapshot(states, opStates, "p1", 1, 1, registry{"jobs_total": {typ: "counter", value: 10}})
	snapshot(states, opStates, "p1", 1, 3, registry{"jobs_total": {typ: "counter", value: 30}})
	n, ok := aggregate(states, "jobs_total", "")
	row("snapshot_drop_recovery", "snapshot", "dropped update", 30, n, ok && n == 30)
	snapshot(states, opStates, "p1", 2, 1, registry{"jobs_total": {typ: "counter", value: 2}})
	n, ok = aggregate(states, "jobs_total", "")
	row("snapshot_producer_restart", "snapshot", "producer epoch", 2, n, ok && n == 2)
	snapshot(states, opStates, "p1", 2, 2, registry{})
	_, stale := states["p1"].values["jobs_total"]
	row("snapshot_stale_removal", "snapshot", "stale series", false, stale, !stale)

	states, opStates = map[string]ownerState{}, map[string]operationState{}
	snapshot(states, opStates, "p1", 1, 1, registry{"jobs_total": {typ: "counter", value: 5}})
	snapshot(states, opStates, "p2", 1, 1, registry{"jobs_total": {typ: "counter", value: 7}})
	n, ok = aggregate(states, "jobs_total", "")
	row("snapshot_multi_producer_counter", "snapshot", "counter aggregation", 12, n, ok && n == 12)
	accepted := snapshot(states, opStates, "p1", 1, 1, registry{"jobs_total": {typ: "counter", value: 99}})
	row("snapshot_duplicate_sequence", "snapshot", "ordering", false, accepted, !accepted)

	states, opStates = map[string]ownerState{}, map[string]operationState{}
	snapshot(states, opStates, "p1", 1, 1, registry{"jobs_total": {typ: "counter", value: 10}})
	accepted = snapshot(states, opStates, "p1", 1, 2, registry{"jobs_total": {typ: "counter", value: 7}})
	row("snapshot_counter_decrease_rejected", "snapshot", "counter monotonicity", false, accepted, !accepted)
	accepted = snapshot(states, opStates, "p1", 1, 2, registry{"jobs_total": {typ: "gauge", value: 10}})
	row("snapshot_type_change_rejected", "snapshot", "type stability", false, accepted, !accepted)
	snapshot(states, opStates, "p1", 2, 1, registry{"jobs_total": {typ: "counter", value: 2}})
	n, ok = aggregate(states, "jobs_total", "")
	row("snapshot_new_epoch_lower_counter", "snapshot", "epoch reset", 2, n, ok && n == 2)

	states, opStates = map[string]ownerState{}, map[string]operationState{}
	h1 := &histogram{bounds: []float64{1, 5, 10}, buckets: []uint64{2, 5, 6}, count: 6, sum: 20}
	snapshot(states, opStates, "p1", 1, 1, registry{"latency": {typ: "histogram", hist: h1}})
	hSchema := &histogram{bounds: []float64{1, 10}, buckets: []uint64{3, 7}, count: 7, sum: 21}
	accepted = snapshot(states, opStates, "p1", 1, 2, registry{"latency": {typ: "histogram", hist: hSchema}})
	row("snapshot_histogram_schema_change_rejected", "snapshot", "histogram schema", false, accepted, !accepted)
	hDecrease := &histogram{bounds: []float64{1, 5, 10}, buckets: []uint64{3, 4, 7}, count: 7, sum: 22}
	accepted = snapshot(states, opStates, "p1", 1, 2, registry{"latency": {typ: "histogram", hist: hDecrease}})
	row("snapshot_histogram_cumulative_decrease_rejected", "snapshot", "histogram monotonicity", false, accepted, !accepted)
	hReset := &histogram{bounds: []float64{1, 5, 10}, buckets: []uint64{0, 1, 2}, count: 2, sum: 4}
	accepted = snapshot(states, opStates, "p1", 2, 1, registry{"latency": {typ: "histogram", hist: hReset}})
	row("snapshot_histogram_new_epoch_reset", "snapshot", "histogram epoch reset", true, accepted, accepted)
	h2 := &histogram{bounds: []float64{1, 5, 10}, buckets: []uint64{1, 3, 4}, count: 4, sum: 12}
	snapshot(states, opStates, "p2", 1, 1, registry{"latency": {typ: "histogram", hist: h2}})
	hTotal, ok := aggregateHistogram(states, "latency")
	histogramCorrect := ok && hTotal.count == 6 && hTotal.sum == 16 && len(hTotal.buckets) == 3 && hTotal.buckets[0] == 1 && hTotal.buckets[1] == 4 && hTotal.buckets[2] == 6
	row("multi_histogram_compatible", "snapshot", "component-wise histogram aggregation", "buckets=1,4,6 count=6 sum=16", fmt.Sprintf("buckets=%d,%d,%d count=%d sum=%.0f", hTotal.buckets[0], hTotal.buckets[1], hTotal.buckets[2], hTotal.count, hTotal.sum), histogramCorrect)
	hMismatch := &histogram{bounds: []float64{1, 10}, buckets: []uint64{1, 2}, count: 2, sum: 5}
	snapshot(states, opStates, "p3", 1, 1, registry{"latency": {typ: "histogram", hist: hMismatch}})
	_, ok = aggregateHistogram(states, "latency")
	row("multi_histogram_schema_mismatch_rejected", "snapshot", "histogram collision", false, ok, !ok)

	states, opStates = map[string]ownerState{}, map[string]operationState{}
	snapshot(states, opStates, "p1", 1, 1, registry{"shared": {typ: "gauge", value: 1}})
	snapshot(states, opStates, "p2", 1, 1, registry{"shared": {typ: "gauge", value: 2}})
	_, ok = aggregate(states, "shared", "")
	row("multi_gauge_without_policy_rejected", "snapshot", "gauge collision", false, ok, !ok)
	n, ok = aggregate(states, "shared", "sum")
	row("multi_gauge_sum_policy", "snapshot", "explicit gauge policy", 3, n, ok && n == 3)
	states["p2"] = ownerState{1, 2, registry{"shared": {typ: "counter", value: 2}}}
	_, ok = aggregate(states, "shared", "sum")
	row("multi_owner_type_conflict_rejected", "snapshot", "owner type conflict", false, ok, !ok)

	abs := registry{"queue_depth": {typ: "gauge", value: 5}}
	abs["queue_depth"] = sample{typ: "gauge", value: 9}
	row("absolute_drop_recovery", "absolute", "later absolute value", 9, abs["queue_depth"].value, abs["queue_depth"].value == 9)
	abs["jobs_total"] = sample{typ: "counter", value: 10}
	abs["jobs_total"] = sample{typ: "counter", value: 7}
	row("absolute_counter_decrease", "absolute", "counter monotonicity", ">=10", abs["jobs_total"].value, abs["jobs_total"].value >= 10)
	abs["shared"] = sample{typ: "gauge", value: 1}
	abs["shared"] = sample{typ: "gauge", value: 2}
	row("absolute_multi_producer_collision", "absolute", "last writer wins", 3, abs["shared"].value, abs["shared"].value == 3)

	ops, owners := registry{}, map[string]operationState{}
	initialSnapshot(owners, "p1", 1)
	operation(ops, owners, "p1", 1, 1, "jobs_total", "counter", 1)
	operation(ops, owners, "p1", 1, 3, "jobs_total", "counter", 1)
	row("operations_drop", "operations", "lost increment", 3, ops["jobs_total"].value, ops["jobs_total"].value == 3)
	row("operations_gap_detected", "operations", "sequence gap", true, owners["p1"].incomplete, owners["p1"].incomplete)
	accepted = operation(ops, owners, "p1", 1, 2, "jobs_total", "counter", 1)
	row("operations_late_does_not_repair_gap", "operations", "late operation", false, accepted || !owners["p1"].incomplete, !accepted && owners["p1"].incomplete)
	duplicateOps, duplicateOwners := registry{}, map[string]operationState{}
	initialSnapshot(duplicateOwners, "p1", 1)
	operation(duplicateOps, duplicateOwners, "p1", 1, 1, "jobs_total", "counter", 1)
	before := duplicateOps["jobs_total"].value
	accepted = operation(duplicateOps, duplicateOwners, "p1", 1, 1, "jobs_total", "counter", 1)
	row("operations_duplicate_no_gap", "operations", "duplicate", false, accepted, !accepted && !duplicateOwners["p1"].incomplete && before == duplicateOps["jobs_total"].value)

	ops, owners = registry{}, map[string]operationState{}
	initialSnapshot(owners, "p1", 1)
	operation(ops, owners, "p1", 1, 1, "jobs_total", "counter", 1)
	accepted = operation(ops, owners, "p1", 1, 2, "jobs_total", "gauge", 9)
	conflictRejected := !accepted && owners["p1"].lastSeq == 1
	accepted = operation(ops, owners, "p1", 1, 2, "jobs_total", "counter", 1)
	row("operations_conflict_sequence_reusable", "operations", "transactional sequence", 2, ops["jobs_total"].value, conflictRejected && accepted && ops["jobs_total"].value == 2)
	ops = registry{}
	owners = map[string]operationState{}
	initialSnapshot(owners, "p1", 1)
	operation(ops, owners, "p1", 1, 1, "jobs_total", "counter", 5)
	ops, owners = registry{}, map[string]operationState{}
	row("operations_receiver_restart", "operations", "state recovery", 5, ops["jobs_total"].value, ops["jobs_total"].value == 5)
	initialSnapshot(owners, "p1", 1)
	initialSnapshot(owners, "p2", 1)
	operation(ops, owners, "p1", 1, 1, "shared_total", "counter", 2)
	operation(ops, owners, "p2", 1, 1, "shared_total", "counter", 3)
	row("operations_multi_producer", "operations", "commutative increments", 5, ops["shared_total"].value, ops["shared_total"].value == 5)

	epochOps, epochOwners, epochSnapshots := registry{}, map[string]operationState{}, map[string]ownerState{}
	snapshot(epochSnapshots, epochOwners, "p1", 1, 0, registry{})
	operation(epochOps, epochOwners, "p1", 1, 100, "jobs_total", "counter", 1)
	accepted = operation(epochOps, epochOwners, "p1", 0, 101, "jobs_total", "counter", 1)
	row("operation_old_epoch_rejected", "operations", "producer epoch", false, accepted, !accepted && epochOwners["p1"].epoch == 1)
	accepted = operation(epochOps, epochOwners, "p1", 2, 1, "jobs_total", "counter", 1)
	row("operation_new_epoch_requires_snapshot", "operations", "initial snapshot gate", false, accepted, !accepted && epochOwners["p1"].epoch == 2 && epochOwners["p1"].incomplete && !epochOwners["p1"].authoritative)
	snapshot(epochSnapshots, epochOwners, "p1", 2, 0, registry{"jobs_total": {typ: "counter", value: 0}})
	epochOps = copyRegistry(epochSnapshots["p1"].values)
	accepted = operation(epochOps, epochOwners, "p1", 2, 1, "jobs_total", "counter", 1)
	row("operation_after_new_epoch_snapshot_accepted", "operations", "initial snapshot gate", 1, epochOps["jobs_total"].value, accepted && epochOps["jobs_total"].value == 1 && epochOwners["p1"].authoritative && !epochOwners["p1"].incomplete)

	// Hybrid: a gap marks fast-path state incomplete; an authoritative snapshot repairs it.
	ops, owners, states = registry{}, map[string]operationState{}, map[string]ownerState{}
	snapshot(states, owners, "p1", 1, 0, registry{})
	operation(ops, owners, "p1", 1, 1, "jobs_total", "counter", 1)
	operation(ops, owners, "p1", 1, 3, "jobs_total", "counter", 1)
	snapshot(states, owners, "p1", 1, 4, registry{"jobs_total": {typ: "counter", value: 3}, "queue_depth": {typ: "gauge", value: 4}})
	ops = copyRegistry(states["p1"].values)
	row("hybrid_snapshot_clears_incomplete", "hybrid", "snapshot reconciliation", false, owners["p1"].incomplete, !owners["p1"].incomplete)
	row("hybrid_loss_reconciliation", "hybrid", "snapshot repair", 3, ops["jobs_total"].value, ops["jobs_total"].value == 3)
	ops, owners = registry{}, map[string]operationState{}
	snapshot(states, owners, "p1", 2, 1, registry{"jobs_total": {typ: "counter", value: 3}})
	ops = copyRegistry(states["p1"].values)
	row("hybrid_receiver_restart", "hybrid", "snapshot recovery", 3, ops["jobs_total"].value, ops["jobs_total"].value == 3)
	_, stale = ops["queue_depth"]
	row("hybrid_stale_removal", "hybrid", "complete owner snapshot", false, stale, !stale)
}

func benchmark(candidate string, producers, series, updates, interval int) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	checksum := 0.0
	reconciliations := 0
	reconciliationTime := time.Duration(0)
	if candidate == "snapshot" {
		states, owners := map[string]ownerState{}, map[string]operationState{}
		for u := 0; u < updates; u++ {
			p, r := fmt.Sprintf("p%d", u%producers), registry{}
			for j := 0; j < series; j++ {
				r[fmt.Sprintf("m%d", j)] = sample{typ: "counter", value: float64(u + j)}
			}
			snapshot(states, owners, p, 1, uint64(u/producers+1), r)
		}
		for _, v := range states {
			checksum += v.values["m0"].value
		}
	} else {
		r, owners := registry{}, map[string]operationState{}
		states := map[string]ownerState{}
		for p := 0; p < producers; p++ {
			owner := fmt.Sprintf("p%d", p)
			snapshot(states, owners, owner, 1, 0, registry{})
		}
		for u := 0; u < updates; u++ {
			p := fmt.Sprintf("p%d", u%producers)
			operation(r, owners, p, 1, uint64(u/producers+1), fmt.Sprintf("m%d", u%series), "counter", 1)
			if candidate == "hybrid_amortized" && (u+1)%interval == 0 {
				reconcileStart := time.Now()
				snapshot(states, owners, p, 1, uint64(u/producers+1), r)
				reconciliations++
				reconciliationTime += time.Since(reconcileStart)
			}
		}
		for _, v := range r {
			checksum += v.value
		}
	}
	runtime.ReadMemStats(&after)
	elapsed := time.Since(start)
	alloc := after.TotalAlloc - before.TotalAlloc
	share := 0.0
	if elapsed > 0 {
		share = 100 * float64(reconciliationTime) / float64(elapsed)
	}
	fmt.Printf("%s\t%d\t%d\t%d\t%d\t%.3f\t%.0f\t%d\t%.0f\t%d\t%d\t%.3f\n", candidate, producers, series, updates, elapsed.Microseconds(), float64(updates)/elapsed.Seconds(), checksum, alloc, float64(alloc)/float64(updates), interval, reconciliations, share)
}

func main() {
	mode := flag.String("mode", "scenarios", "scenarios or benchmark")
	candidate := flag.String("candidate", "operations", "snapshot, operations, or hybrid_amortized")
	producers := flag.Int("producers", 1, "")
	series := flag.Int("series", 100, "")
	updates := flag.Int("updates", 100000, "")
	interval := flag.Int("reconciliation-interval", 1000, "operations between hybrid snapshots")
	flag.Parse()
	if *mode == "scenarios" {
		scenarios()
		return
	}
	valid := []string{"hybrid_amortized", "operations", "snapshot"}
	sort.Strings(valid)
	ok := false
	for _, v := range valid {
		if v == *candidate {
			ok = true
		}
	}
	if !ok || *interval < 1 {
		panic("invalid benchmark configuration")
	}
	benchmark(*candidate, *producers, *series, *updates, *interval)
}
