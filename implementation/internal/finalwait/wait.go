package finalwait

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Mode string

const (
	Immediate Mode = "immediate"
	Duration  Mode = "duration"
	Scrapes   Mode = "scrapes"
)

type Reason string

const (
	ReasonImmediate           Reason = "immediate"
	ReasonDurationElapsed     Reason = "duration_elapsed"
	ReasonRequiredScrapes     Reason = "required_scrapes"
	ReasonTimeout             Reason = "timeout"
	ReasonExternalTermination Reason = "external_termination"
	ReasonRuntimeFailure      Reason = "runtime_failure"
)

var ErrConfiguration = errors.New("invalid final-wait configuration")
var ErrAlreadyStarted = errors.New("final wait already started")

type Config struct {
	Mode            Mode
	Duration        time.Duration
	Timeout         time.Duration
	RequiredScrapes int
	CompletionGrace time.Duration
}

func Defaults() Config {
	return Config{
		Mode: Scrapes, Duration: 30 * time.Second, Timeout: 60 * time.Second,
		RequiredScrapes: 1, CompletionGrace: 500 * time.Millisecond,
	}
}

func (configuration Config) Validate() error {
	if configuration.Mode != Immediate && configuration.Mode != Duration && configuration.Mode != Scrapes ||
		configuration.Duration < 0 || configuration.Duration > time.Hour ||
		configuration.Timeout < time.Second || configuration.Timeout > time.Hour ||
		configuration.RequiredScrapes < 1 || configuration.RequiredScrapes > 16 ||
		configuration.CompletionGrace < 0 || configuration.CompletionGrace > 5*time.Second {
		return ErrConfiguration
	}
	return nil
}

type Result struct {
	Reason     Reason
	Generation uint64
	Completed  int
	Deadline   time.Time
}

type State struct {
	Active     bool
	Generation uint64
	Completed  int
	Deadline   time.Time
	Reason     Reason
}

type Waiter struct {
	configuration Config
	generation    uint64
	now           func() time.Time
	after         func(time.Duration) <-chan time.Time

	mu        sync.Mutex
	started   bool
	active    bool
	completed int
	deadline  time.Time
	reason    Reason
	threshold chan struct{}
	reached   sync.Once
}

func New(configuration Config, generation uint64) (*Waiter, error) {
	return newWaiter(configuration, generation, time.Now, time.After)
}

func newWaiter(configuration Config, generation uint64, now func() time.Time, after func(time.Duration) <-chan time.Time) (*Waiter, error) {
	if configuration.Validate() != nil || now == nil || after == nil {
		return nil, ErrConfiguration
	}
	return &Waiter{configuration: configuration, generation: generation, now: now, after: after, threshold: make(chan struct{})}, nil
}

// Wait runs the configured policy exactly once. Cancellation represents an
// external termination request and takes precedence when already observable.
func (waiter *Waiter) Wait(ctx context.Context) (Result, error) {
	waiter.mu.Lock()
	if waiter.started {
		waiter.mu.Unlock()
		return Result{}, ErrAlreadyStarted
	}
	waiter.started, waiter.active = true, true
	wait := waiter.configuration.Duration
	if waiter.configuration.Mode == Scrapes {
		wait = waiter.configuration.Timeout
	}
	if waiter.configuration.Mode != Immediate {
		waiter.deadline = waiter.now().Add(wait)
	}
	waiter.mu.Unlock()

	reason := ReasonImmediate
	if ctx.Err() != nil {
		reason = ReasonExternalTermination
	} else {
		switch waiter.configuration.Mode {
		case Immediate:
		case Duration:
			select {
			case <-ctx.Done():
				reason = ReasonExternalTermination
			case <-waiter.after(wait):
				if ctx.Err() != nil {
					reason = ReasonExternalTermination
				} else {
					reason = ReasonDurationElapsed
				}
			}
		case Scrapes:
			select {
			case <-ctx.Done():
				reason = ReasonExternalTermination
			case <-waiter.threshold:
				if ctx.Err() != nil {
					reason = ReasonExternalTermination
				} else {
					reason = ReasonRequiredScrapes
				}
			case <-waiter.after(wait):
				if ctx.Err() != nil {
					reason = ReasonExternalTermination
				} else {
					select {
					case <-waiter.threshold:
						reason = ReasonRequiredScrapes
					default:
						reason = ReasonTimeout
					}
				}
			}
		}
	}

	waiter.mu.Lock()
	waiter.active, waiter.reason = false, reason
	result := Result{Reason: reason, Generation: waiter.generation, Completed: waiter.completed, Deadline: waiter.deadline}
	waiter.mu.Unlock()
	return result, nil
}

// Complete records one eligible complete response for the frozen generation.
// HTTP response eligibility and write completion are established by exposition.
func (waiter *Waiter) Complete(generation uint64) (completed int, counted bool) {
	waiter.mu.Lock()
	if !waiter.active || waiter.configuration.Mode != Scrapes || generation != waiter.generation || waiter.completed >= waiter.configuration.RequiredScrapes {
		completed = waiter.completed
		waiter.mu.Unlock()
		return completed, false
	}
	waiter.completed++
	completed = waiter.completed
	if completed == waiter.configuration.RequiredScrapes {
		waiter.reached.Do(func() { close(waiter.threshold) })
	}
	waiter.mu.Unlock()
	return completed, true
}

func (waiter *Waiter) State() State {
	waiter.mu.Lock()
	defer waiter.mu.Unlock()
	return State{
		Active: waiter.active, Generation: waiter.generation, Completed: waiter.completed,
		Deadline: waiter.deadline, Reason: waiter.reason,
	}
}
