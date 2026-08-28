package snapshot

import (
	"sync"
	"testing"
)

const goldenDocument = `{
  "schema_version": 1,
  "families": [
    {"name":"requests","help":"Done.","type":"counter","series":[{"labels":{"method":"GET"},"value":"42.0"}]},
    {"name":"temperature","help":"","type":"gauge","series":[{"labels":{},"value":"NaN"}]},
    {"name":"latency","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"2","sum":"1.50","buckets":[{"le":"0.5","count":"1"},{"le":"+Inf","count":"2"}]}}]}
  ]
}`

func TestParseGoldenDocument(t *testing.T) {
	snapshot, err := Parse([]byte(goldenDocument), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SeriesCount() != 3 {
		t.Fatalf("series = %d, want 3", snapshot.SeriesCount())
	}
	want := "{\"schema_version\":1,\"families\":[{\"name\":\"latency\",\"help\":\"\",\"type\":\"histogram\",\"series\":[{\"labels\":{},\"histogram\":{\"count\":\"2\",\"sum\":\"1.5\",\"buckets\":[{\"le\":\"0.5\",\"count\":\"1\"},{\"le\":\"+Inf\",\"count\":\"2\"}]}}]},{\"name\":\"requests\",\"help\":\"Done.\",\"type\":\"counter\",\"series\":[{\"labels\":{\"method\":\"GET\"},\"value\":\"42\"}]},{\"name\":\"temperature\",\"help\":\"\",\"type\":\"gauge\",\"series\":[{\"labels\":{},\"value\":\"NaN\"}]}]}\n"
	if string(snapshot.Canonical()) != want {
		t.Fatalf("canonical = %s", snapshot.Canonical())
	}
}

func TestParserRejectionCorpus(t *testing.T) {
	tests := []struct {
		name    string
		content string
		limits  Limits
		reason  Reason
	}{
		{"empty", "", DefaultLimits(), ReasonEmptyPayload},
		{"malformed", "{", DefaultLimits(), ReasonMalformed},
		{"invalid utf8", string([]byte{0xff}), DefaultLimits(), ReasonMalformed},
		{"root array", `[]`, DefaultLimits(), ReasonMalformed},
		{"trailing", `{"schema_version":1,"families":[]} {}`, DefaultLimits(), ReasonMalformed},
		{"unknown top", `{"schema_version":1,"families":[],"unknown":0}`, DefaultLimits(), ReasonMalformed},
		{"missing families", `{"schema_version":1}`, DefaultLimits(), ReasonMalformed},
		{"version", `{"schema_version":2,"families":[]}`, DefaultLimits(), ReasonSchemaVersion},
		{"version type", `{"schema_version":"1","families":[]}`, DefaultLimits(), ReasonSchemaVersion},
		{"version fractional", `{"schema_version":1.0,"families":[]}`, DefaultLimits(), ReasonSchemaVersion},
		{"null families", `{"schema_version":1,"families":null}`, DefaultLimits(), ReasonMalformed},
		{"unknown family", `{"schema_version":1,"families":[{"name":"a","help":"","type":"gauge","series":[],"x":0}]}`, DefaultLimits(), ReasonMalformed},
		{"unknown type", `{"schema_version":1,"families":[{"name":"a","help":"","type":"summary","series":[]}]}`, DefaultLimits(), ReasonPolicy},
		{"unknown series", `{"schema_version":1,"families":[{"name":"a","help":"","type":"gauge","series":[{"labels":{},"value":"1","x":0}]}]}`, DefaultLimits(), ReasonMalformed},
		{"duplicate member", `{"schema_version":1,"schema_version":1,"families":[]}`, DefaultLimits(), ReasonMalformed},
		{"duplicate label", `{"schema_version":1,"families":[{"name":"a","help":"","type":"gauge","series":[{"labels":{"x":"1","x":"2"},"value":"1"}]}]}`, DefaultLimits(), ReasonDuplicateSeries},
		{"policy", `{"schema_version":1,"families":[{"name":"1bad","help":"","type":"gauge","series":[]}]}`, DefaultLimits(), ReasonPolicy},
		{"reserved", `{"schema_version":1,"families":[{"name":"metricshell_bad","help":"","type":"gauge","series":[]}]}`, DefaultLimits(), ReasonReservedName},
		{"numeric", `{"schema_version":1,"families":[{"name":"a","help":"","type":"gauge","series":[{"labels":{},"value":"1e999"}]}]}`, DefaultLimits(), ReasonNumericInvalid},
		{"duplicate series", `{"schema_version":1,"families":[{"name":"a","help":"","type":"gauge","series":[{"labels":{},"value":"1"},{"labels":{},"value":"2"}]}]}`, DefaultLimits(), ReasonDuplicateSeries},
		{"type conflict", `{"schema_version":1,"families":[{"name":"a","help":"","type":"gauge","series":[]},{"name":"a","help":"","type":"counter","series":[]}]}`, DefaultLimits(), ReasonTypeConflict},
		{"metadata", `{"schema_version":1,"families":[{"name":"a","help":"","type":"counter","series":[]},{"name":"a_total","help":"","type":"gauge","series":[]}]}`, DefaultLimits(), ReasonMetadataConflict},
		{"histogram", `{"schema_version":1,"families":[{"name":"a","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"1","sum":"1","buckets":[]}}]}]}`, DefaultLimits(), ReasonHistogramInvalid},
		{"series limit", `{"schema_version":1,"families":[{"name":"a","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`, withLimit(func(value *Limits) { value.Series = 0 }), ReasonSeriesLimit},
		{"label limit", `{"schema_version":1,"families":[{"name":"a","help":"","type":"gauge","series":[{"labels":{"x":"1"},"value":"1"}]}]}`, withLimit(func(value *Limits) { value.LabelsPerSeries = 0 }), ReasonLabelLimit},
		{"name limit", `{"schema_version":1,"families":[{"name":"ab","help":"","type":"gauge","series":[]}]}`, withLimit(func(value *Limits) { value.MetricNameBytes = 1 }), ReasonNameLimit},
		{"decoded limit", `{"schema_version":1,"families":[]}`, withLimit(func(value *Limits) { value.DecodedBytes = 1 }), ReasonPayloadLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.content), test.limits)
			if reason, ok := RejectionReason(err); !ok || reason != test.reason {
				t.Fatalf("error = %v reason = %s, want %s", err, reason, test.reason)
			}
		})
	}
}

func TestConcurrentParsingIsRaceSafe(t *testing.T) {
	var wait sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 100; iteration++ {
				parsed, err := Parse([]byte(goldenDocument), DefaultLimits())
				if err != nil || parsed.SeriesCount() != 3 {
					t.Errorf("Parse() = series %d, err %v", parsed.SeriesCount(), err)
				}
			}
		}()
	}
	wait.Wait()
}

func TestParseAppliesDecodedAndCanonicalLimitsIndependently(t *testing.T) {
	zero := []byte(`{"schema_version":1,"families":[]}`)
	limits := DefaultLimits()
	limits.DecodedBytes = len(zero)
	parsed, err := Parse(zero, limits)
	if err != nil {
		t.Fatal(err)
	}

	limits.DecodedBytes--
	if _, err := Parse(zero, limits); rejectionReason(t, err) != ReasonPayloadLimit {
		t.Fatalf("decoded limit error = %v", err)
	}

	limits = DefaultLimits()
	limits.SnapshotBytes = parsed.CanonicalBytes()
	if _, err := Parse(zero, limits); err != nil {
		t.Fatalf("exact canonical limit: %v", err)
	}
	limits.SnapshotBytes--
	if _, err := Parse(zero, limits); rejectionReason(t, err) != ReasonPayloadLimit {
		t.Fatalf("canonical limit error = %v", err)
	}
}

func TestParseDoesNotRetainInput(t *testing.T) {
	content := []byte(`{"schema_version":1,"families":[{"name":"value","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`)
	parsed, err := Parse(content, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for index := range content {
		content[index] = 'x'
	}
	if got := string(parsed.Canonical()); got != "{\"schema_version\":1,\"families\":[{\"name\":\"value\",\"help\":\"\",\"type\":\"gauge\",\"series\":[{\"labels\":{},\"value\":\"1\"}]}]}\n" {
		t.Fatalf("canonical changed with input: %s", got)
	}
}

func rejectionReason(t *testing.T, err error) Reason {
	t.Helper()
	reason, ok := RejectionReason(err)
	if !ok {
		t.Fatalf("error %v is not a rejection", err)
	}
	return reason
}

func FuzzParseNeverPanics(fuzz *testing.F) {
	fuzz.Add([]byte(goldenDocument))
	fuzz.Add([]byte(`{"schema_version":1,"families":[]}`))
	fuzz.Add([]byte(`{"schema_version":1,"families":[`))
	fuzz.Fuzz(func(t *testing.T, content []byte) {
		_, _ = Parse(content, DefaultLimits())
	})
}
