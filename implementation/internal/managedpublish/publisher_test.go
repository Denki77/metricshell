package managedpublish

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/managedbridge"
)

type installerStub struct {
	mu      sync.Mutex
	calls   int
	failFor int
	entered chan struct{}
	release chan struct{}
}

func (stub *installerStub) Install(ctx context.Context) (managedbridge.Result, error) {
	stub.mu.Lock()
	stub.calls++
	call := stub.calls
	stub.mu.Unlock()
	if stub.entered != nil {
		select {
		case stub.entered <- struct{}{}:
		default:
		}
	}
	if stub.release != nil {
		select {
		case <-stub.release:
		case <-ctx.Done():
			return managedbridge.Result{}, ctx.Err()
		}
	}
	if call <= stub.failFor {
		return managedbridge.Result{}, errors.New("injected")
	}
	return managedbridge.Result{Core: ingestion.Result{Outcome: ingestion.Accepted}, Installed: true}, nil
}

func (stub *installerStub) count() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.calls
}

func TestPublisherRetriesAfterNonTerminalFailureWithoutQueueing(t *testing.T) {
	stub := &installerStub{failFor: 1}
	publisher, _ := New(stub, time.Millisecond, nil)
	publisher.Start(context.Background())
	deadline := time.Now().Add(time.Second)
	for stub.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !publisher.Stop(ctx) || stub.count() < 2 {
		t.Fatalf("publication did not recover: calls=%d", stub.count())
	}
}

func TestStopCancelsAndJoinsInFlightPublication(t *testing.T) {
	stub := &installerStub{entered: make(chan struct{}, 1), release: make(chan struct{})}
	publisher, _ := New(stub, time.Millisecond, nil)
	publisher.Start(context.Background())
	select {
	case <-stub.entered:
	case <-time.After(time.Second):
		t.Fatal("publication did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !publisher.Stop(ctx) {
		t.Fatal("in-flight publication did not stop")
	}
	calls := stub.count()
	time.Sleep(5 * time.Millisecond)
	if stub.count() != calls {
		t.Fatal("publication occurred after stop")
	}
}
