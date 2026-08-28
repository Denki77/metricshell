package snapshot

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestHolderReplacesCompleteStateAndDeletesOmissions(t *testing.T) {
	initial := mustParse(t, `{"schema_version":1,"families":[{"name":"old","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`)
	holder := NewHolder(initial)
	if active := holder.Active(); active.Generation() != 0 || active.Validated().SeriesCount() != 1 {
		t.Fatalf("initial generation=%d series=%d", active.Generation(), active.Validated().SeriesCount())
	}

	next := mustParse(t, `{"schema_version":1,"families":[{"name":"new","help":"","type":"counter","series":[{"labels":{},"value":"2"}]}]}`)
	installed, err := holder.Install(next)
	if err != nil {
		t.Fatal(err)
	}
	if installed.Generation() != 1 || installed.Validated().Families()[0].Name() != "new" {
		t.Fatalf("installed generation=%d families=%v", installed.Generation(), installed.Validated().Families())
	}
	if got := string(holder.Active().Validated().Canonical()); strings.Contains(got, `"old"`) {
		t.Fatalf("omitted family retained: %s", got)
	}
}

func TestHolderPreservesStateOnTypeConflictAndFreeze(t *testing.T) {
	holder := NewHolder(mustParse(t, `{"schema_version":1,"families":[]}`))
	gauge := mustParse(t, `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`)
	if _, err := holder.Install(gauge); err != nil {
		t.Fatal(err)
	}
	want := holder.Active()

	counter := mustParse(t, `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"counter","series":[{"labels":{},"value":"1"}]}]}`)
	if active, err := holder.Install(counter); rejectionReason(t, err) != ReasonTypeConflict || active.Generation() != want.Generation() {
		t.Fatalf("type conflict generation=%d err=%v", active.Generation(), err)
	}
	if got := holder.Active(); got.Generation() != want.Generation() || string(got.Validated().Canonical()) != string(want.Validated().Canonical()) {
		t.Fatal("type conflict changed last-valid state")
	}

	frozen := holder.Freeze()
	if !holder.Frozen() || frozen.Generation() != want.Generation() {
		t.Fatalf("frozen=%t generation=%d", holder.Frozen(), frozen.Generation())
	}
	if _, err := holder.Install(mustParse(t, `{"schema_version":1,"families":[]}`)); rejectionReason(t, err) != ReasonFrozen {
		t.Fatalf("frozen install error=%v", err)
	}
	if got := holder.Active(); got.Generation() != want.Generation() || string(got.Validated().Canonical()) != string(want.Validated().Canonical()) {
		t.Fatal("frozen rejection changed last-valid state")
	}
}

func TestEmptyFamilyDoesNotCreateLifetimeTypeBinding(t *testing.T) {
	emptyGauge := mustParse(t, `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[]}]}`)
	holder := NewHolder(emptyGauge)
	counter := mustParse(t, `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"counter","series":[{"labels":{},"value":"1"}]}]}`)
	if _, err := holder.Install(counter); err != nil {
		t.Fatalf("empty family created binding: %v", err)
	}
}

func TestHolderGenerationOverflowPreservesState(t *testing.T) {
	holder := NewHolder(mustParse(t, `{"schema_version":1,"families":[]}`))
	current := holder.active.Load()
	exhausted := NewActive(current.validated, ^uint64(0))
	holder.active.Store(&exhausted)
	if _, err := holder.Install(mustParse(t, `{"schema_version":1,"families":[]}`)); rejectionReason(t, err) != ReasonInternal {
		t.Fatalf("overflow error=%v", err)
	}
	if holder.Active().Generation() != ^uint64(0) {
		t.Fatal("overflow changed generation")
	}
}

func TestHolderConcurrentReadersObserveOnlyCompleteGenerations(t *testing.T) {
	holder := NewHolder(mustParse(t, `{"schema_version":1,"families":[]}`))
	const writes = 100
	want := make(map[string]struct{}, writes+1)
	want[snapshotKey(holder.Active())] = struct{}{}
	for value := 1; value <= writes; value++ {
		document := fmt.Sprintf(`{"schema_version":1,"families":[{"name":"value","help":"","type":"gauge","series":[{"labels":{},"value":"%d"}]}]}`, value)
		want[fmt.Sprintf("%d:%s", value, mustParse(t, document).Canonical())] = struct{}{}
	}

	var wait sync.WaitGroup
	start := make(chan struct{})
	for reader := 0; reader < 32; reader++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for iteration := 0; iteration < writes*10; iteration++ {
				if key := snapshotKey(holder.Active()); !hasKey(want, key) {
					t.Errorf("observed partial state %q", key)
					return
				}
			}
		}()
	}
	close(start)
	for value := 1; value <= writes; value++ {
		document := fmt.Sprintf(`{"schema_version":1,"families":[{"name":"value","help":"","type":"gauge","series":[{"labels":{},"value":"%d"}]}]}`, value)
		if active, err := holder.Install(mustParse(t, document)); err != nil || active.Generation() != uint64(value) {
			t.Fatalf("install %d: generation=%d err=%v", value, active.Generation(), err)
		}
	}
	wait.Wait()
}

func mustParse(t *testing.T, document string) ValidatedSnapshot {
	t.Helper()
	parsed, err := Parse([]byte(document), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func snapshotKey(active ActiveSnapshot) string {
	return fmt.Sprintf("%d:%s", active.Generation(), active.Validated().Canonical())
}

func hasKey(values map[string]struct{}, key string) bool {
	_, exists := values[key]
	return exists
}
