package config

import (
	"reflect"
	"testing"
)

func TestParseWorkload(t *testing.T) {
	t.Parallel()

	want := []string{"program", "space value", "", "Привет", "--version", "--"}
	config, err := Parse(append([]string{"--"}, want...))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !reflect.DeepEqual(config.Workload, want) {
		t.Fatalf("Workload = %#v, want %#v", config.Workload, want)
	}
}

func TestParseRejectsInvalidCommandLine(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{nil, {"--"}, {"program"}, {"--unknown", "--", "program"}} {
		if _, err := Parse(args); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", args)
		}
	}
}
