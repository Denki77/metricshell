package managed

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

type MetricType string

const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
)

type Reason string

const (
	ReasonInvalidName        Reason = "invalid_name"
	ReasonReservedName       Reason = "reserved_name"
	ReasonInvalidType        Reason = "invalid_type"
	ReasonDerivedName        Reason = "derived_name_conflict"
	ReasonInvalidLabels      Reason = "invalid_label_schema"
	ReasonDuplicateLabel     Reason = "duplicate_label"
	ReasonInvalidBuckets     Reason = "invalid_buckets"
	ReasonDescriptorConflict Reason = "descriptor_conflict"
	ReasonUndeclared         Reason = "undeclared"
	ReasonLabelSchema        Reason = "label_schema"
	ReasonWrongType          Reason = "wrong_type"
	ReasonAlreadyInitialized Reason = "already_initialized"
	ReasonInvalidNumber      Reason = "invalid_number"
	ReasonOverflow           Reason = "overflow"
	ReasonUnsupported        Reason = "unsupported_operation"
	ReasonFamilyLimit        Reason = "family_limit"
	ReasonSeriesLimit        Reason = "series_limit"
	ReasonLabelLimit         Reason = "label_limit"
	ReasonBucketLimit        Reason = "bucket_limit"
	ReasonBatchLimit         Reason = "batch_limit"
	ReasonNameLimit          Reason = "name_limit"
	ReasonValueLimit         Reason = "label_value_limit"
	ReasonHelpLimit          Reason = "help_limit"
	ReasonLate               Reason = "late"
)

type SemanticError struct {
	Reason Reason
}

func (err *SemanticError) Error() string { return "managed semantic rejection: " + string(err.Reason) }

func RejectionReason(err error) (Reason, bool) {
	var rejection *SemanticError
	if !errors.As(err, &rejection) {
		return "", false
	}
	return rejection.Reason, true
}

func reject(reason Reason) error { return &SemanticError{Reason: reason} }

type Descriptor struct {
	Name    string
	Help    string
	Type    MetricType
	Labels  []string
	Buckets []float64
}

type OperationKind string

const (
	CounterInitialize OperationKind = "counter_initialize"
	CounterAdd        OperationKind = "counter_add"
	GaugeSet          OperationKind = "gauge_set"
	HistogramObserve  OperationKind = "histogram_observe"
)

type Operation struct {
	Kind   OperationKind
	Name   string
	Labels map[string]string
	Value  float64
}

type Series struct {
	Labels       map[string]string
	Value        float64
	Count        uint64
	Sum          float64
	BucketCounts []uint64
}

type Family struct {
	Descriptor Descriptor
	Series     map[string]Series
}

type Limits struct {
	Families        int
	Series          int
	Labels          int
	Buckets         int
	Batch           int
	MetricNameBytes int
	LabelNameBytes  int
	LabelValueBytes int
	HelpBytes       int
}

func DefaultLimits() Limits {
	return Limits{
		Families: 1024, Series: 10_000, Labels: 8, Buckets: 64, Batch: 64,
		MetricNameBytes: 256, LabelNameBytes: 128, LabelValueBytes: 1 << 10, HelpBytes: 4 << 10,
	}
}

func (limits Limits) Validate() error {
	if limits.Families < 1 || limits.Families > 100_000 || limits.Series < 1 || limits.Series > 100_000 ||
		limits.Labels < 0 || limits.Labels > 64 || limits.Buckets < 1 || limits.Buckets > 1024 ||
		limits.Batch < 1 || limits.Batch > 1024 || limits.MetricNameBytes < 1 || limits.MetricNameBytes > 1024 ||
		limits.LabelNameBytes < 1 || limits.LabelNameBytes > 1024 || limits.LabelValueBytes < 1 || limits.LabelValueBytes > 16<<10 ||
		limits.HelpBytes < 0 || limits.HelpBytes > 64<<10 {
		return reject(ReasonInvalidNumber)
	}
	return nil
}

type Model struct {
	families     map[string]*Family
	limits       Limits
	activeSeries int
}

func NewModel() *Model { return NewModelWithLimits(DefaultLimits()) }

func NewModelWithLimits(limits Limits) *Model {
	return &Model{families: make(map[string]*Family), limits: limits}
}

func (model *Model) Declare(input Descriptor) error {
	if err := model.validateDescriptorLimits(input); err != nil {
		return err
	}
	descriptor, err := canonicalDescriptor(input)
	if err != nil {
		return err
	}
	if existing := model.families[descriptor.Name]; existing != nil {
		if sameDescriptor(existing.Descriptor, descriptor) {
			return nil
		}
		return reject(ReasonDescriptorConflict)
	}
	if len(model.families) >= model.limits.Families {
		return reject(ReasonFamilyLimit)
	}
	for _, family := range model.families {
		if namesOverlap(derivedNames(family.Descriptor), derivedNames(descriptor)) {
			return reject(ReasonDerivedName)
		}
	}
	model.families[descriptor.Name] = &Family{Descriptor: descriptor, Series: make(map[string]Series)}
	return nil
}

func (model *Model) Apply(operation Operation) error {
	next := model.clone()
	if err := next.apply(operation); err != nil {
		return err
	}
	model.families = next.families
	model.activeSeries = next.activeSeries
	return nil
}

func (model *Model) ApplyBatch(operations []Operation) error {
	if len(operations) == 0 {
		return reject(ReasonUnsupported)
	}
	if len(operations) > model.limits.Batch {
		return reject(ReasonBatchLimit)
	}
	next := model.clone()
	for _, operation := range operations {
		if err := next.apply(operation); err != nil {
			return err
		}
	}
	model.families = next.families
	model.activeSeries = next.activeSeries
	return nil
}

func (model *Model) Families() map[string]Family {
	result := make(map[string]Family, len(model.families))
	for name, family := range model.families {
		result[name] = cloneFamily(*family)
	}
	return result
}

func (model *Model) apply(operation Operation) error {
	if len(operation.Name) > model.limits.MetricNameBytes {
		return reject(ReasonNameLimit)
	}
	if len(operation.Labels) > model.limits.Labels {
		return reject(ReasonLabelLimit)
	}
	for name, value := range operation.Labels {
		if len(name) > model.limits.LabelNameBytes {
			return reject(ReasonNameLimit)
		}
		if len(value) > model.limits.LabelValueBytes {
			return reject(ReasonValueLimit)
		}
	}
	family := model.families[operation.Name]
	if family == nil {
		return reject(ReasonUndeclared)
	}
	key, labels, err := resolveLabels(family.Descriptor.Labels, operation.Labels)
	if err != nil {
		return err
	}
	series, exists := family.Series[key]
	if !exists && model.activeSeries >= model.limits.Series {
		return reject(ReasonSeriesLimit)
	}
	switch operation.Kind {
	case CounterInitialize:
		if family.Descriptor.Type != Counter {
			return reject(ReasonWrongType)
		}
		if exists {
			return reject(ReasonAlreadyInitialized)
		}
		if !finiteNonNegative(operation.Value) {
			return reject(ReasonInvalidNumber)
		}
		family.Series[key] = Series{Labels: labels, Value: operation.Value}
		model.activeSeries++
	case CounterAdd:
		if family.Descriptor.Type != Counter {
			return reject(ReasonWrongType)
		}
		if !finiteNonNegative(operation.Value) {
			return reject(ReasonInvalidNumber)
		}
		next := series.Value + operation.Value
		if math.IsInf(next, 0) {
			return reject(ReasonOverflow)
		}
		if !exists {
			series.Labels = labels
			model.activeSeries++
		}
		series.Value = next
		family.Series[key] = series
	case GaugeSet:
		if family.Descriptor.Type != Gauge {
			return reject(ReasonWrongType)
		}
		if !exists {
			series.Labels = labels
			model.activeSeries++
		}
		series.Value = operation.Value
		family.Series[key] = series
	case HistogramObserve:
		if family.Descriptor.Type != Histogram {
			return reject(ReasonWrongType)
		}
		if math.IsNaN(operation.Value) || operation.Value < 0 || math.IsInf(operation.Value, -1) || math.Signbit(operation.Value) {
			return reject(ReasonInvalidNumber)
		}
		if series.Count == ^uint64(0) || (!math.IsInf(operation.Value, 1) && series.Sum > math.MaxFloat64-operation.Value) {
			return reject(ReasonOverflow)
		}
		if !exists {
			series.Labels = labels
			series.BucketCounts = make([]uint64, len(family.Descriptor.Buckets))
			model.activeSeries++
		}
		for index, boundary := range family.Descriptor.Buckets {
			if operation.Value <= boundary && series.BucketCounts[index] == ^uint64(0) {
				return reject(ReasonOverflow)
			}
		}
		series.Count++
		series.Sum += operation.Value
		for index, boundary := range family.Descriptor.Buckets {
			if operation.Value <= boundary {
				series.BucketCounts[index]++
			}
		}
		family.Series[key] = series
	default:
		return reject(ReasonUnsupported)
	}
	return nil
}

func (model *Model) validateDescriptorLimits(descriptor Descriptor) error {
	if len(descriptor.Name) > model.limits.MetricNameBytes {
		return reject(ReasonNameLimit)
	}
	if len(descriptor.Help) > model.limits.HelpBytes {
		return reject(ReasonHelpLimit)
	}
	if len(descriptor.Labels) > model.limits.Labels {
		return reject(ReasonLabelLimit)
	}
	if len(descriptor.Buckets) > model.limits.Buckets {
		return reject(ReasonBucketLimit)
	}
	for _, label := range descriptor.Labels {
		if len(label) > model.limits.LabelNameBytes {
			return reject(ReasonNameLimit)
		}
	}
	return nil
}

func canonicalDescriptor(descriptor Descriptor) (Descriptor, error) {
	if !validName(descriptor.Name) {
		return Descriptor{}, reject(ReasonInvalidName)
	}
	if strings.HasPrefix(descriptor.Name, "metricshell_") {
		return Descriptor{}, reject(ReasonReservedName)
	}
	if descriptor.Type != Counter && descriptor.Type != Gauge && descriptor.Type != Histogram {
		return Descriptor{}, reject(ReasonInvalidType)
	}
	if descriptor.Type == Counter && strings.HasSuffix(descriptor.Name, "_total") {
		return Descriptor{}, reject(ReasonDerivedName)
	}
	if descriptor.Type == Histogram && (strings.HasSuffix(descriptor.Name, "_bucket") || strings.HasSuffix(descriptor.Name, "_sum") || strings.HasSuffix(descriptor.Name, "_count")) {
		return Descriptor{}, reject(ReasonDerivedName)
	}
	descriptor.Labels = append([]string(nil), descriptor.Labels...)
	sort.Strings(descriptor.Labels)
	for index, label := range descriptor.Labels {
		if !validLabelName(label) || label == "__name__" || (descriptor.Type == Histogram && label == "le") {
			return Descriptor{}, reject(ReasonInvalidLabels)
		}
		if index > 0 && label == descriptor.Labels[index-1] {
			return Descriptor{}, reject(ReasonDuplicateLabel)
		}
	}
	descriptor.Buckets = append([]float64(nil), descriptor.Buckets...)
	if descriptor.Type != Histogram && len(descriptor.Buckets) != 0 {
		return Descriptor{}, reject(ReasonInvalidBuckets)
	}
	if descriptor.Type == Histogram {
		if len(descriptor.Buckets) == 0 || !math.IsInf(descriptor.Buckets[len(descriptor.Buckets)-1], 1) {
			return Descriptor{}, reject(ReasonInvalidBuckets)
		}
		for index, boundary := range descriptor.Buckets {
			if math.IsNaN(boundary) || boundary < 0 || math.Signbit(boundary) || (index > 0 && boundary <= descriptor.Buckets[index-1]) {
				return Descriptor{}, reject(ReasonInvalidBuckets)
			}
		}
	}
	return descriptor, nil
}

func resolveLabels(schema []string, input map[string]string) (string, map[string]string, error) {
	if len(input) != len(schema) {
		return "", nil, reject(ReasonLabelSchema)
	}
	labels := make(map[string]string, len(input))
	var key strings.Builder
	for _, name := range schema {
		value, exists := input[name]
		if !exists {
			return "", nil, reject(ReasonLabelSchema)
		}
		labels[name] = value
		fmt.Fprintf(&key, "%d:%s=%d:%s;", len(name), name, len(value), value)
	}
	return key.String(), labels, nil
}

func validName(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if character == '_' || character == ':' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

func validLabelName(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && !math.Signbit(value)
}

func derivedNames(descriptor Descriptor) []string {
	switch descriptor.Type {
	case Counter:
		return []string{descriptor.Name, descriptor.Name + "_total"}
	case Histogram:
		return []string{descriptor.Name, descriptor.Name + "_bucket", descriptor.Name + "_sum", descriptor.Name + "_count"}
	default:
		return []string{descriptor.Name}
	}
}

func namesOverlap(left, right []string) bool {
	for _, a := range left {
		for _, b := range right {
			if a == b {
				return true
			}
		}
	}
	return false
}

func sameDescriptor(left, right Descriptor) bool {
	if left.Name != right.Name || left.Help != right.Help || left.Type != right.Type || len(left.Labels) != len(right.Labels) || len(left.Buckets) != len(right.Buckets) {
		return false
	}
	for index := range left.Labels {
		if left.Labels[index] != right.Labels[index] {
			return false
		}
	}
	for index := range left.Buckets {
		if left.Buckets[index] != right.Buckets[index] {
			return false
		}
	}
	return true
}

func (model *Model) clone() *Model {
	next := NewModelWithLimits(model.limits)
	next.activeSeries = model.activeSeries
	for name, family := range model.families {
		cloned := cloneFamily(*family)
		next.families[name] = &cloned
	}
	return next
}

func cloneFamily(family Family) Family {
	family.Descriptor.Labels = append([]string(nil), family.Descriptor.Labels...)
	family.Descriptor.Buckets = append([]float64(nil), family.Descriptor.Buckets...)
	family.Series = cloneSeries(family.Series)
	return family
}

func cloneSeries(input map[string]Series) map[string]Series {
	result := make(map[string]Series, len(input))
	for key, series := range input {
		labels := make(map[string]string, len(series.Labels))
		for name, value := range series.Labels {
			labels[name] = value
		}
		series.Labels = labels
		series.BucketCounts = append([]uint64(nil), series.BucketCounts...)
		result[key] = series
	}
	return result
}
