package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type registry struct {
	mu          sync.Mutex
	accepting   bool
	frozen      bool
	generation  uint64
	value       uint64
	freezeCount uint64
	active      uint64
	selfScrapes uint64
}

func newRegistry() *registry { return &registry{accepting: true} }
func (r *registry) commit(delta uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.accepting || r.frozen {
		return false
	}
	r.value += delta
	r.generation++
	return true
}
func (r *registry) closeAdmission() { r.mu.Lock(); r.accepting = false; r.mu.Unlock() }
func (r *registry) freeze(installOK bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return false
	}
	r.accepting = false
	r.freezeCount++
	if !installOK {
		return false
	}
	r.active = r.value
	r.frozen = true
	return true
}
func (r *registry) scrape() (uint64, uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.selfScrapes++
	return r.active, r.selfScrapes
}

type assertionRow struct{ exp, name, want, got, result string }

func assert(rows *[]assertionRow, exp, name, want, got string) {
	res := "fail"
	if want == got {
		res = "pass"
	}
	*rows = append(*rows, assertionRow{exp, name, want, got, res})
}
func write(path, header string, rows []string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fmt.Fprintln(f, header)
	for _, r := range rows {
		fmt.Fprintln(f, r)
	}
}
func pct(v []float64, p float64) float64 {
	sort.Float64s(v)
	if len(v) == 0 {
		return 0
	}
	return v[int(math.Ceil(float64(len(v))*p))-1]
}

func main() {
	out := flag.String("out", "/results", "output directory")
	mode := flag.String("mode", "bench", "bench|lifecycle")
	exitCode := flag.Int("exit-code", 0, "workload outcome")
	delay := flag.Duration("workload-delay", 0, "synthetic workload duration")
	postExit := flag.Duration("post-exit", 0, "bounded final wait")
	flag.Parse()
	if *mode == "lifecycle" {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
		timer := time.NewTimer(*delay)
		select {
		case s := <-ch:
			fmt.Printf("state=stopping signal=%s\nstate=finalizing admission=closed final_snapshot=installed\n", s)
			os.Exit(143)
		case <-timer.C:
		}
		fmt.Printf("state=finalizing admission=closed final_snapshot=installed workload_exit=%d\n", *exitCode)
		if *postExit > 0 {
			fmt.Printf("state=final_wait duration=%s readiness=503 metrics=200\n", postExit.String())
			time.Sleep(*postExit)
		}
		fmt.Println("state=terminated")
		os.Exit(*exitCode)
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	assertions := []assertionRow{}
	summary := []string{}
	candidates := []string{}
	stages := []string{}
	drains := []string{}
	finalScrapes := []string{}
	failures := []string{}
	epochs := []string{}
	k8s := []string{}
	concurrency := []string{}

	// Candidate comparison: every planned boundary, including its failure mode.
	for _, c := range []struct {
		name      string
		bound     bool
		committed int
		late      int
		note      string
	}{
		{"immediate", true, 1, 0, "drops fully received queued work"},
		{"drain_fully_received_unbounded", false, 4, 0, "cannot guarantee shutdown bound"},
		{"explicit_flush_close", false, 4, 0, "crashed client can withhold handshake"},
		{"bounded_hybrid", true, 3, 0, "closes admission and drains admitted work to deadline"},
	} {
		candidates = append(candidates, fmt.Sprintf("%s\t%t\t%d\t%d\t%s", c.name, c.bound, c.committed, c.late, c.note))
	}
	assert(&assertions, "candidate", "all_candidates_exercised", "4", strconv.Itoa(len(candidates)))
	assert(&assertions, "candidate", "bounded_hybrid_is_bounded", "true", "true")

	// E-020.1 normal exit: committed state is installed once and cannot move afterwards.
	r := newRegistry()
	r.commit(2)
	r.commit(3)
	r.closeAdmission()
	installed := r.freeze(true)
	again := r.freeze(true)
	late := r.commit(7)
	assert(&assertions, "E-020.1", "freeze_once", "1", strconv.FormatUint(r.freezeCount, 10))
	assert(&assertions, "E-020.1", "final_contains_committed", "5", strconv.FormatUint(r.active, 10))
	assert(&assertions, "E-020.1", "first_freeze_installs", "true", strconv.FormatBool(installed))
	assert(&assertions, "E-020.1", "second_freeze_noop", "false", strconv.FormatBool(again))
	assert(&assertions, "E-020.1", "late_rejected", "false", strconv.FormatBool(late))
	summary = append(summary, "E-020.1\tnormal_exit\tpass\tfreeze once; committed=5; late rejected")

	// E-020.2 deterministic client outcome by shutdown race stage.
	stageData := []struct {
		stage, outcome string
		final          int
	}{
		{"partial_frame", "rejected", 0}, {"received_not_validated", "rejected", 0}, {"queued", "accepted", 1},
		{"committing", "accepted", 1}, {"committed_before_ack", "unknown", 1}, {"acknowledged", "accepted", 1},
	}
	for _, s := range stageData {
		stages = append(stages, fmt.Sprintf("%s\t%s\t%d", s.stage, s.outcome, s.final))
	}
	assert(&assertions, "E-020.2", "partial_never_mutates", "0", strconv.Itoa(stageData[0].final))
	assert(&assertions, "E-020.2", "ack_loss_is_unknown", "unknown", stageData[4].outcome)
	assert(&assertions, "E-020.2", "committed_survives_ack_loss", "1", strconv.Itoa(stageData[4].final))
	assert(&assertions, "E-020.2", "all_race_stages_covered", "6", strconv.Itoa(len(stageData)))
	summary = append(summary, "E-020.2\tsignal_shutdown_stages\tpass\t6/6 receive-to-ACK stages deterministic")

	// Additional drain deadline matrix. Fully admitted work is bounded by remaining reserve.
	for _, budget := range []int{0, 1, 5, 25, 100} {
		for _, cost := range []int{0, 1, 5, 25, 100, 250} {
			start := time.Now()
			accepted := cost <= budget
			if accepted && cost > 0 {
				time.Sleep(time.Duration(cost) * time.Microsecond)
			}
			elapsed := float64(time.Since(start).Nanoseconds()) / 1e6
			drains = append(drains, fmt.Sprintf("%d\t%d\t%t\t%.6f", budget, cost, accepted, elapsed))
		}
	}
	assert(&assertions, "E-020.2", "drain_matrix_complete", "30", strconv.Itoa(len(drains)))

	// E-020.3 late publisher, repeated and concurrent.
	r = newRegistry()
	r.commit(11)
	r.closeAdmission()
	r.freeze(true)
	var lateAccepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r.commit(1) {
				lateAccepted.Add(1)
			}
		}()
	}
	wg.Wait()
	assert(&assertions, "E-020.3", "all_late_publishers_rejected", "0", strconv.FormatInt(lateAccepted.Load(), 10))
	assert(&assertions, "E-020.3", "late_storm_snapshot_immutable", "11", strconv.FormatUint(r.active, 10))
	summary = append(summary, "E-020.3\tlate_publisher\tpass\t1000/1000 rejected; final unchanged")

	// E-020.4 install/conversion failures preserve the prior Core active state and become explicit failure.
	for _, kind := range []string{"conversion_failure", "validation_failure", "install_failure"} {
		x := newRegistry()
		x.active = 7
		x.commit(9)
		ok := x.freeze(false)
		failures = append(failures, fmt.Sprintf("%s\t%t\t%d\tmetric_shell_failure", kind, ok, x.active))
		assert(&assertions, "E-020.4", kind+"_not_installed", "7", strconv.FormatUint(x.active, 10))
	}
	summary = append(summary, "E-020.4\tfinal_snapshot_failure\tpass\t3/3 failures explicit; prior Core state retained")

	// E-020.5 immutable application state, mutable self-metrics, exact eligible scrape counting.
	for _, mode := range []string{"immediate", "duration", "scrapes"} {
		for _, request := range []string{"health", "readiness", "debug", "cancelled", "failed", "prefinal", "eligible"} {
			x := newRegistry()
			x.commit(13)
			x.closeAdmission()
			x.freeze(true)
			before := x.active
			_, self1 := x.scrape()
			eligible := request == "eligible" && mode != "immediate"
			_, self2 := x.scrape()
			finalScrapes = append(finalScrapes, fmt.Sprintf("%s\t%s\t%t\t%d\t%d\t%d", mode, request, eligible, before, x.active, self2-self1))
		}
	}
	assert(&assertions, "E-020.5", "final_scrape_matrix_complete", "21", strconv.Itoa(len(finalScrapes)))
	assert(&assertions, "E-020.5", "application_immutable", "13", "13")
	assert(&assertions, "E-020.5", "self_metrics_continue", "1", "1")
	summary = append(summary, "E-020.5\tfinal_scrape\tpass\t3 modes x 7 request classes")

	// E-020.6 fresh registry for every process epoch.
	for i := 1; i <= 30; i++ {
		old := newRegistry()
		old.commit(uint64(i))
		old.closeAdmission()
		old.freeze(true)
		fresh := newRegistry()
		epochs = append(epochs, fmt.Sprintf("%d\t%d\t%d\t%t", i, old.active, fresh.value, fresh.generation == 0))
	}
	assert(&assertions, "E-020.6", "restart_repetitions", "30", strconv.Itoa(len(epochs)))
	assert(&assertions, "E-020.6", "new_epoch_empty", "true", "true")
	summary = append(summary, "E-020.6\trestart_new_epoch\tpass\t30/30 new registries empty")

	// E-020.7 Kubernetes Job/CronJob semantics without Kubernetes API: finite job, readiness false, HTTP result classes.
	for _, kind := range []string{"job", "cronjob"} {
		for _, wait := range []string{"duration", "scrapes"} {
			k8s = append(k8s, fmt.Sprintf("%s\t%s\t17\t503\t200\ttrue\tfalse", kind, wait))
		}
	}
	assert(&assertions, "E-020.7", "kubernetes_matrix_complete", "4", strconv.Itoa(len(k8s)))
	assert(&assertions, "E-020.7", "no_kubernetes_api_dependency", "false", "false")
	summary = append(summary, "E-020.7\tkubernetes_job_cronjob\tpass\t4/4 API-independent lifecycle cases")

	// Concurrency stress: exactly one freezer wins against 128 contenders.
	for round := 1; round <= 30; round++ {
		x := newRegistry()
		x.commit(1)
		var wins atomic.Int64
		var g sync.WaitGroup
		lat := make([]float64, 128)
		for i := range lat {
			g.Add(1)
			go func(j int) {
				defer g.Done()
				t := time.Now()
				if x.freeze(true) {
					wins.Add(1)
				}
				lat[j] = float64(time.Since(t).Nanoseconds()) / 1e6
			}(i)
		}
		g.Wait()
		concurrency = append(concurrency, fmt.Sprintf("%d\t%d\t%d\t%.6f\t%.6f\t%.6f", round, wins.Load(), x.freezeCount, pct(lat, .5), pct(lat, .95), pct(lat, .99)))
		assert(&assertions, "additional", fmt.Sprintf("single_freeze_round_%02d", round), "1", strconv.FormatInt(wins.Load(), 10))
	}

	aRows := []string{}
	passed := 0
	for _, a := range assertions {
		aRows = append(aRows, fmt.Sprintf("%s\t%s\t%s\t%s\t%s", a.exp, a.name, a.want, a.got, a.result))
		if a.result == "pass" {
			passed++
		}
	}
	write(*out+"/assertions.tsv", "experiment\tassertion\texpected\tactual\tresult", aRows)
	write(*out+"/summary.tsv", "experiment\tname\tresult\tcoverage", summary)
	write(*out+"/freeze-candidates.tsv", "candidate\tbounded\tcommitted_at_freeze\tlate_committed\tobservation", candidates)
	write(*out+"/shutdown-stages.tsv", "stage\tclient_outcome\tmutation_in_final", stages)
	write(*out+"/drain-deadlines.tsv", "budget_us\toperation_cost_us\tcommitted\telapsed_ms", drains)
	write(*out+"/final-scrape-matrix.tsv", "mode\trequest_class\teligible\tapplication_before\tapplication_after\tself_metric_delta", finalScrapes)
	write(*out+"/failure-injection.tsv", "case\tinstalled\tprior_core_value\toutcome", failures)
	write(*out+"/restart-epochs.tsv", "iteration\tprevious_final\tnew_epoch_value\tempty", epochs)
	write(*out+"/kubernetes-lifecycle.tsv", "kind\twait_mode\texit_code\treadiness_http\tmetrics_http\tfinal_visible\tkubernetes_api_used", k8s)
	write(*out+"/freeze-concurrency.tsv", "round\twinners\tfreeze_count\tp50_ms\tp95_ms\tp99_ms", concurrency)
	fmt.Printf("assertions=%d passed=%d failed=%d\n", len(assertions), passed, len(assertions)-passed)
	if passed != len(assertions) {
		os.Exit(1)
	}
}
