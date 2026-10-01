package exposition

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

const applicationCandidate = `{"schema_version":1,"families":[` +
	`{"name":"jobs","help":"completed jobs","type":"counter","series":[{"labels":{"queue":"main"},"value":"7"},{"labels":{"queue":"overflow"},"value":"+Inf"}]},` +
	`{"name":"temperature","help":"line\\help\nnext","type":"gauge","series":[{"labels":{"kind":"quote\\\""},"value":"NaN"},{"labels":{"kind":"positive"},"value":"+Inf"},{"labels":{"kind":"negative"},"value":"-Inf"},{"labels":{"kind":"negative-zero"},"value":"-0"}]},` +
	`{"name":"latency","help":"seconds","type":"histogram","series":[{"labels":{},"histogram":{"count":"7","sum":"-0.5","buckets":[{"le":"-10","count":"1"},{"le":"-1","count":"3"},{"le":"0","count":"4"},{"le":"1","count":"5"},{"le":"10","count":"6"},{"le":"+Inf","count":"7"}]}}]}` +
	`]}`

func TestPrepareFormatsBoundsAndCompression(t *testing.T) {
	active, metrics := testState(t)
	for _, format := range []Format{Prometheus, OpenMetrics} {
		t.Run(string(format), func(t *testing.T) {
			plain, err := Prepare(active, metrics, nil, format, Identity, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			body := string(plain.Body())
			for _, wanted := range []string{
				`jobs_total{queue="main"} 7`,
				`jobs_total{queue="overflow"} +Inf`, `temperature{kind="negative-zero"} -0`,
				`temperature{kind="positive"} +Inf`, `temperature{kind="negative"} -Inf`,
				`latency_bucket{le="-10"} 1`, `latency_bucket{le="+Inf"} 7`, "metricshell_build_info",
			} {
				if !strings.Contains(body, wanted) {
					t.Errorf("body does not contain %q:\n%s", wanted, body)
				}
			}
			metadataName := "jobs"
			if format == Prometheus {
				metadataName = "jobs_total"
			}
			if !strings.Contains(body, "# HELP "+metadataName+" completed jobs\n") || !strings.Contains(body, "# TYPE "+metadataName+" counter\n") {
				t.Errorf("format %s has incorrect counter metadata name:\n%s", format, body)
			}
			if format == Prometheus && !strings.Contains(body, "latency_sum -0.5\n") {
				t.Errorf("Prometheus output omitted negative histogram sum:\n%s", body)
			}
			if format == OpenMetrics && strings.Contains(body, "latency_sum ") {
				t.Errorf("OpenMetrics 1.0 output emitted sum for negative thresholds:\n%s", body)
			}
			if got := strings.Count(body, `latency_bucket{le="+Inf"} 7`); got != 1 {
				t.Fatalf("terminal bucket emitted %d times:\n%s", got, body)
			}
			if strings.HasSuffix(body, "# EOF\n") != (format == OpenMetrics) {
				t.Fatalf("format %s EOF mismatch", format)
			}
			if _, err := Prepare(active, metrics, nil, format, Identity, plain.UncompressedBytes()); err != nil {
				t.Fatalf("exact limit failed: %v", err)
			}
			if _, err := Prepare(active, metrics, nil, format, Identity, plain.UncompressedBytes()-1); err != ErrResponseLimit {
				t.Fatalf("limit-1 error = %v", err)
			}

			compressed, err := Prepare(active, metrics, nil, format, Gzip, plain.UncompressedBytes())
			if err != nil {
				t.Fatal(err)
			}
			reader, err := gzip.NewReader(bytes.NewReader(compressed.Body()))
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decoded, plain.Body()) || compressed.Generation() != plain.Generation() || compressed.UncompressedBytes() != plain.UncompressedBytes() {
				t.Fatal("gzip changed response identity")
			}
		})
	}
}

func TestCounterMetadataIsFormatSpecific(t *testing.T) {
	validated := parseApplication(t, `{"schema_version":1,"families":[`+
		`{"name":"requests","help":"completed requests","type":"counter","series":[{"labels":{},"value":"42"}]}`+
		`]}`)

	tests := []struct {
		name   string
		format Format
		want   string
	}{
		{
			name:   "prometheus",
			format: Prometheus,
			want:   "# HELP requests_total completed requests\n# TYPE requests_total counter\nrequests_total 42\n",
		},
		{
			name:   "openmetrics",
			format: OpenMetrics,
			want:   "# HELP requests completed requests\n# TYPE requests counter\nrequests_total 42\n# EOF\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := Encode(validated, selfmetric.View{}, nil, test.format)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(body); got != test.want {
				t.Fatalf("encoded counter mismatch\ngot:\n%s\nwant:\n%s", got, test.want)
			}
		})
	}
}

func TestPrepareSelfMetricsOnlyAndInvalidFormat(t *testing.T) {
	active, metrics := testState(t)
	prepared, err := Prepare(active, metrics, func(snapshot.Family) bool { return false }, Prometheus, Identity, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(prepared.Body()), "jobs_total") || !strings.Contains(string(prepared.Body()), "metricshell_build_info") {
		t.Fatalf("unexpected filtered response:\n%s", prepared.Body())
	}
	if _, err := Prepare(active, metrics, nil, Format("invalid"), Identity, 1<<20); err == nil {
		t.Fatal("invalid format accepted")
	}
}

func TestHistogramSumsAreFormatSpecificAndOpenMetrics10Valid(t *testing.T) {
	validated := parseApplication(t, `{"schema_version":1,"families":[`+
		`{"name":"negative_value","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"1","sum":"-2","buckets":[{"le":"0","count":"1"},{"le":"+Inf","count":"1"}]}}]},`+
		`{"name":"negative_boundary","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"1","sum":"2","buckets":[{"le":"-1","count":"0"},{"le":"+Inf","count":"1"}]}}]},`+
		`{"name":"nan_value","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"1","sum":"NaN","buckets":[{"le":"0","count":"0"},{"le":"+Inf","count":"1"}]}}]},`+
		`{"name":"positive_inf_value","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"1","sum":"+Inf","buckets":[{"le":"0","count":"0"},{"le":"+Inf","count":"1"}]}}]},`+
		`{"name":"negative_inf_value","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"1","sum":"-Inf","buckets":[{"le":"0","count":"1"},{"le":"+Inf","count":"1"}]}}]}`+
		`]}`)

	prometheusBody, err := Encode(validated, selfmetric.View{}, nil, Prometheus)
	if err != nil {
		t.Fatal(err)
	}
	openMetricsBody, err := Encode(validated, selfmetric.View{}, nil, OpenMetrics)
	if err != nil {
		t.Fatal(err)
	}
	prometheusText, openMetricsText := string(prometheusBody), string(openMetricsBody)
	for _, sample := range []string{
		"negative_value_sum -2\n", "negative_boundary_sum 2\n", "nan_value_sum NaN\n",
		"positive_inf_value_sum +Inf\n", "negative_inf_value_sum -Inf\n",
	} {
		if !strings.Contains(prometheusText, sample) {
			t.Errorf("Prometheus output omitted %q:\n%s", sample, prometheusText)
		}
	}
	for _, forbidden := range []string{"negative_value_sum ", "negative_boundary_sum ", "nan_value_sum ", "negative_inf_value_sum "} {
		if strings.Contains(openMetricsText, forbidden) {
			t.Errorf("OpenMetrics 1.0 emitted forbidden sum %q:\n%s", forbidden, openMetricsText)
		}
	}
	if !strings.Contains(openMetricsText, "positive_inf_value_sum +Inf\n") {
		t.Fatalf("OpenMetrics 1.0 omitted permitted +Inf sum:\n%s", openMetricsText)
	}
	for _, family := range []string{"negative_value", "negative_boundary", "nan_value", "positive_inf_value", "negative_inf_value"} {
		if strings.Count(openMetricsText, family+`_bucket{le="+Inf"} 1`) != 1 || !strings.Contains(openMetricsText, family+"_count 1\n") {
			t.Errorf("OpenMetrics 1.0 lost count/terminal bucket for %s:\n%s", family, openMetricsText)
		}
	}
	if !strings.HasSuffix(openMetricsText, "# EOF\n") {
		t.Fatal("OpenMetrics 1.0 output lacks EOF")
	}
}

func TestWriteClassifiesCompleteShortAndCancelledResponses(t *testing.T) {
	active, metrics := testState(t)
	prepared, err := Prepare(active, metrics, nil, Prometheus, Identity, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if outcome := Write(context.Background(), recorder, prepared, time.Second); outcome != Success {
		t.Fatalf("complete outcome = %s", outcome)
	}
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Length") == "" || recorder.Header().Get("Content-Type") != ContentType(Prometheus) {
		t.Fatalf("complete response = %d %#v", recorder.Code, recorder.Header())
	}

	short := &shortResponseWriter{header: make(http.Header)}
	if outcome := Write(context.Background(), short, prepared, time.Second); outcome != WriteError {
		t.Fatalf("short outcome = %s", outcome)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	notStarted := httptest.NewRecorder()
	if outcome := Write(cancelled, notStarted, prepared, time.Second); outcome != Timeout || notStarted.Code != http.StatusOK {
		t.Fatalf("cancelled outcome/code = %s/%d", outcome, notStarted.Code)
	}
}

func TestLimiterRejectsSaturationWithoutBlocking(t *testing.T) {
	limiter, err := NewLimiter(1)
	if err != nil {
		t.Fatal(err)
	}
	if !limiter.Acquire() || limiter.Acquire() {
		t.Fatal("limiter did not enforce exact capacity")
	}
	limiter.Release()
	if !limiter.Acquire() {
		t.Fatal("released capacity was not reusable")
	}
	limiter.Release()
}

type shortResponseWriter struct {
	header http.Header
}

func (writer *shortResponseWriter) Header() http.Header               { return writer.header }
func (writer *shortResponseWriter) WriteHeader(int)                   {}
func (writer *shortResponseWriter) Write(content []byte) (int, error) { return len(content) / 2, nil }

func testState(t *testing.T) (snapshot.ActiveSnapshot, selfmetric.View) {
	t.Helper()
	validated, err := snapshot.Parse([]byte(applicationCandidate), snapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := selfmetric.New(buildinfo.Info{Version: "test", Revision: "revision"}, selfmetric.FinalWaitImmediate, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	active := snapshot.NewActive(validated, 9)
	registry.SetActiveSnapshot(active)
	return active, registry.View()
}
