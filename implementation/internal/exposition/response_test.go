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
	`{"name":"jobs","help":"completed jobs","type":"counter","series":[{"labels":{"queue":"main"},"value":"7"}]},` +
	`{"name":"temperature","help":"line\\help\nnext","type":"gauge","series":[{"labels":{"kind":"quote\\\""},"value":"NaN"},{"labels":{"kind":"positive"},"value":"+Inf"},{"labels":{"kind":"negative"},"value":"-Inf"}]},` +
	`{"name":"latency","help":"seconds","type":"histogram","series":[{"labels":{},"histogram":{"count":"2","sum":"1.5","buckets":[{"le":"0.5","count":"1"},{"le":"+Inf","count":"2"}]}}]}` +
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
				"# HELP jobs completed jobs\n", "# TYPE jobs counter\n", `jobs_total{queue="main"} 7`,
				`temperature{kind="positive"} +Inf`, `temperature{kind="negative"} -Inf`,
				`latency_bucket{le="+Inf"} 2`, "metricshell_build_info",
			} {
				if !strings.Contains(body, wanted) {
					t.Errorf("body does not contain %q:\n%s", wanted, body)
				}
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
