package finalwait

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDefaultsAndValidation(t *testing.T) {
	configuration := Defaults()
	if configuration.Mode != Scrapes || configuration.Duration != 30*time.Second || configuration.Timeout != time.Minute || configuration.RequiredScrapes != 1 || configuration.CompletionGrace != 500*time.Millisecond {
		t.Fatalf("defaults = %+v", configuration)
	}
	for _, invalid := range []Config{
		{Mode: "auto", Timeout: time.Second, RequiredScrapes: 1},
		{Mode: Scrapes, Timeout: 0, RequiredScrapes: 1},
		{Mode: Scrapes, Timeout: time.Second, RequiredScrapes: 0},
		{Mode: Duration, Duration: time.Hour + 1, Timeout: time.Second, RequiredScrapes: 1},
		{Mode: Immediate, Timeout: time.Second, RequiredScrapes: 1, CompletionGrace: 5*time.Second + 1},
	} {
		if invalid.Validate() == nil {
			t.Errorf("accepted invalid configuration %+v", invalid)
		}
	}
}

func TestImmediateDurationAndTimeoutModes(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		config Config
		reason Reason
		wait   time.Duration
	}{
		{"immediate", Config{Mode: Immediate, Timeout: time.Second, RequiredScrapes: 1}, ReasonImmediate, 0},
		{"duration", Config{Mode: Duration, Duration: 20 * time.Second, Timeout: time.Second, RequiredScrapes: 1}, ReasonDurationElapsed, 20 * time.Second},
		{"scrape timeout", Config{Mode: Scrapes, Timeout: 40 * time.Second, RequiredScrapes: 1}, ReasonTimeout, 40 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			timer := make(chan time.Time, 1)
			waiter, err := newWaiter(test.config, 7, func() time.Time { return now }, func(duration time.Duration) <-chan time.Time {
				if duration != test.wait {
					t.Errorf("wait = %s, want %s", duration, test.wait)
				}
				return timer
			})
			if err != nil {
				t.Fatal(err)
			}
			if test.wait == 0 {
				result, err := waiter.Wait(context.Background())
				if err != nil || result.Reason != test.reason || !result.Deadline.IsZero() {
					t.Fatalf("result=%+v error=%v", result, err)
				}
				return
			}
			result := make(chan Result, 1)
			go func() { got, _ := waiter.Wait(context.Background()); result <- got }()
			waitActive(t, waiter)
			timer <- now.Add(test.wait)
			got := <-result
			if got.Reason != test.reason || !got.Deadline.Equal(now.Add(test.wait)) {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

func TestScrapeThresholdIsGenerationBoundAndSaturating(t *testing.T) {
	configuration := Defaults()
	configuration.RequiredScrapes = 3
	waiter, err := New(configuration, 11)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan Result, 1)
	go func() { got, _ := waiter.Wait(context.Background()); result <- got }()
	waitActive(t, waiter)
	if completion := waiter.Complete(10); completion.Counted || completion.Completed != 0 || !completion.Tracked {
		t.Fatalf("wrong generation counted: %+v", completion)
	}

	var group sync.WaitGroup
	for range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			waiter.Complete(11)
		}()
	}
	group.Wait()
	got := <-result
	if got.Reason != ReasonRequiredScrapes || got.Completed != 3 || waiter.State().Completed != 3 {
		t.Fatalf("result=%+v state=%+v", got, waiter.State())
	}
	if completion := waiter.Complete(11); completion.Counted {
		t.Fatal("completion counted after terminal state")
	}
}

func TestTransitionAndActivationShareCompletionBoundary(t *testing.T) {
	configuration := Defaults()
	waiter, err := New(configuration, 5)
	if err != nil {
		t.Fatal(err)
	}
	transitionEntered := make(chan struct{})
	releaseTransition := make(chan struct{})
	result := make(chan Result, 1)
	go func() {
		got, waitErr := waiter.WaitTransition(context.Background(), func() error {
			close(transitionEntered)
			<-releaseTransition
			return nil
		})
		if waitErr != nil {
			t.Errorf("WaitTransition: %v", waitErr)
		}
		result <- got
	}()
	<-transitionEntered
	completion := make(chan Completion, 1)
	go func() { completion <- waiter.Complete(5) }()
	select {
	case got := <-completion:
		t.Fatalf("completion crossed unfinished transition: %+v", got)
	case <-time.After(10 * time.Millisecond):
	}
	close(releaseTransition)
	if got := <-completion; !got.Counted || !got.Threshold {
		t.Fatalf("completion = %+v", got)
	}
	if got := <-result; got.Reason != ReasonRequiredScrapes {
		t.Fatalf("result = %+v", got)
	}
}

func TestExternalTerminationPrecedesNormalCompletion(t *testing.T) {
	configuration := Defaults()
	waiter, err := New(configuration, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := waiter.Wait(ctx)
	if err != nil || result.Reason != ReasonExternalTermination || result.Completed != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if _, err := waiter.Wait(context.Background()); err != ErrAlreadyStarted {
		t.Fatalf("second Wait error = %v", err)
	}
}

func waitActive(t *testing.T, waiter *Waiter) {
	t.Helper()
	deadline := time.After(time.Second)
	for !waiter.State().Active {
		select {
		case <-deadline:
			t.Fatal("final wait did not become active")
		default:
		}
	}
}
