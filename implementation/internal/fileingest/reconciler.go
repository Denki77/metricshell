package fileingest

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
)

type Trigger string

const (
	Startup        Trigger = "startup"
	Event          Trigger = "event"
	Periodic       Trigger = "periodic"
	Overflow       Trigger = "overflow"
	WatchReinstall Trigger = "watch_reinstall"
)

type Outcome string

const (
	Accepted  Outcome = "accepted"
	Unchanged Outcome = "unchanged"
	Absent    Outcome = "absent"
	Invalid   Outcome = "invalid"
	Error     Outcome = "error"
)

type Result struct {
	Trigger     Trigger
	Outcome     Outcome
	Publication ingestion.Result
}

type Observer interface {
	Reconciled(Result, time.Duration)
	WatchEvent(string)
}

type noopObserver struct{}

func (noopObserver) Reconciled(Result, time.Duration) {}
func (noopObserver) WatchEvent(string)                {}

type Config struct {
	Path              string
	ReconcileInterval time.Duration
	DecodedBytes      int
}

type Reconciler struct {
	configuration Config
	publisher     ingestion.Publisher
	observer      Observer
	now           func() time.Time

	mu       sync.Mutex
	lastHash [sha256.Size]byte
	hasHash  bool
}

func New(configuration Config, publisher ingestion.Publisher, observer Observer) (*Reconciler, error) {
	if !filepath.IsAbs(configuration.Path) || configuration.ReconcileInterval < 100*time.Millisecond || configuration.ReconcileInterval > time.Second || configuration.DecodedBytes < 1 || publisher == nil {
		return nil, errors.New("invalid file ingestion configuration")
	}
	if observer == nil {
		observer = noopObserver{}
	}
	return &Reconciler{configuration: configuration, publisher: publisher, observer: observer, now: time.Now}, nil
}

func (reconciler *Reconciler) Reconcile(ctx context.Context, trigger Trigger) Result {
	reconciler.mu.Lock()
	defer reconciler.mu.Unlock()
	started := reconciler.now()
	result := Result{Trigger: trigger}
	defer func() { reconciler.observer.Reconciled(result, reconciler.now().Sub(started)) }()

	content, outcome := readRegular(reconciler.configuration.Path, reconciler.configuration.DecodedBytes)
	if outcome != Accepted {
		result.Outcome = outcome
		return result
	}
	digest := sha256.Sum256(content)
	if reconciler.hasHash && digest == reconciler.lastHash {
		result.Outcome = Unchanged
		return result
	}
	result.Publication = reconciler.publisher.Publish(ctx, ingestion.File, content)
	switch result.Publication.Outcome {
	case ingestion.Accepted:
		reconciler.lastHash, reconciler.hasHash = digest, true
		result.Outcome = Accepted
	case ingestion.Rejected:
		result.Outcome = Invalid
	default:
		result.Outcome = Error
	}
	return result
}

func readRegular(path string, limit int) ([]byte, Outcome) {
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, syscall.ENOENT) {
		return nil, Absent
	}
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return nil, Invalid
		}
		return nil, Error
	}
	file := os.NewFile(uintptr(descriptor), "snapshot")
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, Error
	}
	if !info.Mode().IsRegular() {
		return nil, Invalid
	}
	content, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, Error
	}
	if len(content) > limit {
		return nil, Invalid
	}
	return content, Accepted
}
