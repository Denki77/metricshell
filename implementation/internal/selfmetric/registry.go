package selfmetric

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/lifecycle"
	applicationsnapshot "github.com/Denki77/metricshell/implementation/internal/snapshot"
)

type MetricType string

const (
	Gauge     MetricType = "gauge"
	Counter   MetricType = "counter"
	Histogram MetricType = "histogram"
)

const (
	BuildInfo                  = "metricshell_build_info"
	UptimeSeconds              = "metricshell_uptime_seconds"
	RuntimeState               = "metricshell_runtime_state"
	RuntimeFailuresTotal       = "metricshell_runtime_failures_total"
	WorkloadRunning            = "metricshell_workload_running"
	WorkloadProcessID          = "metricshell_workload_process_id"
	WorkloadStartsTotal        = "metricshell_workload_starts_total"
	WorkloadExitCode           = "metricshell_workload_exit_code"
	WorkloadSignalsTotal       = "metricshell_workload_signals_forwarded_total"
	WorkloadForcedTotal        = "metricshell_workload_forced_terminations_total"
	ChildrenReapedTotal        = "metricshell_children_reaped_total"
	SnapshotGeneration         = "metricshell_snapshot_generation"
	SnapshotSeries             = "metricshell_snapshot_series"
	SnapshotBytes              = "metricshell_snapshot_bytes"
	SnapshotPublicationsTotal  = "metricshell_snapshot_publications_total"
	SnapshotRejectionsTotal    = "metricshell_snapshot_rejections_total"
	IngestionInflight          = "metricshell_ingestion_inflight"
	IngestionConnections       = "metricshell_ingestion_connections"
	IngestionLastSuccess       = "metricshell_ingestion_last_success_timestamp_seconds"
	FileReconciliationsTotal   = "metricshell_file_reconciliations_total"
	FileWatchEventsTotal       = "metricshell_file_watch_events_total"
	SocketTransactionsInflight = "metricshell_socket_transactions_inflight"
	SocketFramesRejectedTotal  = "metricshell_socket_frames_rejected_total"
	FilterRules                = "metricshell_filter_rules"
	FilterFamilies             = "metricshell_filter_families"
	ExpositionRequestsTotal    = "metricshell_exposition_requests_total"
	ExpositionInflight         = "metricshell_exposition_inflight"
	ExpositionResponseBytes    = "metricshell_exposition_response_bytes"
	FinalWaitActive            = "metricshell_final_wait_active"
	FinalWaitModeInfo          = "metricshell_final_wait_mode_info"
	FinalWaitRequiredScrapes   = "metricshell_final_wait_required_scrapes"
	FinalWaitCompletedScrapes  = "metricshell_final_wait_completed_scrapes"
	FinalScrapeAttemptsTotal   = "metricshell_final_scrape_attempts_total"
	FinalWaitCompletionsTotal  = "metricshell_final_wait_completions_total"
	FinalWaitDeadline          = "metricshell_final_wait_deadline_timestamp_seconds"
	ShutdownActive             = "metricshell_shutdown_active"
	ShutdownDeadline           = "metricshell_shutdown_deadline_timestamp_seconds"
	ShutdownPhaseDuration      = "metricshell_shutdown_phase_duration_seconds"
)

type FinalWaitMode string

const (
	FinalWaitImmediate FinalWaitMode = "immediate"
	FinalWaitDuration  FinalWaitMode = "duration"
	FinalWaitScrapes   FinalWaitMode = "scrapes"

	RuntimeFailureConfiguration = "configuration"
	RuntimeFailureBind          = "bind"
	RuntimeFailureWorkloadStart = "workload_start"
	RuntimeFailureProtocol      = "protocol"
	RuntimeFailureResource      = "resource"
	RuntimeFailureInternal      = "internal"

	WorkloadOutcomeStarted     = "started"
	WorkloadOutcomeStartFailed = "start_failed"
	SignalTargetProcess        = "process"
	SignalTargetProcessGroup   = "process_group"
)

var (
	FinalWaitModes        = [...]string{"immediate", "duration", "scrapes"}
	RuntimeFailureReasons = [...]string{RuntimeFailureConfiguration, RuntimeFailureBind, RuntimeFailureWorkloadStart, RuntimeFailureProtocol, RuntimeFailureResource, RuntimeFailureInternal}
	WorkloadStartOutcomes = [...]string{WorkloadOutcomeStarted, WorkloadOutcomeStartFailed}
	Signals               = [...]string{"TERM", "INT", "HUP", "QUIT", "KILL"}
	SignalTargets         = [...]string{SignalTargetProcess, SignalTargetProcessGroup}
	ChildKinds            = [...]string{"direct", "adopted"}
	Transports            = [...]string{"file", "unix", "http"}
	PublicationOutcomes   = [...]string{"accepted", "rejected", "busy", "timeout", "internal_error"}
	FileTriggers          = [...]string{"startup", "event", "periodic", "overflow", "watch_reinstall"}
	FileOutcomes          = [...]string{"accepted", "unchanged", "absent", "invalid", "error"}
	FileWatchEvents       = [...]string{"overflow", "invalidated", "reinstalled"}
	SocketFrameReasons    = [...]string{"malformed", "protocol_version", "frame_limit", "part_limit", "duplicate_part", "missing_part", "transaction_invalid", "transaction_expired"}
	FilterKinds           = [...]string{"include", "exclude"}
	FilterOutcomes        = [...]string{"included", "excluded"}
	ExpositionFormats     = [...]string{"prometheus", "openmetrics"}
	ExpositionOutcomes    = [...]string{"success", "write_error", "response_limit", "encoding_error", "timeout"}
	FinalScrapeOutcomes   = [...]string{"completed", "ineligible", "write_error", "cancelled"}
	FinalWaitReasons      = [...]string{"immediate", "duration_elapsed", "required_scrapes", "timeout", "external_termination", "runtime_failure"}
	ShutdownPhases        = [...]string{"workload_wait", "forced_termination", "finalization", "http_drain", "total"}
	ShutdownBuckets       = [...]float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60}
)

var (
	ErrUnknownMetric = errors.New("unknown self-metric")
	ErrMetricType    = errors.New("self-metric type mismatch")
	ErrLabels        = errors.New("invalid self-metric labels")
	ErrValue         = errors.New("invalid self-metric value")
	ErrManaged       = errors.New("self-metric requires an atomic domain update")
)

type Label struct {
	Name  string
	Value string
}

type Bucket struct {
	UpperBound float64
	Count      uint64
}

type HistogramValue struct {
	Count   uint64
	Sum     float64
	Buckets []Bucket
}

type Sample struct {
	Labels    []Label
	Gauge     float64
	Counter   uint64
	Histogram *HistogramValue
}

type Family struct {
	Name    string
	Help    string
	Type    MetricType
	Samples []Sample
}

type View struct {
	Families []Family
}

type definition struct {
	name       string
	help       string
	typeID     MetricType
	labelNames []string
	allowed    [][]string
	buckets    []float64
	managed    bool
}

type seriesValue struct {
	labels    []Label
	gauge     float64
	counter   uint64
	histogram HistogramValue
}

type familyValue struct {
	definition definition
	series     map[string]*seriesValue
	order      []string
}

type Registry struct {
	mu       sync.RWMutex
	started  time.Time
	now      func() time.Time
	families map[string]*familyValue
	order    []string
}

func New(identity buildinfo.Info, mode FinalWaitMode, now func() time.Time) (*Registry, error) {
	if now == nil || !contains(FinalWaitModes[:], string(mode)) {
		return nil, ErrValue
	}
	registry, err := newRegistry(definitions(), now)
	if err != nil {
		return nil, err
	}
	registry.started = now()
	registry.addSpecialSeries(BuildInfo, []Label{{Name: "version", Value: identity.Version}, {Name: "revision", Value: identity.Revision}}, 1)
	registry.mustSetManagedGauge(WorkloadExitCode, nil, -1)
	registry.setOneHot(RuntimeState, "state", string(lifecycle.Initializing))
	registry.setOneHot(FinalWaitModeInfo, "mode", string(mode))
	registry.SetActiveSnapshot(applicationsnapshot.NewActive(applicationsnapshot.Zero(), 0))
	return registry, nil
}

func newRegistry(definitions []definition, now func() time.Time) (*Registry, error) {
	registry := &Registry{now: now, families: make(map[string]*familyValue, len(definitions)), order: make([]string, 0, len(definitions))}
	for _, item := range definitions {
		if err := validateDefinition(item); err != nil {
			return nil, err
		}
		if _, exists := registry.families[item.name]; exists {
			return nil, ErrUnknownMetric
		}
		family := &familyValue{definition: item, series: make(map[string]*seriesValue)}
		if item.name != BuildInfo {
			for _, labels := range labelCombinations(item) {
				value := &seriesValue{labels: labels}
				if item.typeID == Histogram {
					value.histogram.Buckets = make([]Bucket, len(item.buckets))
					for index, upperBound := range item.buckets {
						value.histogram.Buckets[index].UpperBound = upperBound
					}
				}
				key := labelsKey(labels)
				family.series[key] = value
				family.order = append(family.order, key)
			}
		}
		registry.families[item.name] = family
		registry.order = append(registry.order, item.name)
	}
	return registry, nil
}

func (registry *Registry) addSpecialSeries(name string, labels []Label, value float64) {
	family := registry.families[name]
	key := labelsKey(labels)
	family.series[key] = &seriesValue{labels: cloneLabels(labels), gauge: value}
	family.order = append(family.order, key)
}

func (registry *Registry) SetGauge(name string, labels map[string]string, value float64) error {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return ErrValue
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	family, series, err := registry.resolve(name, labels, Gauge)
	if err != nil {
		return err
	}
	if family.definition.managed {
		return ErrManaged
	}
	if isBinaryGauge(name) && value != 0 && value != 1 {
		return ErrValue
	}
	if isIntegralGauge(name) && math.Trunc(value) != value {
		return ErrValue
	}
	series.gauge = value
	return nil
}

func (registry *Registry) AddCounter(name string, labels map[string]string, delta uint64) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	_, series, err := registry.resolve(name, labels, Counter)
	if err != nil {
		return err
	}
	if ^uint64(0)-series.counter < delta {
		return ErrValue
	}
	series.counter += delta
	return nil
}

func (registry *Registry) Observe(name string, labels map[string]string, value float64) error {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return ErrValue
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	_, series, err := registry.resolve(name, labels, Histogram)
	if err != nil {
		return err
	}
	if series.histogram.Count == ^uint64(0) {
		return ErrValue
	}
	nextSum := series.histogram.Sum + value
	if math.IsInf(nextSum, 0) {
		return ErrValue
	}
	series.histogram.Count++
	series.histogram.Sum = nextSum
	for index := range series.histogram.Buckets {
		if value <= series.histogram.Buckets[index].UpperBound {
			series.histogram.Buckets[index].Count++
		}
	}
	return nil
}

func (registry *Registry) SetRuntimeState(state lifecycle.State) error {
	if !containsStates(state) {
		return ErrValue
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.setOneHot(RuntimeState, "state", string(state))
	return nil
}

func (registry *Registry) SetFinalWaitMode(mode FinalWaitMode) error {
	if !contains(FinalWaitModes[:], string(mode)) {
		return ErrValue
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.setOneHot(FinalWaitModeInfo, "mode", string(mode))
	return nil
}

func (registry *Registry) SetWorkload(pid int, running bool) error {
	if pid < 0 || (running && pid == 0) || (!running && pid != 0) {
		return ErrValue
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	value := float64(0)
	if running {
		value = 1
	}
	registry.mustSetManagedGauge(WorkloadRunning, nil, value)
	registry.mustSetManagedGauge(WorkloadProcessID, nil, float64(pid))
	return nil
}

func (registry *Registry) SetWorkloadExitCode(exitCode int) error {
	if exitCode < 0 || exitCode > 255 {
		return ErrValue
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.mustSetManagedGauge(WorkloadExitCode, nil, float64(exitCode))
	return nil
}

func (registry *Registry) SetActiveSnapshot(active applicationsnapshot.ActiveSnapshot) {
	validated := active.Validated()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.mustSetManagedGauge(SnapshotGeneration, nil, float64(active.Generation()))
	registry.mustSetManagedGauge(SnapshotSeries, nil, float64(validated.SeriesCount()))
	registry.mustSetManagedGauge(SnapshotBytes, nil, float64(validated.CanonicalBytes()))
}

func (registry *Registry) View() View {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	now := registry.now()
	families := make([]Family, 0, len(registry.order))
	for _, name := range registry.order {
		stored := registry.families[name]
		family := Family{Name: stored.definition.name, Help: stored.definition.help, Type: stored.definition.typeID, Samples: make([]Sample, 0, len(stored.order))}
		for _, key := range stored.order {
			value := stored.series[key]
			sample := Sample{Labels: cloneLabels(value.labels), Gauge: value.gauge, Counter: value.counter}
			if name == UptimeSeconds {
				sample.Gauge = max(0, now.Sub(registry.started).Seconds())
			}
			if stored.definition.typeID == Histogram {
				histogram := value.histogram
				histogram.Buckets = append([]Bucket(nil), value.histogram.Buckets...)
				sample.Histogram = &histogram
			}
			family.Samples = append(family.Samples, sample)
		}
		families = append(families, family)
	}
	return View{Families: families}
}

func (registry *Registry) resolve(name string, labels map[string]string, metricType MetricType) (*familyValue, *seriesValue, error) {
	family, exists := registry.families[name]
	if !exists {
		return nil, nil, ErrUnknownMetric
	}
	if family.definition.typeID != metricType {
		return nil, nil, ErrMetricType
	}
	if len(labels) != len(family.definition.labelNames) {
		return nil, nil, ErrLabels
	}
	ordered := make([]Label, len(family.definition.labelNames))
	for index, labelName := range family.definition.labelNames {
		value, exists := labels[labelName]
		if !exists || !contains(family.definition.allowed[index], value) {
			return nil, nil, ErrLabels
		}
		ordered[index] = Label{Name: labelName, Value: value}
	}
	series, exists := family.series[labelsKey(ordered)]
	if !exists {
		return nil, nil, ErrLabels
	}
	return family, series, nil
}

func (registry *Registry) setOneHot(name, labelName, selected string) {
	family := registry.families[name]
	for _, key := range family.order {
		value := family.series[key]
		value.gauge = 0
		if value.labels[0].Name == labelName && value.labels[0].Value == selected {
			value.gauge = 1
		}
	}
}

func (registry *Registry) mustSetManagedGauge(name string, labels map[string]string, value float64) {
	_, series, err := registry.resolve(name, labels, Gauge)
	if err != nil {
		panic(err)
	}
	series.gauge = value
}

func definitions() []definition {
	states := make([]string, len(lifecycle.States))
	for index, state := range lifecycle.States {
		states[index] = string(state)
	}
	rejections := make([]string, len(applicationsnapshot.RejectionReasons))
	for index, reason := range applicationsnapshot.RejectionReasons {
		rejections[index] = string(reason)
	}
	return []definition{
		{BuildInfo, "MetricShell build identity.", Gauge, []string{"version", "revision"}, nil, nil, true},
		{UptimeSeconds, "Seconds since the MetricShell process started.", Gauge, nil, nil, nil, true},
		{RuntimeState, "Current MetricShell runtime state as a one-hot vector.", Gauge, []string{"state"}, [][]string{states}, nil, true},
		{RuntimeFailuresTotal, "Unrecoverable MetricShell failures.", Counter, []string{"reason"}, [][]string{RuntimeFailureReasons[:]}, nil, false},
		{WorkloadRunning, "Whether the primary workload is running.", Gauge, nil, nil, nil, true},
		{WorkloadProcessID, "Primary workload process identifier, or zero.", Gauge, nil, nil, nil, true},
		{WorkloadStartsTotal, "Primary workload start attempts.", Counter, []string{"outcome"}, [][]string{WorkloadStartOutcomes[:]}, nil, false},
		{WorkloadExitCode, "Resolved process-compatible workload exit code, or minus one.", Gauge, nil, nil, nil, true},
		{WorkloadSignalsTotal, "Signals forwarded to the workload.", Counter, []string{"signal", "target"}, [][]string{Signals[:], SignalTargets[:]}, nil, false},
		{WorkloadForcedTotal, "Forced workload or process-group terminations.", Counter, nil, nil, nil, false},
		{ChildrenReapedTotal, "Child processes reaped by MetricShell.", Counter, []string{"kind"}, [][]string{ChildKinds[:]}, nil, false},
		{SnapshotGeneration, "Accepted application snapshot generation.", Gauge, nil, nil, nil, true},
		{SnapshotSeries, "Application series in the active complete snapshot.", Gauge, nil, nil, nil, true},
		{SnapshotBytes, "Canonical bytes in the active application snapshot.", Gauge, nil, nil, nil, true},
		{SnapshotPublicationsTotal, "Complete application snapshot publication outcomes.", Counter, []string{"transport", "outcome"}, [][]string{Transports[:], PublicationOutcomes[:]}, nil, false},
		{SnapshotRejectionsTotal, "Atomic application candidate rejections.", Counter, []string{"transport", "reason"}, [][]string{Transports[:], rejections}, nil, false},
		{IngestionInflight, "Currently executing ingestion operations.", Gauge, []string{"transport"}, [][]string{Transports[:]}, nil, false},
		{IngestionConnections, "Currently accepted ingestion connections.", Gauge, []string{"transport"}, [][]string{Transports[:]}, nil, false},
		{IngestionLastSuccess, "Timestamp of the last accepted application snapshot.", Gauge, []string{"transport"}, [][]string{Transports[:]}, nil, false},
		{FileReconciliationsTotal, "File snapshot reconciliation attempts.", Counter, []string{"trigger", "outcome"}, [][]string{FileTriggers[:], FileOutcomes[:]}, nil, false},
		{FileWatchEventsTotal, "File watch lifecycle events.", Counter, []string{"event"}, [][]string{FileWatchEvents[:]}, nil, false},
		{SocketTransactionsInflight, "Retained multipart socket transactions.", Gauge, nil, nil, nil, false},
		{SocketFramesRejectedTotal, "Rejected socket protocol frames.", Counter, []string{"reason"}, [][]string{SocketFrameReasons[:]}, nil, false},
		{FilterRules, "Effective normalized metric filter rules.", Gauge, []string{"kind"}, [][]string{FilterKinds[:]}, nil, false},
		{FilterFamilies, "Families in the current filtered exposition view.", Gauge, []string{"outcome"}, [][]string{FilterOutcomes[:]}, nil, false},
		{ExpositionRequestsTotal, "Metric exposition response outcomes.", Counter, []string{"format", "outcome"}, [][]string{ExpositionFormats[:], ExpositionOutcomes[:]}, nil, false},
		{ExpositionInflight, "Active metric exposition handlers.", Gauge, nil, nil, nil, false},
		{ExpositionResponseBytes, "Last complete uncompressed exposition response size.", Gauge, []string{"format"}, [][]string{ExpositionFormats[:]}, nil, false},
		{FinalWaitActive, "Whether natural-completion final wait is active.", Gauge, nil, nil, nil, false},
		{FinalWaitModeInfo, "Effective final-wait mode as a one-hot vector.", Gauge, []string{"mode"}, [][]string{FinalWaitModes[:]}, nil, true},
		{FinalWaitRequiredScrapes, "Effective required final scrape count.", Gauge, nil, nil, nil, false},
		{FinalWaitCompletedScrapes, "Completed eligible final scrape responses.", Gauge, nil, nil, nil, false},
		{FinalScrapeAttemptsTotal, "Final-state scrape attempt outcomes.", Counter, []string{"outcome"}, [][]string{FinalScrapeOutcomes[:]}, nil, false},
		{FinalWaitCompletionsTotal, "Final-wait terminal reasons.", Counter, []string{"reason"}, [][]string{FinalWaitReasons[:]}, nil, false},
		{FinalWaitDeadline, "Absolute final-wait deadline timestamp.", Gauge, nil, nil, nil, false},
		{ShutdownActive, "Whether external termination is active.", Gauge, nil, nil, nil, false},
		{ShutdownDeadline, "Effective absolute external shutdown deadline.", Gauge, nil, nil, nil, false},
		{ShutdownPhaseDuration, "Completed shutdown phase durations.", Histogram, []string{"phase"}, [][]string{ShutdownPhases[:]}, ShutdownBuckets[:], false},
	}
}

var metricNamePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

func validateDefinition(item definition) error {
	if !strings.HasPrefix(item.name, "metricshell_") || !metricNamePattern.MatchString(item.name) || item.help == "" {
		return ErrUnknownMetric
	}
	if item.typeID != Gauge && item.typeID != Counter && item.typeID != Histogram {
		return ErrMetricType
	}
	if item.typeID == Counter && !strings.HasSuffix(item.name, "_total") {
		return ErrMetricType
	}
	if item.typeID != Counter && strings.HasSuffix(item.name, "_total") {
		return ErrMetricType
	}
	if item.name == BuildInfo {
		if item.typeID != Gauge || !item.managed || len(item.labelNames) != 2 || item.labelNames[0] != "version" || item.labelNames[1] != "revision" || len(item.allowed) != 0 || len(item.buckets) != 0 {
			return ErrLabels
		}
		return nil
	}
	if len(item.labelNames) != len(item.allowed) {
		return ErrLabels
	}
	seen := make(map[string]struct{}, len(item.labelNames))
	for index, name := range item.labelNames {
		if _, exists := seen[name]; exists || name == "" || len(item.allowed[index]) == 0 {
			return ErrLabels
		}
		seen[name] = struct{}{}
		values := make(map[string]struct{}, len(item.allowed[index]))
		for _, value := range item.allowed[index] {
			if value == "" {
				return ErrLabels
			}
			if _, exists := values[value]; exists {
				return ErrLabels
			}
			values[value] = struct{}{}
		}
	}
	if item.typeID == Histogram {
		if len(item.buckets) == 0 {
			return ErrValue
		}
		for index, bound := range item.buckets {
			if bound <= 0 || math.IsNaN(bound) || math.IsInf(bound, 0) || (index > 0 && bound <= item.buckets[index-1]) {
				return ErrValue
			}
		}
	} else if len(item.buckets) != 0 {
		return ErrValue
	}
	return nil
}

func labelCombinations(item definition) [][]Label {
	if len(item.labelNames) == 0 {
		return [][]Label{{}}
	}
	result := make([][]Label, 0)
	var appendLabels func(int, []Label)
	appendLabels = func(index int, labels []Label) {
		if index == len(item.labelNames) {
			result = append(result, cloneLabels(labels))
			return
		}
		for _, value := range item.allowed[index] {
			appendLabels(index+1, append(labels, Label{Name: item.labelNames[index], Value: value}))
		}
	}
	appendLabels(0, nil)
	return result
}

func labelsKey(labels []Label) string {
	var result strings.Builder
	for _, label := range labels {
		result.WriteString(strconv.Itoa(len(label.Name)))
		result.WriteByte(':')
		result.WriteString(label.Name)
		result.WriteString(strconv.Itoa(len(label.Value)))
		result.WriteByte(':')
		result.WriteString(label.Value)
	}
	return result.String()
}

func cloneLabels(source []Label) []Label {
	return append([]Label(nil), source...)
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsStates(wanted lifecycle.State) bool {
	for _, state := range lifecycle.States {
		if state == wanted {
			return true
		}
	}
	return false
}

func isBinaryGauge(name string) bool {
	return name == FinalWaitActive || name == ShutdownActive
}

func isIntegralGauge(name string) bool {
	switch name {
	case IngestionInflight, IngestionConnections, SocketTransactionsInflight, FilterRules, FilterFamilies,
		ExpositionInflight, ExpositionResponseBytes, FinalWaitRequiredScrapes, FinalWaitCompletedScrapes:
		return true
	default:
		return isBinaryGauge(name)
	}
}
