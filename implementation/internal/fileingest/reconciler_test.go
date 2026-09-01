package fileingest

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

const validSnapshot = `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`

type observation struct {
	mu      sync.Mutex
	results []Result
	watches []string
}

func (observer *observation) Reconciled(result Result, _ time.Duration) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.results = append(observer.results, result)
}
func (observer *observation) WatchEvent(event string) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.watches = append(observer.watches, event)
}

func TestReconcileStartupAbsentPresentAndUnchanged(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "snapshot.json")
	holder := snapshot.NewHolder(snapshot.Zero())
	reconciler := newReconciler(t, path, holder, nil, 2<<20)
	if result := reconciler.Reconcile(context.Background(), Startup); result.Outcome != Absent {
		t.Fatalf("absent = %#v", result)
	}
	writeFile(t, path, validSnapshot)
	if result := reconciler.Reconcile(context.Background(), Startup); result.Outcome != Accepted || result.Publication.Generation != 1 {
		t.Fatalf("present = %#v", result)
	}
	if result := reconciler.Reconcile(context.Background(), Periodic); result.Outcome != Unchanged {
		t.Fatalf("unchanged = %#v", result)
	}
}

func TestReconcileRejectsPartialSymlinkNonRegularAndAmplifiedInput(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "snapshot.json")
	holder := snapshot.NewHolder(snapshot.Zero())
	reconciler := newReconciler(t, path, holder, nil, 128)
	writeFile(t, path, validSnapshot)
	if result := reconciler.Reconcile(context.Background(), Event); result.Outcome != Accepted {
		t.Fatal(result)
	}
	wantGeneration := holder.Active().Generation()

	writeFile(t, path, `{"schema_version":1`)
	if result := reconciler.Reconcile(context.Background(), Event); result.Outcome != Invalid {
		t.Fatalf("partial = %#v", result)
	}
	os.Remove(path)
	if err := os.Symlink(filepath.Join(directory, "source"), path); err != nil {
		t.Fatal(err)
	}
	if result := reconciler.Reconcile(context.Background(), Event); result.Outcome != Invalid {
		t.Fatalf("symlink = %#v", result)
	}
	os.Remove(path)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if result := reconciler.Reconcile(context.Background(), Event); result.Outcome != Invalid {
		t.Fatalf("directory = %#v", result)
	}
	os.Remove(path)
	writeFile(t, path, string(make([]byte, 129)))
	if result := reconciler.Reconcile(context.Background(), Event); result.Outcome != Invalid {
		t.Fatalf("oversized = %#v", result)
	}
	if holder.Active().Generation() != wantGeneration {
		t.Fatal("invalid file changed last-valid state")
	}
}

func TestRunObservesAtomicRenameAndPeriodicRecovery(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "snapshot.json")
	holder := snapshot.NewHolder(snapshot.Zero())
	observer := &observation{}
	reconciler := newReconciler(t, path, holder, observer, 2<<20)
	reconciler.configuration.ReconcileInterval = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- reconciler.Run(ctx) }()

	temporary := filepath.Join(directory, ".snapshot.tmp")
	writeFile(t, temporary, validSnapshot)
	if err := os.Rename(temporary, path); err != nil {
		t.Fatal(err)
	}
	waitGeneration(t, holder, 1)

	// A direct write intentionally produces invalid content and must retain state.
	writeFile(t, path, `{"schema_version":`)
	time.Sleep(150 * time.Millisecond)
	if holder.Active().Generation() != 1 {
		t.Fatal("partial in-place write changed state")
	}
	// Restore without relying on a notification: periodic reconciliation is the recovery authority.
	writeFile(t, path, validSnapshot+" ")
	waitGeneration(t, holder, 2)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestEventName(t *testing.T) {
	if got := eventName([]byte{'a', 'b', 0, 0}); got != "ab" {
		t.Fatalf("name = %q", got)
	}
}

func newReconciler(t *testing.T, path string, holder *snapshot.Holder, observer Observer, decoded int) *Reconciler {
	t.Helper()
	core, err := ingestion.New(holder, snapshot.DefaultLimits(), 2, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	reconciler, err := New(Config{Path: path, ReconcileInterval: time.Second, DecodedBytes: decoded}, core, observer)
	if err != nil {
		t.Fatal(err)
	}
	return reconciler
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitGeneration(t *testing.T, holder *snapshot.Holder, generation uint64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for holder.Active().Generation() != generation {
		if time.Now().After(deadline) {
			t.Fatalf("generation = %d, want %d", holder.Active().Generation(), generation)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
