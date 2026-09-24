package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type metricType string

const (
	counter   metricType = "counter"
	gauge     metricType = "gauge"
	histogram metricType = "histogram"
)

type descriptor struct {
	Name, Help string
	Type       metricType
	Labels     []string
	Buckets    []float64
}
type series struct {
	Labels       map[string]string
	Value        float64
	Count        uint64
	Sum          float64
	BucketCounts []uint64
}
type family struct {
	Descriptor descriptor
	Series     map[string]*series
}
type registry struct {
	Epoch                 uint64
	AllowSignedHistograms bool
	Families              map[string]*family
}
type op struct {
	Kind, Name string
	Labels     map[string]string
	Value      float64
}

func newRegistry(epoch uint64) *registry {
	return &registry{Epoch: epoch, Families: map[string]*family{}}
}
func newSignedHistogramRegistry(epoch uint64) *registry {
	return &registry{Epoch: epoch, AllowSignedHistograms: true, Families: map[string]*family{}}
}
func canonicalLabels(labels map[string]string) string {
	ks := make([]string, 0, len(labels))
	for k := range labels {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var b strings.Builder
	for _, k := range ks {
		fmt.Fprintf(&b, "%d:%s=%d:%s;", len(k), k, len(labels[k]), labels[k])
	}
	return b.String()
}
func derived(d descriptor) []string {
	switch d.Type {
	case counter:
		return []string{d.Name, d.Name + "_total"}
	case histogram:
		return []string{d.Name, d.Name + "_bucket", d.Name + "_sum", d.Name + "_count"}
	default:
		return []string{d.Name}
	}
}
func validName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if !(c == '_' || c == ':' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func (r *registry) declare(d descriptor) error {
	if !validName(d.Name) || strings.HasPrefix(d.Name, "metricshell_") {
		return errors.New("invalid_or_reserved_name")
	}
	if d.Type != counter && d.Type != gauge && d.Type != histogram {
		return errors.New("invalid_type")
	}
	if d.Type == counter && strings.HasSuffix(d.Name, "_total") {
		return errors.New("derived_name_conflict")
	}
	if d.Type == histogram && (strings.HasSuffix(d.Name, "_bucket") || strings.HasSuffix(d.Name, "_sum") || strings.HasSuffix(d.Name, "_count")) {
		return errors.New("derived_name_conflict")
	}
	ls := append([]string(nil), d.Labels...)
	sort.Strings(ls)
	for i, l := range ls {
		if !validName(l) || l == "__name__" || l == "le" && d.Type == histogram {
			return errors.New("invalid_label_schema")
		}
		if i > 0 && l == ls[i-1] {
			return errors.New("duplicate_label")
		}
	}
	d.Labels = ls
	if d.Type == histogram {
		if len(d.Buckets) == 0 || !math.IsInf(d.Buckets[len(d.Buckets)-1], 1) {
			return errors.New("invalid_buckets")
		}
		for i, b := range d.Buckets {
			if math.IsNaN(b) || (!r.AllowSignedHistograms && b < 0) || (i > 0 && b <= d.Buckets[i-1]) {
				return errors.New("invalid_buckets")
			}
		}
	} else if len(d.Buckets) > 0 {
		return errors.New("buckets_on_non_histogram")
	}
	for n, f := range r.Families {
		if n == d.Name {
			if !sameDescriptor(f.Descriptor, d) {
				return errors.New("descriptor_conflict")
			}
			return nil
		}
		for _, a := range derived(f.Descriptor) {
			for _, b := range derived(d) {
				if a == b {
					return errors.New("derived_name_conflict")
				}
			}
		}
	}
	r.Families[d.Name] = &family{Descriptor: d, Series: map[string]*series{}}
	return nil
}
func sameDescriptor(a, b descriptor) bool {
	if a.Name != b.Name || a.Help != b.Help || a.Type != b.Type || len(a.Labels) != len(b.Labels) || len(a.Buckets) != len(b.Buckets) {
		return false
	}
	for i := range a.Labels {
		if a.Labels[i] != b.Labels[i] {
			return false
		}
	}
	for i := range a.Buckets {
		if a.Buckets[i] != b.Buckets[i] {
			return false
		}
	}
	return true
}
func (r *registry) resolve(o op) (*family, *series, error) {
	f := r.Families[o.Name]
	if f == nil {
		return nil, nil, errors.New("undeclared")
	}
	if len(o.Labels) != len(f.Descriptor.Labels) {
		return nil, nil, errors.New("label_schema")
	}
	for _, n := range f.Descriptor.Labels {
		if _, ok := o.Labels[n]; !ok {
			return nil, nil, errors.New("label_schema")
		}
	}
	key := canonicalLabels(o.Labels)
	s := f.Series[key]
	return f, s, nil
}
func finiteNonNegative(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && !math.Signbit(v)
}
func (r *registry) apply(o op) error {
	f, s, err := r.resolve(o)
	if err != nil {
		return err
	}
	key := canonicalLabels(o.Labels)
	switch o.Kind {
	case "counter_init":
		if f.Descriptor.Type != counter {
			return errors.New("wrong_type")
		}
		if s != nil {
			return errors.New("already_initialized")
		}
		if !finiteNonNegative(o.Value) {
			return errors.New("invalid_counter")
		}
		f.Series[key] = &series{Labels: cloneLabels(o.Labels), Value: o.Value}
	case "counter_add":
		if f.Descriptor.Type != counter {
			return errors.New("wrong_type")
		}
		if !finiteNonNegative(o.Value) {
			return errors.New("invalid_counter")
		}
		if s == nil {
			s = &series{Labels: cloneLabels(o.Labels)}
			f.Series[key] = s
		}
		next := s.Value + o.Value
		if math.IsInf(next, 0) {
			return errors.New("overflow")
		}
		s.Value = next
	case "counter_set":
		if f.Descriptor.Type != counter {
			return errors.New("wrong_type")
		}
		if !finiteNonNegative(o.Value) {
			return errors.New("invalid_counter")
		}
		if s == nil {
			s = &series{Labels: cloneLabels(o.Labels)}
			f.Series[key] = s
		}
		if o.Value < s.Value {
			return errors.New("counter_decrease")
		}
		s.Value = o.Value
	case "gauge_set":
		if f.Descriptor.Type != gauge {
			return errors.New("wrong_type")
		}
		if s == nil {
			s = &series{Labels: cloneLabels(o.Labels)}
			f.Series[key] = s
		}
		s.Value = o.Value
	case "gauge_add":
		if f.Descriptor.Type != gauge {
			return errors.New("wrong_type")
		}
		if math.IsNaN(o.Value) || math.IsInf(o.Value, 0) {
			return errors.New("invalid_gauge_delta")
		}
		if s == nil {
			s = &series{Labels: cloneLabels(o.Labels)}
			f.Series[key] = s
		}
		if math.IsNaN(s.Value) || math.IsInf(s.Value, 0) || math.IsInf(s.Value+o.Value, 0) {
			return errors.New("invalid_gauge_delta")
		}
		s.Value += o.Value
	case "observe":
		if f.Descriptor.Type != histogram {
			return errors.New("wrong_type")
		}
		if math.IsNaN(o.Value) || (!r.AllowSignedHistograms && (o.Value < 0 || math.IsInf(o.Value, -1))) {
			return errors.New("invalid_observation")
		}
		if s == nil {
			s = &series{Labels: cloneLabels(o.Labels), BucketCounts: make([]uint64, len(f.Descriptor.Buckets))}
			f.Series[key] = s
		}
		if !r.AllowSignedHistograms && !math.IsInf(o.Value, 0) && o.Value > 0 && s.Sum > math.MaxFloat64-o.Value {
			return errors.New("overflow")
		}
		s.Count++
		s.Sum += o.Value
		for i, b := range f.Descriptor.Buckets {
			if o.Value <= b {
				s.BucketCounts[i]++
			}
		}
	default:
		return errors.New("unsupported_operation")
	}
	return nil
}
func cloneLabels(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (r *registry) clone() *registry {
	out := newRegistry(r.Epoch)
	out.AllowSignedHistograms = r.AllowSignedHistograms
	for n, f := range r.Families {
		nf := &family{Descriptor: f.Descriptor, Series: map[string]*series{}}
		nf.Descriptor.Labels = append([]string(nil), f.Descriptor.Labels...)
		nf.Descriptor.Buckets = append([]float64(nil), f.Descriptor.Buckets...)
		for k, s := range f.Series {
			ns := *s
			ns.Labels = cloneLabels(s.Labels)
			ns.BucketCounts = append([]uint64(nil), s.BucketCounts...)
			nf.Series[k] = &ns
		}
		out.Families[n] = nf
	}
	return out
}
func (r *registry) batch(ops []op) error {
	next := r.clone()
	for _, o := range ops {
		if err := next.apply(o); err != nil {
			return err
		}
	}
	*r = *next
	return nil
}

type snap struct {
	SchemaVersion int          `json:"schema_version"`
	Families      []snapFamily `json:"families"`
}
type snapFamily struct {
	Name   string       `json:"name"`
	Help   string       `json:"help"`
	Type   metricType   `json:"type"`
	Series []snapSeries `json:"series"`
}
type snapSeries struct {
	Labels    map[string]string `json:"labels"`
	Value     string            `json:"value,omitempty"`
	Histogram *snapHist         `json:"histogram,omitempty"`
}
type snapHist struct {
	Count   string       `json:"count"`
	Sum     string       `json:"sum"`
	Buckets []snapBucket `json:"buckets"`
}
type snapBucket struct {
	LE    string `json:"le"`
	Count string `json:"count"`
}

func number(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 1) {
		return "+Inf"
	}
	if math.IsInf(v, -1) {
		return "-Inf"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}
func (r *registry) snapshot() []byte {
	out := snap{SchemaVersion: 1, Families: []snapFamily{}}
	names := make([]string, 0, len(r.Families))
	for n := range r.Families {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f := r.Families[n]
		if len(f.Series) == 0 {
			continue
		}
		sf := snapFamily{Name: n, Help: f.Descriptor.Help, Type: f.Descriptor.Type, Series: []snapSeries{}}
		keys := make([]string, 0, len(f.Series))
		for k := range f.Series {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s := f.Series[k]
			ss := snapSeries{Labels: s.Labels}
			if f.Descriptor.Type == histogram {
				h := &snapHist{Count: strconv.FormatUint(s.Count, 10), Sum: number(s.Sum), Buckets: []snapBucket{}}
				for i, b := range f.Descriptor.Buckets {
					h.Buckets = append(h.Buckets, snapBucket{LE: number(b), Count: strconv.FormatUint(s.BucketCounts[i], 10)})
				}
				ss.Histogram = h
			} else {
				ss.Value = number(s.Value)
			}
			sf.Series = append(sf.Series, ss)
		}
		out.Families = append(out.Families, sf)
	}
	b, _ := json.Marshal(out)
	return append(b, '\n')
}

type recorder struct {
	rows           []string
	passed, failed int
}

func (x *recorder) check(exp bool, experiment, name, expected, actual string) {
	result := "fail"
	if exp {
		result = "pass"
		x.passed++
	} else {
		x.failed++
	}
	x.rows = append(x.rows, strings.Join([]string{experiment, name, expected, actual, result}, "\t"))
}
func main() {
	out := flag.String("out", "/results", "output directory")
	repetitions := flag.Int("repetitions", 30, "snapshot repetitions")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	r := newRegistry(1)
	rec := &recorder{}
	decls := []descriptor{{"requests", "Requests.", counter, []string{"method", "status"}, nil}, {"temperature", "Temperature.", gauge, []string{"room"}, nil}, {"latency_seconds", "Latency.", histogram, []string{"route"}, []float64{0, 0.1, 0.5, math.Inf(1)}}}
	for _, d := range decls {
		rec.check(r.declare(d) == nil, "E-016.4", "declare_"+d.Name, "accepted", "accepted")
	}
	labels := map[string]string{"method": "GET", "status": "200"}
	for _, v := range []float64{0, 1, 2.5} {
		err := r.apply(op{"counter_add", "requests", labels, v})
		rec.check(err == nil, "E-016.1", "counter_add_"+number(v), "accepted", errText(err))
	}
	rec.check(value(r, "requests", labels) == 3.5, "E-016.1", "counter_exact", "3.5", number(value(r, "requests", labels)))
	before := string(r.snapshot())
	for name, v := range map[string]float64{"negative": -1, "nan": math.NaN(), "pos_inf": math.Inf(1), "neg_inf": math.Inf(-1)} {
		err := r.apply(op{"counter_add", "requests", labels, v})
		rec.check(err != nil && string(r.snapshot()) == before, "E-016.1", "counter_reject_"+name, "reject_unchanged", errText(err))
	}
	err := r.apply(op{"counter_set", "requests", labels, 3.5})
	rec.check(err == nil, "E-016.1", "absolute_repeat", "accepted", errText(err))
	err = r.apply(op{"counter_set", "requests", labels, 4})
	rec.check(err == nil, "E-016.1", "absolute_increase", "accepted", errText(err))
	before = string(r.snapshot())
	err = r.apply(op{"counter_set", "requests", labels, 3})
	rec.check(err != nil && string(r.snapshot()) == before, "E-016.1", "absolute_decrease", "reject_unchanged", errText(err))
	err = r.apply(op{"counter_init", "requests", map[string]string{"method": "POST", "status": "201"}, 7})
	rec.check(err == nil, "E-016.1", "explicit_initialization", "accepted", errText(err))
	before = string(r.snapshot())
	err = r.apply(op{"counter_init", "requests", labels, 9})
	rec.check(err != nil && string(r.snapshot()) == before, "E-016.1", "reinitialize", "reject_unchanged", errText(err))
	before = string(r.snapshot())
	err = r.apply(op{"counter_reset", "requests", labels, 0})
	rec.check(err != nil && string(r.snapshot()) == before, "E-016.1", "same_epoch_reset", "reject_unchanged", errText(err))
	overflowLabels := map[string]string{"method": "MAX", "status": "200"}
	_ = r.apply(op{"counter_init", "requests", overflowLabels, math.MaxFloat64})
	before = string(r.snapshot())
	err = r.apply(op{"counter_add", "requests", overflowLabels, math.MaxFloat64})
	rec.check(err != nil && string(r.snapshot()) == before, "E-016.1", "counter_overflow", "reject_unchanged", errText(err))
	r2 := newRegistry(2)
	_ = r2.declare(decls[0])
	rec.check(len(r2.Families["requests"].Series) == 0, "E-016.1", "new_epoch_empty", "0", fmt.Sprint(len(r2.Families["requests"].Series)))
	for name, v := range map[string]float64{"zero": 0, "positive": 12.5, "negative": -3, "nan": math.NaN(), "pos_inf": math.Inf(1), "neg_inf": math.Inf(-1)} {
		err = r.apply(op{"gauge_set", "temperature", map[string]string{"room": "a"}, v})
		rec.check(err == nil, "E-016.2", "gauge_set_"+name, "accepted", errText(err))
	}
	err = r.apply(op{"gauge_set", "temperature", map[string]string{"room": "a"}, 10})
	_ = err
	err = r.apply(op{"gauge_add", "temperature", map[string]string{"room": "a"}, -2})
	rec.check(err == nil && value(r, "temperature", map[string]string{"room": "a"}) == 8, "E-016.2", "gauge_add_sub", "8", number(value(r, "temperature", map[string]string{"room": "a"})))
	before = string(r.snapshot())
	err = r.apply(op{"gauge_add", "temperature", map[string]string{"room": "a"}, math.NaN()})
	rec.check(err != nil && string(r.snapshot()) == before, "E-016.2", "gauge_invalid_delta", "reject_unchanged", errText(err))
	for _, v := range []float64{0, 0.1, 0.1000001, 0.5, 1, math.Inf(1)} {
		err = r.apply(op{"observe", "latency_seconds", map[string]string{"route": "/"}, v})
		rec.check(err == nil, "E-016.3", "observe_"+number(v), "accepted", errText(err))
	}
	hs := r.Families["latency_seconds"].Series[canonicalLabels(map[string]string{"route": "/"})]
	rec.check(hs.Count == 6 && hs.BucketCounts[len(hs.BucketCounts)-1] == 6, "E-016.3", "histogram_atomic_totals", "count=6,+Inf=6", fmt.Sprintf("count=%d,+Inf=%d", hs.Count, hs.BucketCounts[len(hs.BucketCounts)-1]))
	before = string(r.snapshot())
	for name, v := range map[string]float64{"negative": -1, "nan": math.NaN(), "neg_inf": math.Inf(-1)} {
		err = r.apply(op{"observe", "latency_seconds", map[string]string{"route": "/"}, v})
		rec.check(err != nil && string(r.snapshot()) == before, "E-016.3", "observe_reject_"+name, "reject_unchanged", errText(err))
	}
	conflicts := []descriptor{{"requests", "different", counter, []string{"method", "status"}, nil}, {"requests", "Requests.", gauge, []string{"method", "status"}, nil}, {"latency_seconds", "Latency.", histogram, []string{"route"}, []float64{0, 0.2, math.Inf(1)}}, {"requests_total", "x", gauge, nil, nil}, {"latency_seconds_bucket", "x", gauge, nil, nil}, {"metricshell_bad", "x", gauge, nil, nil}, {"bad_hist", "x", histogram, nil, []float64{1, 0.5, math.Inf(1)}}, {"duplicate_labels", "x", gauge, []string{"x", "x"}, nil}}
	before = string(r.snapshot())
	for i, d := range conflicts {
		err = r.declare(d)
		rec.check(err != nil && string(r.snapshot()) == before, "E-016.4", fmt.Sprintf("conflict_%d", i), "reject_unchanged", errText(err))
	}
	reordered := map[string]string{"status": "200", "method": "GET"}
	rec.check(canonicalLabels(labels) == canonicalLabels(reordered), "E-016.5", "label_order_identity", "same", "same")
	for name, l := range map[string]map[string]string{"missing": {"method": "GET"}, "extra": {"method": "GET", "status": "200", "x": "y"}} {
		before = string(r.snapshot())
		err = r.apply(op{"counter_add", "requests", l, 1})
		rec.check(err != nil && string(r.snapshot()) == before, "E-016.5", "labels_"+name, "reject_unchanged", errText(err))
	}
	before = string(r.snapshot())
	err = r.apply(op{"counter_add", "implicit_family", nil, 1})
	rec.check(err != nil && string(r.snapshot()) == before, "E-016.5", "implicit_declaration", "reject_unchanged", errText(err))
	err = r.apply(op{"delete_series", "requests", labels, 0})
	rec.check(err != nil && string(r.snapshot()) == before, "E-016.5", "series_deletion", "reject_unchanged", errText(err))
	rec.check(string(r.snapshot()) == before, "E-016.5", "disconnect_no_effect", "unchanged", "unchanged")
	before = string(r.snapshot())
	err = r.batch([]op{{"counter_add", "requests", labels, 1}, {"gauge_set", "missing", nil, 1}})
	rec.check(err != nil && string(r.snapshot()) == before, "E-016.6", "invalid_batch", "reject_all_unchanged", errText(err))
	err = r.batch([]op{{"counter_add", "requests", labels, 1}, {"gauge_set", "temperature", map[string]string{"room": "a"}, 3}})
	rec.check(err == nil, "E-016.6", "valid_batch", "accepted_all", errText(err))

	// Compare a signed-observation candidate independently of the restrictive candidate above.
	negativeSum, err := histogramCandidate([]float64{0, 1, math.Inf(1)}, []float64{-0.5})
	rec.check(err == nil, "E-016.3-signed", "negative_observation_candidate", "accepted", errText(err))
	negativeSeries := negativeSum.Families["candidate_histogram"].Series[canonicalLabels(map[string]string{})]
	rec.check(negativeSeries.Sum == -0.5 && negativeSeries.Count == 1, "E-016.3-signed", "negative_sum", "count=1,sum=-0.5", fmt.Sprintf("count=%d,sum=%s", negativeSeries.Count, number(negativeSeries.Sum)))

	crossZero, err := histogramCandidate([]float64{0, 1, math.Inf(1)}, []float64{-0.5, 1})
	rec.check(err == nil, "E-016.3-signed", "sum_crosses_zero_candidate", "accepted", errText(err))
	crossSeries := crossZero.Families["candidate_histogram"].Series[canonicalLabels(map[string]string{})]
	rec.check(crossSeries.Sum == 0.5 && crossSeries.Count == 2, "E-016.3-signed", "sum_after_crossing_zero", "count=2,sum=0.5", fmt.Sprintf("count=%d,sum=%s", crossSeries.Count, number(crossSeries.Sum)))

	mixedBuckets, err := histogramCandidate([]float64{-1, 0, 1, math.Inf(1)}, []float64{-0.5, 0, 1.5})
	rec.check(err == nil, "E-016.3-signed", "negative_and_positive_buckets_candidate", "accepted", errText(err))
	mixedSeries := mixedBuckets.Families["candidate_histogram"].Series[canonicalLabels(map[string]string{})]
	rec.check(mixedSeries.Count == 3 && mixedSeries.BucketCounts[0] == 0 && mixedSeries.BucketCounts[1] == 2 && mixedSeries.BucketCounts[3] == 3, "E-016.3-signed", "signed_bucket_counts", "0,2,2,3", fmt.Sprint(mixedSeries.BucketCounts))

	balanced, err := histogramCandidate([]float64{-1, 0, 1, math.Inf(1)}, []float64{-1, 1})
	rec.check(err == nil, "E-016.3-signed", "multiple_negative_positive_observations", "accepted", errText(err))
	balancedSeries := balanced.Families["candidate_histogram"].Series[canonicalLabels(map[string]string{})]
	rec.check(balancedSeries.Sum == 0 && balancedSeries.Count == 2, "E-016.3-signed", "signed_sum_returns_zero", "count=2,sum=0", fmt.Sprintf("count=%d,sum=%s", balancedSeries.Count, number(balancedSeries.Sum)))

	negativeInfinity, err := histogramCandidate([]float64{0, 1, math.Inf(1)}, []float64{math.Inf(-1)})
	rec.check(err == nil, "E-016.3-signed", "negative_infinity_candidate", "accepted", errText(err))
	_, nanErr := histogramCandidate([]float64{0, 1, math.Inf(1)}, []float64{math.NaN()})
	rec.check(nanErr != nil, "E-016.3-signed", "nan_candidate", "rejected", errText(nanErr))

	snapshot := r.snapshot()
	_ = os.WriteFile(*out+"/snapshot.json", snapshot, 0644)
	empty := newRegistry(9).snapshot()
	_ = os.WriteFile(*out+"/empty-snapshot.json", empty, 0644)
	_ = os.WriteFile(*out+"/histogram-negative-sum.json", negativeSum.snapshot(), 0644)
	_ = os.WriteFile(*out+"/histogram-cross-zero.json", crossZero.snapshot(), 0644)
	_ = os.WriteFile(*out+"/histogram-negative-buckets.json", mixedBuckets.snapshot(), 0644)
	_ = os.WriteFile(*out+"/histogram-balanced-signed.json", balanced.snapshot(), 0644)
	_ = os.WriteFile(*out+"/histogram-negative-infinity.json", negativeInfinity.snapshot(), 0644)
	rec.check(string(empty) == "{\"schema_version\":1,\"families\":[]}\n", "E-016.7", "empty_registry", "core_zero_snapshot", strings.TrimSpace(string(empty)))
	start := time.Now()
	bytes := 0
	for i := 0; i < *repetitions; i++ {
		bytes += len(r.snapshot())
	}
	elapsed := time.Since(start)
	_ = os.WriteFile(*out+"/materialization.tsv", []byte(fmt.Sprintf("repetitions\ttotal_bytes\ttotal_ns\tns_per_snapshot\n%d\t%d\t%d\t%d\n", *repetitions, bytes, elapsed.Nanoseconds(), elapsed.Nanoseconds()/int64(*repetitions))), 0644)
	scaling := "series\tsnapshot_bytes\trepetitions\ttotal_ns\tns_per_snapshot\n"
	for _, n := range []int{0, 1, 10, 100, 1000, 10000} {
		sr := newRegistry(1)
		_ = sr.declare(descriptor{"scale", "Scale.", counter, []string{"id"}, nil})
		for i := 0; i < n; i++ {
			_ = sr.apply(op{"counter_add", "scale", map[string]string{"id": strconv.Itoa(i)}, 1})
		}
		started := time.Now()
		size := 0
		for i := 0; i < *repetitions; i++ {
			size = len(sr.snapshot())
		}
		ns := time.Since(started).Nanoseconds()
		scaling += fmt.Sprintf("%d\t%d\t%d\t%d\t%d\n", n, size, *repetitions, ns, ns/int64(*repetitions))
	}
	_ = os.WriteFile(*out+"/scaling.tsv", []byte(scaling), 0644)
	data := "experiment\tassertion\texpected\tactual\tresult\n" + strings.Join(rec.rows, "\n") + "\n"
	_ = os.WriteFile(*out+"/assertions.tsv", []byte(data), 0644)
	_ = os.WriteFile(*out+"/summary.tsv", []byte(fmt.Sprintf("passed\tfailed\n%d\t%d\n", rec.passed, rec.failed)), 0644)
	if rec.failed > 0 {
		os.Exit(1)
	}
}
func value(r *registry, name string, labels map[string]string) float64 {
	s := r.Families[name].Series[canonicalLabels(labels)]
	if s == nil {
		return 0
	}
	return s.Value
}
func errText(err error) string {
	if err == nil {
		return "accepted"
	}
	return err.Error()
}

func histogramCandidate(buckets, observations []float64) (*registry, error) {
	r := newSignedHistogramRegistry(1)
	if err := r.declare(descriptor{"candidate_histogram", "Candidate histogram.", histogram, nil, buckets}); err != nil {
		return nil, err
	}
	for _, observation := range observations {
		if err := r.apply(op{"observe", "candidate_histogram", map[string]string{}, observation}); err != nil {
			return nil, err
		}
	}
	return r, nil
}
