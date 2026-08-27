package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const SchemaVersion = 1

type Reason string

const (
	ReasonMalformed        Reason = "malformed"
	ReasonNumericInvalid   Reason = "numeric_invalid"
	ReasonSchemaVersion    Reason = "schema_version"
	ReasonEmptyPayload     Reason = "empty_payload"
	ReasonPayloadLimit     Reason = "payload_limit"
	ReasonSeriesLimit      Reason = "series_limit"
	ReasonLabelLimit       Reason = "label_limit"
	ReasonNameLimit        Reason = "name_limit"
	ReasonPolicy           Reason = "policy"
	ReasonDuplicateSeries  Reason = "duplicate_series"
	ReasonTypeConflict     Reason = "type_conflict"
	ReasonMetadataConflict Reason = "metadata_conflict"
	ReasonHistogramInvalid Reason = "histogram_invalid"
	ReasonReservedName     Reason = "reserved_name"
	ReasonFrozen           Reason = "frozen"
	ReasonInternal         Reason = "internal"
)

var RejectionReasons = [...]Reason{
	ReasonMalformed, ReasonNumericInvalid, ReasonSchemaVersion, ReasonEmptyPayload,
	ReasonPayloadLimit, ReasonSeriesLimit, ReasonLabelLimit, ReasonNameLimit, ReasonPolicy,
	ReasonDuplicateSeries, ReasonTypeConflict, ReasonMetadataConflict, ReasonHistogramInvalid,
	ReasonReservedName, ReasonFrozen, ReasonInternal,
}

type Rejection struct{ reason Reason }

func Reject(reason Reason) error           { return Rejection{reason: reason} }
func (rejection Rejection) Error() string  { return string(rejection.reason) }
func (rejection Rejection) Reason() Reason { return rejection.reason }

func RejectionReason(err error) (Reason, bool) {
	var rejection Rejection
	if !errors.As(err, &rejection) {
		return "", false
	}
	return rejection.reason, true
}

type Limits struct {
	SnapshotBytes   int
	DecodedBytes    int
	Series          int
	LabelsPerSeries int
	MetricNameBytes int
	LabelNameBytes  int
	LabelValueBytes int
	HelpBytes       int
}

func DefaultLimits() Limits {
	return Limits{
		SnapshotBytes: 1 << 20, DecodedBytes: 2 << 20, Series: 10_000, LabelsPerSeries: 8,
		MetricNameBytes: 256, LabelNameBytes: 128, LabelValueBytes: 1 << 10, HelpBytes: 4 << 10,
	}
}

type MetricType string

const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
)

type InputLabel struct {
	Name  string
	Value string
}

type InputBucket struct {
	UpperBound string
	Count      string
}

type InputHistogram struct {
	Count   string
	Sum     string
	Buckets []InputBucket
}

type InputSeries struct {
	Labels    []InputLabel
	Value     string
	Histogram *InputHistogram
}

type InputFamily struct {
	Name   string
	Help   string
	Type   MetricType
	Series []InputSeries
}

type CandidateSnapshot struct {
	schemaVersion int
	families      []InputFamily
	decodedBytes  int
}

func NewCandidate(schemaVersion int, families []InputFamily, decodedBytes int) CandidateSnapshot {
	return CandidateSnapshot{schemaVersion: schemaVersion, families: cloneInputFamilies(families), decodedBytes: decodedBytes}
}

func (candidate CandidateSnapshot) SchemaVersion() int { return candidate.schemaVersion }
func (candidate CandidateSnapshot) DecodedBytes() int  { return candidate.decodedBytes }
func (candidate CandidateSnapshot) Families() []InputFamily {
	return cloneInputFamilies(candidate.families)
}

type Label struct {
	name  string
	value string
}

func (label Label) Name() string  { return label.name }
func (label Label) Value() string { return label.value }

type Bucket struct {
	upperBound string
	count      uint64
}

func (bucket Bucket) UpperBound() string { return bucket.upperBound }
func (bucket Bucket) Count() uint64      { return bucket.count }

type HistogramValue struct {
	count   uint64
	sum     string
	buckets []Bucket
}

func (value HistogramValue) Count() uint64     { return value.count }
func (value HistogramValue) Sum() string       { return value.sum }
func (value HistogramValue) Buckets() []Bucket { return append([]Bucket(nil), value.buckets...) }

type Series struct {
	labels    []Label
	value     string
	histogram *HistogramValue
}

func (series Series) Labels() []Label { return append([]Label(nil), series.labels...) }
func (series Series) Value() string   { return series.value }
func (series Series) Histogram() (HistogramValue, bool) {
	if series.histogram == nil {
		return HistogramValue{}, false
	}
	value := *series.histogram
	value.buckets = append([]Bucket(nil), value.buckets...)
	return value, true
}

type Family struct {
	name   string
	help   string
	typeID MetricType
	series []Series
}

func (family Family) Name() string     { return family.name }
func (family Family) Help() string     { return family.help }
func (family Family) Type() MetricType { return family.typeID }
func (family Family) Series() []Series { return cloneSeries(family.series) }

type ValidatedSnapshot struct {
	families  []Family
	canonical []byte
	series    int
}

func (snapshot ValidatedSnapshot) Families() []Family { return cloneFamilies(snapshot.families) }
func (snapshot ValidatedSnapshot) Canonical() []byte {
	return append([]byte(nil), snapshot.canonical...)
}
func (snapshot ValidatedSnapshot) SeriesCount() int    { return snapshot.series }
func (snapshot ValidatedSnapshot) CanonicalBytes() int { return len(snapshot.canonical) }
func (snapshot ValidatedSnapshot) IsZeroSeries() bool  { return snapshot.series == 0 }

type ActiveSnapshot struct {
	validated  ValidatedSnapshot
	generation uint64
}

func NewActive(validated ValidatedSnapshot, generation uint64) ActiveSnapshot {
	return ActiveSnapshot{validated: cloneValidated(validated), generation: generation}
}

func (snapshot ActiveSnapshot) Generation() uint64 { return snapshot.generation }
func (snapshot ActiveSnapshot) Validated() ValidatedSnapshot {
	return cloneValidated(snapshot.validated)
}

var (
	metricNamePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	labelNamePattern  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	finitePattern     = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
	countPattern      = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
)

func Validate(candidate CandidateSnapshot, limits Limits) (ValidatedSnapshot, error) {
	if candidate.schemaVersion != SchemaVersion {
		return ValidatedSnapshot{}, Reject(ReasonSchemaVersion)
	}
	if candidate.decodedBytes > limits.DecodedBytes {
		return ValidatedSnapshot{}, Reject(ReasonPayloadLimit)
	}

	families := make([]Family, 0, len(candidate.families))
	baseTypes := make(map[string]MetricType, len(candidate.families))
	encodedNames := make(map[string]string, len(candidate.families)*3)
	totalSeries := 0
	for _, input := range candidate.families {
		if err := validateFamilyMetadata(input, limits); err != nil {
			return ValidatedSnapshot{}, err
		}
		if previous, exists := baseTypes[input.Name]; exists {
			if previous != input.Type {
				return ValidatedSnapshot{}, Reject(ReasonTypeConflict)
			}
			return ValidatedSnapshot{}, Reject(ReasonMetadataConflict)
		}
		baseTypes[input.Name] = input.Type
		for _, name := range encodedSampleNames(input.Name, input.Type) {
			if len(name) > limits.MetricNameBytes {
				return ValidatedSnapshot{}, Reject(ReasonNameLimit)
			}
			if owner, exists := encodedNames[name]; exists && owner != input.Name {
				return ValidatedSnapshot{}, Reject(ReasonMetadataConflict)
			}
			encodedNames[name] = input.Name
		}

		family := Family{name: input.Name, help: input.Help, typeID: input.Type, series: make([]Series, 0, len(input.Series))}
		identities := make(map[string]struct{}, len(input.Series))
		for _, inputSeries := range input.Series {
			series, identity, err := validateSeries(input.Type, inputSeries, limits)
			if err != nil {
				return ValidatedSnapshot{}, err
			}
			if _, exists := identities[identity]; exists {
				return ValidatedSnapshot{}, Reject(ReasonDuplicateSeries)
			}
			identities[identity] = struct{}{}
			family.series = append(family.series, series)
			totalSeries++
			if totalSeries > limits.Series {
				return ValidatedSnapshot{}, Reject(ReasonSeriesLimit)
			}
		}
		if len(family.series) > 0 {
			sort.Slice(family.series, func(left, right int) bool {
				return seriesIdentity(family.series[left].labels) < seriesIdentity(family.series[right].labels)
			})
			families = append(families, family)
		}
	}
	sort.Slice(families, func(left, right int) bool { return families[left].name < families[right].name })
	canonical, err := marshalCanonical(families)
	if err != nil {
		return ValidatedSnapshot{}, Reject(ReasonInternal)
	}
	if len(canonical) > limits.SnapshotBytes {
		return ValidatedSnapshot{}, Reject(ReasonPayloadLimit)
	}
	return ValidatedSnapshot{families: families, canonical: canonical, series: totalSeries}, nil
}

func validateFamilyMetadata(family InputFamily, limits Limits) error {
	if !utf8.ValidString(family.Name) || !metricNamePattern.MatchString(family.Name) {
		return Reject(ReasonPolicy)
	}
	if len(family.Name) > limits.MetricNameBytes || len(family.Help) > limits.HelpBytes {
		return Reject(ReasonNameLimit)
	}
	if !utf8.ValidString(family.Help) {
		return Reject(ReasonPolicy)
	}
	if strings.HasPrefix(family.Name, "metricshell_") {
		return Reject(ReasonReservedName)
	}
	switch family.Type {
	case Counter:
		if strings.HasSuffix(family.Name, "_total") {
			return Reject(ReasonMetadataConflict)
		}
	case Gauge:
	case Histogram:
		if strings.HasSuffix(family.Name, "_bucket") || strings.HasSuffix(family.Name, "_sum") || strings.HasSuffix(family.Name, "_count") {
			return Reject(ReasonMetadataConflict)
		}
	default:
		return Reject(ReasonPolicy)
	}
	return nil
}

func validateSeries(metricType MetricType, input InputSeries, limits Limits) (Series, string, error) {
	if len(input.Labels) > limits.LabelsPerSeries {
		return Series{}, "", Reject(ReasonLabelLimit)
	}
	labels := make([]Label, 0, len(input.Labels))
	seen := make(map[string]struct{}, len(input.Labels))
	for _, inputLabel := range input.Labels {
		if !utf8.ValidString(inputLabel.Name) || !labelNamePattern.MatchString(inputLabel.Name) || inputLabel.Name == "__name__" {
			return Series{}, "", Reject(ReasonPolicy)
		}
		if len(inputLabel.Name) > limits.LabelNameBytes || len(inputLabel.Value) > limits.LabelValueBytes {
			return Series{}, "", Reject(ReasonNameLimit)
		}
		if !utf8.ValidString(inputLabel.Value) {
			return Series{}, "", Reject(ReasonPolicy)
		}
		if _, exists := seen[inputLabel.Name]; exists {
			return Series{}, "", Reject(ReasonDuplicateSeries)
		}
		seen[inputLabel.Name] = struct{}{}
		labels = append(labels, Label{name: inputLabel.Name, value: inputLabel.Value})
	}
	sort.Slice(labels, func(left, right int) bool { return labels[left].name < labels[right].name })
	identity := seriesIdentity(labels)
	series := Series{labels: labels}
	switch metricType {
	case Counter:
		if input.Histogram != nil {
			return Series{}, "", Reject(ReasonTypeConflict)
		}
		value, parsed, err := canonicalFinite(input.Value)
		if err != nil || math.Signbit(parsed) {
			return Series{}, "", Reject(ReasonNumericInvalid)
		}
		series.value = value
	case Gauge:
		if input.Histogram != nil {
			return Series{}, "", Reject(ReasonTypeConflict)
		}
		value, err := canonicalGauge(input.Value)
		if err != nil {
			return Series{}, "", err
		}
		series.value = value
	case Histogram:
		if _, exists := seen["le"]; exists || input.Histogram == nil || input.Value != "" {
			return Series{}, "", Reject(ReasonHistogramInvalid)
		}
		value, err := validateHistogram(*input.Histogram)
		if err != nil {
			return Series{}, "", err
		}
		series.histogram = &value
	}
	return series, identity, nil
}

func validateHistogram(input InputHistogram) (HistogramValue, error) {
	count, err := parseCount(input.Count)
	if err != nil {
		return HistogramValue{}, Reject(ReasonHistogramInvalid)
	}
	sum, sumValue, err := canonicalNonNegative(input.Sum, true)
	if err != nil || math.Signbit(sumValue) {
		return HistogramValue{}, Reject(ReasonHistogramInvalid)
	}
	value := HistogramValue{count: count, sum: sum, buckets: make([]Bucket, 0, len(input.Buckets))}
	var previousBound float64
	var previousCount uint64
	for index, inputBucket := range input.Buckets {
		bound, parsedBound, boundErr := canonicalNonNegative(inputBucket.UpperBound, true)
		bucketCount, countErr := parseCount(inputBucket.Count)
		if boundErr != nil || countErr != nil || math.Signbit(parsedBound) || (index > 0 && parsedBound <= previousBound) || (index > 0 && bucketCount < previousCount) {
			return HistogramValue{}, Reject(ReasonHistogramInvalid)
		}
		value.buckets = append(value.buckets, Bucket{upperBound: bound, count: bucketCount})
		previousBound, previousCount = parsedBound, bucketCount
	}
	if len(value.buckets) == 0 || value.buckets[len(value.buckets)-1].upperBound != "+Inf" || previousCount != count {
		return HistogramValue{}, Reject(ReasonHistogramInvalid)
	}
	return value, nil
}

func canonicalGauge(token string) (string, error) {
	switch token {
	case "NaN", "+Inf", "-Inf":
		return token, nil
	default:
		value, _, err := canonicalFinite(token)
		if err != nil {
			return "", Reject(ReasonNumericInvalid)
		}
		return value, nil
	}
}

func canonicalNonNegative(token string, allowPositiveInfinity bool) (string, float64, error) {
	if allowPositiveInfinity && token == "+Inf" {
		return token, math.Inf(1), nil
	}
	canonical, value, err := canonicalFinite(token)
	if err != nil || value < 0 || math.Signbit(value) {
		return "", 0, Reject(ReasonHistogramInvalid)
	}
	return canonical, value, nil
}

func canonicalFinite(token string) (string, float64, error) {
	if !finitePattern.MatchString(token) {
		return "", 0, Reject(ReasonNumericInvalid)
	}
	value, err := strconv.ParseFloat(token, 64)
	if err != nil || math.IsInf(value, 0) || (value == 0 && tokenRepresentsNonZero(token)) {
		return "", 0, Reject(ReasonNumericInvalid)
	}
	return strconv.FormatFloat(value, 'g', -1, 64), value, nil
}

func tokenRepresentsNonZero(token string) bool {
	mantissa := token
	if exponent := strings.IndexAny(mantissa, "eE"); exponent >= 0 {
		mantissa = mantissa[:exponent]
	}
	mantissa = strings.TrimPrefix(mantissa, "-")
	mantissa = strings.ReplaceAll(mantissa, ".", "")
	return strings.Trim(mantissa, "0") != ""
}

func parseCount(token string) (uint64, error) {
	if !countPattern.MatchString(token) {
		return 0, Reject(ReasonNumericInvalid)
	}
	return strconv.ParseUint(token, 10, 64)
}

func encodedSampleNames(base string, metricType MetricType) []string {
	switch metricType {
	case Counter:
		return []string{base, base + "_total"}
	case Histogram:
		return []string{base, base + "_bucket", base + "_sum", base + "_count"}
	default:
		return []string{base}
	}
}

func seriesIdentity(labels []Label) string {
	var identity strings.Builder
	for _, label := range labels {
		identity.WriteString(strconv.Itoa(len(label.name)))
		identity.WriteByte(':')
		identity.WriteString(label.name)
		identity.WriteString(strconv.Itoa(len(label.value)))
		identity.WriteByte(':')
		identity.WriteString(label.value)
	}
	return identity.String()
}

type canonicalDocument struct {
	SchemaVersion int               `json:"schema_version"`
	Families      []canonicalFamily `json:"families"`
}

type canonicalFamily struct {
	Name   string            `json:"name"`
	Help   string            `json:"help"`
	Type   MetricType        `json:"type"`
	Series []canonicalSeries `json:"series"`
}

type canonicalSeries struct {
	Labels    map[string]string   `json:"labels"`
	Value     string              `json:"value,omitempty"`
	Histogram *canonicalHistogram `json:"histogram,omitempty"`
}

type canonicalHistogram struct {
	Count   string            `json:"count"`
	Sum     string            `json:"sum"`
	Buckets []canonicalBucket `json:"buckets"`
}

type canonicalBucket struct {
	UpperBound string `json:"le"`
	Count      string `json:"count"`
}

func marshalCanonical(families []Family) ([]byte, error) {
	document := canonicalDocument{SchemaVersion: SchemaVersion, Families: make([]canonicalFamily, 0, len(families))}
	for _, family := range families {
		encoded := canonicalFamily{Name: family.name, Help: family.help, Type: family.typeID, Series: make([]canonicalSeries, 0, len(family.series))}
		for _, series := range family.series {
			labels := make(map[string]string, len(series.labels))
			for _, label := range series.labels {
				labels[label.name] = label.value
			}
			encodedSeries := canonicalSeries{Labels: labels, Value: series.value}
			if series.histogram != nil {
				histogram := canonicalHistogram{Count: strconv.FormatUint(series.histogram.count, 10), Sum: series.histogram.sum, Buckets: make([]canonicalBucket, 0, len(series.histogram.buckets))}
				for _, bucket := range series.histogram.buckets {
					histogram.Buckets = append(histogram.Buckets, canonicalBucket{UpperBound: bucket.upperBound, Count: strconv.FormatUint(bucket.count, 10)})
				}
				encodedSeries.Histogram = &histogram
			}
			encoded.Series = append(encoded.Series, encodedSeries)
		}
		document.Families = append(document.Families, encoded)
	}
	content, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func cloneInputFamilies(source []InputFamily) []InputFamily {
	result := make([]InputFamily, len(source))
	for index, family := range source {
		result[index] = family
		result[index].Series = make([]InputSeries, len(family.Series))
		for seriesIndex, series := range family.Series {
			result[index].Series[seriesIndex] = series
			result[index].Series[seriesIndex].Labels = append([]InputLabel(nil), series.Labels...)
			if series.Histogram != nil {
				histogram := *series.Histogram
				histogram.Buckets = append([]InputBucket(nil), series.Histogram.Buckets...)
				result[index].Series[seriesIndex].Histogram = &histogram
			}
		}
	}
	return result
}

func cloneSeries(source []Series) []Series {
	result := make([]Series, len(source))
	for index, series := range source {
		result[index] = series
		result[index].labels = append([]Label(nil), series.labels...)
		if series.histogram != nil {
			histogram := *series.histogram
			histogram.buckets = append([]Bucket(nil), series.histogram.buckets...)
			result[index].histogram = &histogram
		}
	}
	return result
}

func cloneFamilies(source []Family) []Family {
	result := make([]Family, len(source))
	for index, family := range source {
		result[index] = family
		result[index].series = cloneSeries(family.series)
	}
	return result
}

func cloneValidated(source ValidatedSnapshot) ValidatedSnapshot {
	return ValidatedSnapshot{families: cloneFamilies(source.families), canonical: bytes.Clone(source.canonical), series: source.series}
}
