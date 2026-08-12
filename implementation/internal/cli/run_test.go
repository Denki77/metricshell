package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
)

func TestRunVersion(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	identity := buildinfo.Info{Version: "1.2.3", Revision: "0123456"}
	code := Run([]string{"--version"}, &stdout, &stderr, identity, time.Now)

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if got, want := stdout.String(), identity.String()+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &stderr, buildinfo.Info{}, time.Now)
	if code != 0 || stdout.String() != usage || stderr.Len() != 0 {
		t.Fatalf("Run(--help) = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestRunRejectsInvalidStartupConfiguration(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	now := time.Date(2026, 8, 11, 10, 30, 0, 123, time.FixedZone("test", 3*60*60))
	code := Run(nil, &stdout, &stderr, buildinfo.Info{}, func() time.Time { return now })

	if code != 64 {
		t.Fatalf("Run() code = %d, want 64", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}

	var record map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &record); err != nil {
		t.Fatalf("stderr is not JSON Lines: %v", err)
	}
	want := map[string]any{
		"timestamp":      "2026-08-11T07:30:00.000000123Z",
		"schema_version": "1",
		"level":          "error",
		"event":          "configuration.rejected",
		"component":      "runtime",
		"state":          "initializing",
		"reason":         "configuration",
		"error_code":     "CONFIG_INVALID",
	}
	for field, expected := range want {
		if got := record[field]; got != expected {
			t.Errorf("field %s = %#v, want %#v", field, got, expected)
		}
	}
	if record["sequence"] != float64(1) {
		t.Errorf("sequence = %#v, want 1", record["sequence"])
	}
	if record["runtime_id"] == "" {
		t.Error("runtime_id is empty")
	}
}
