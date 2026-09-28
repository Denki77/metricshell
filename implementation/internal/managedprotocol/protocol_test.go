package managedprotocol

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/Denki77/metricshell/implementation/internal/managed"
)

func TestProtocolGoldenCorpus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		frame string
		code  Code
	}{
		{name: "empty", frame: "\n", code: CodeEmptyFrame},
		{name: "partial", frame: `{}`, code: CodePartialFrame},
		{name: "malformed", frame: "{]\n", code: CodeMalformedJSON},
		{name: "duplicate", frame: "{\"version\":1,\"version\":1}\n", code: CodeMalformedJSON},
		{name: "missing version", frame: "{\"op\":\"gauge_set\"}\n", code: CodeMissingVersion},
		{name: "invalid version", frame: "{\"version\":\"1\",\"op\":\"gauge_set\"}\n", code: CodeInvalidVersion},
		{name: "unsupported version", frame: "{\"version\":2,\"op\":\"gauge_set\"}\n", code: CodeUnsupportedVersion},
		{name: "unknown operation", frame: "{\"version\":1,\"op\":\"reset\"}\n", code: CodeInvalidRequest},
		{name: "multiple", frame: "{}\n{}\n", code: CodeMultipleFrames},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseFrame([]byte(test.frame), 1024)
			got, ok := ErrorCode(err)
			if !ok || got != test.code {
				t.Fatalf("code = %q, %v; want %q; error %v", got, ok, test.code, err)
			}
		})
	}
}

func TestProtocolParsesDomainMutations(t *testing.T) {
	t.Parallel()

	declaration, err := ParseFrame([]byte(`{"version":1,"op":"declare","name":"latency","help":"Latency.","type":"histogram","label_names":["route"],"buckets":["0.5","+Inf"]}`+"\n"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if declaration.Descriptor == nil || declaration.Descriptor.Name != "latency" || !math.IsInf(declaration.Descriptor.Buckets[1], 1) {
		t.Fatalf("declaration = %+v", declaration)
	}
	operation, err := ParseFrame([]byte(`{"version":1,"op":"histogram_observe","name":"latency","labels":{"route":"/jobs"},"value":"0.25"}`+"\n"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(operation.Operations) != 1 || operation.Operations[0].Kind != managed.HistogramObserve || operation.Operations[0].Value != 0.25 {
		t.Fatalf("operation = %+v", operation)
	}
}

func TestProtocolParsesPrometheusSpecialNumbers(t *testing.T) {
	for _, test := range []struct {
		token string
		check func(float64) bool
	}{
		{"NaN", math.IsNaN},
		{"+Inf", func(value float64) bool { return math.IsInf(value, 1) }},
		{"-Inf", func(value float64) bool { return math.IsInf(value, -1) }},
	} {
		frame := `{"version":1,"op":"gauge_set","name":"depth","labels":{},"value":"` + test.token + `"}` + "\n"
		mutation, err := ParseFrame([]byte(frame), 1024)
		if err != nil || len(mutation.Operations) != 1 || !test.check(mutation.Operations[0].Value) {
			t.Fatalf("token=%s mutation=%+v err=%v", test.token, mutation, err)
		}
	}
}

func TestFrameLimitIsExactAndBounded(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"version":1,"op":"gauge_set","name":"depth","labels":{},"value":"1"}`)
	if _, err := ParseFrame(append(append([]byte(nil), payload...), '\n'), len(payload)); err != nil {
		t.Fatalf("exact limit: %v", err)
	}
	if _, err := ParseFrame(append(append([]byte(nil), payload...), '\n'), len(payload)-1); errorCode(t, err) != CodeFrameTooLarge {
		t.Fatalf("limit+1 error = %v", err)
	}
	content, err := ReadFrame(strings.NewReader(strings.Repeat("x", 1024)), 8)
	if errorCode(t, err) != CodeFrameTooLarge || content != nil {
		t.Fatalf("bounded read = %d bytes, %v", len(content), err)
	}
}

func FuzzParseFrame(f *testing.F) {
	f.Add([]byte("\n"), 8)
	f.Add([]byte(`{"version":1}`+"\n"), 64)
	f.Fuzz(func(t *testing.T, frame []byte, maximum int) {
		if maximum < 1 || maximum > 4096 {
			return
		}
		_, _ = ParseFrame(frame, maximum)
	})
}

func TestSuccessRequiresCommittedOwnerResult(t *testing.T) {
	t.Parallel()

	frame := []byte(`{"version":1,"op":"counter_add","name":"jobs","labels":{},"value":"1"}` + "\n")
	for _, outcome := range []managed.Outcome{managed.OutcomeRejected, managed.OutcomeOverloaded, managed.OutcomeCancelled, managed.OutcomeClosed, managed.OutcomeUnknown} {
		submitter := stubSubmitter{result: managed.Result{Outcome: outcome, Generation: 7, Reason: managed.ReasonUndeclared}}
		response := Handle(context.Background(), submitter, frame, 1024)
		if response.Outcome != outcome || response.Outcome == managed.OutcomeCommitted {
			t.Fatalf("response = %+v", response)
		}
	}
	committed := Handle(context.Background(), stubSubmitter{result: managed.Result{Outcome: managed.OutcomeCommitted, Generation: 8, Commit: 3}}, frame, 1024)
	if committed.Outcome != managed.OutcomeCommitted || committed.Generation != 8 || committed.Commit != 3 {
		t.Fatalf("committed response = %+v", committed)
	}
	protocol := Handle(context.Background(), stubSubmitter{}, []byte("{}"), 1024)
	if protocol.Outcome != "protocol" || protocol.Reason != string(CodePartialFrame) {
		t.Fatalf("protocol response = %+v", protocol)
	}
}

type stubSubmitter struct{ result managed.Result }

func (stub stubSubmitter) Submit(context.Context, managed.Mutation) managed.Result {
	return stub.result
}

func errorCode(t *testing.T, err error) Code {
	t.Helper()
	code, _ := ErrorCode(err)
	return code
}
