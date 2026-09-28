package managedpublish

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/managedbridge"
)

type Installer interface {
	Install(context.Context) (managedbridge.Result, error)
}

type Observer interface {
	Materialized(bool, error)
}

type noopObserver struct{}

func (noopObserver) Materialized(bool, error) {}

// Publisher coalesces committed registry generations behind one bounded ticker.
// It owns exactly one goroutine and never queues publication work.
type Publisher struct {
	installer Installer
	interval  time.Duration
	observer  Observer
	cancel    context.CancelFunc
	done      chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

func New(installer Installer, interval time.Duration, observer Observer) (*Publisher, error) {
	if installer == nil || interval <= 0 {
		return nil, errors.New("invalid managed publication configuration")
	}
	if observer == nil {
		observer = noopObserver{}
	}
	return &Publisher{installer: installer, interval: interval, observer: observer, done: make(chan struct{})}, nil
}

func (publisher *Publisher) Start(parent context.Context) {
	publisher.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(parent)
		publisher.cancel = cancel
		go publisher.run(ctx)
	})
}

func (publisher *Publisher) Stop(ctx context.Context) bool {
	publisher.stopOnce.Do(func() {
		if publisher.cancel != nil {
			publisher.cancel()
		}
	})
	select {
	case <-publisher.done:
		return true
	case <-ctx.Done():
		return false
	}
}

func (publisher *Publisher) run(ctx context.Context) {
	defer close(publisher.done)
	ticker := time.NewTicker(publisher.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := publisher.installer.Install(ctx)
			publisher.observer.Materialized(result.CacheHit, err)
		}
	}
}
