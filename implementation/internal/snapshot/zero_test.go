package snapshot

import "testing"

func TestInitialHolderExposesGenerationZeroWithoutPublication(t *testing.T) {
	active := NewInitialHolder().Active()
	if active.Generation() != 0 {
		t.Fatalf("generation = %d, want 0", active.Generation())
	}
	validated := active.Validated()
	if !validated.IsZeroSeries() || validated.SeriesCount() != 0 {
		t.Fatalf("zero=%t series=%d", validated.IsZeroSeries(), validated.SeriesCount())
	}
	if got, want := string(validated.Canonical()), "{\"schema_version\":1,\"families\":[]}\n"; got != want {
		t.Fatalf("canonical = %q, want %q", got, want)
	}
}

func TestExplicitZeroSeriesPublicationAdvancesGenerationAndClearsState(t *testing.T) {
	holder := NewInitialHolder()
	nonEmpty := mustParse(t, `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`)
	if _, err := holder.Install(nonEmpty); err != nil {
		t.Fatal(err)
	}

	explicitZero := mustParse(t, `{"schema_version":1,"families":[{"name":"validated_but_removed","help":"","type":"gauge","series":[]}]}`)
	active, err := holder.Install(explicitZero)
	if err != nil {
		t.Fatal(err)
	}
	if active.Generation() != 2 || !active.Validated().IsZeroSeries() {
		t.Fatalf("generation=%d zero=%t", active.Generation(), active.Validated().IsZeroSeries())
	}
	if got := string(active.Validated().Canonical()); got != string(zeroCanonical) {
		t.Fatalf("canonical = %q", got)
	}
}

func TestEmptyTransportPayloadPreservesInitialGeneration(t *testing.T) {
	holder := NewInitialHolder()
	if _, err := Parse(nil, DefaultLimits()); rejectionReason(t, err) != ReasonEmptyPayload {
		t.Fatalf("empty payload error = %v", err)
	}
	active := holder.Active()
	if active.Generation() != 0 || string(active.Validated().Canonical()) != string(zeroCanonical) {
		t.Fatal("empty payload changed initial state")
	}
}

func TestZeroSnapshotIsCallerOwned(t *testing.T) {
	first := Zero()
	content := first.Canonical()
	content[0] = 'X'
	if got := string(Zero().Canonical()); got != string(zeroCanonical) {
		t.Fatalf("zero snapshot retained mutation: %q", got)
	}
}
