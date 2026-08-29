package ingestion

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

const valid = `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`

func TestCoreAcceptedRejectedFrozenAndInternal(t *testing.T) {
	holder := snapshot.NewHolder(snapshot.Zero())
	core := mustCore(t, holder, 1, 0, nil)
	if result := core.Publish(context.Background(), File, []byte(valid)); result.Outcome != Accepted || result.Generation != 1 {
		t.Fatalf("accepted result = %#v", result)
	}
	want := holder.Active()
	if result := core.Publish(context.Background(), HTTP, []byte(`{}`)); result.Outcome != Rejected || result.Reason != snapshot.ReasonMalformed {
		t.Fatalf("rejected result = %#v", result)
	}
	if holder.Active().Generation() != want.Generation() {
		t.Fatal("rejection changed state")
	}
	holder.Freeze()
	if result := core.Publish(context.Background(), Unix, []byte(valid)); result.Outcome != Rejected || result.Reason != snapshot.ReasonFrozen {
		t.Fatalf("frozen result = %#v", result)
	}

	broken, err := NewWithParser(snapshot.NewHolder(snapshot.Zero()), snapshot.DefaultLimits(), 1, 0, nil,
		func([]byte, snapshot.Limits) (snapshot.ValidatedSnapshot, error) {
			return snapshot.ValidatedSnapshot{}, errors.New("boom")
		}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if result := broken.Publish(context.Background(), HTTP, []byte(valid)); result.Outcome != InternalError || result.Reason != snapshot.ReasonInternal {
		t.Fatalf("internal result = %#v", result)
	}
}

func TestCoreAdmissionQueueCancellationAndBoundaries(t *testing.T) {
	entered := make(chan struct{}, 3)
	release := make(chan struct{})
	parser := func(content []byte, limits snapshot.Limits) (snapshot.ValidatedSnapshot, error) {
		entered <- struct{}{}
		<-release
		return snapshot.Parse(content, limits)
	}
	core, err := NewWithParser(snapshot.NewHolder(snapshot.Zero()), snapshot.DefaultLimits(), 1, 1, nil, parser, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan Result, 2)
	go func() { results <- core.Publish(context.Background(), File, []byte(valid)) }()
	<-entered
	queuedContext, cancel := context.WithCancel(context.Background())
	go func() { results <- core.Publish(queuedContext, HTTP, []byte(valid)) }()
	for len(core.admissions) != 2 {
		time.Sleep(time.Millisecond)
	}
	if result := core.Publish(context.Background(), Unix, []byte(valid)); result.Outcome != Busy {
		t.Fatalf("full queue result = %#v", result)
	}
	cancel()
	if result := <-results; result.Outcome != Timeout {
		t.Fatalf("queued cancellation result = %#v", result)
	}
	close(release)
	if result := <-results; result.Outcome != Accepted {
		t.Fatalf("executing result = %#v", result)
	}

	cancelled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if result := core.Publish(cancelled, File, []byte(valid)); result.Outcome != Timeout {
		t.Fatalf("pre-cancel result = %#v", result)
	}
}

func TestCoreChecksCancellationBeforeCandidateHandoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	core, err := NewWithParser(snapshot.NewHolder(snapshot.Zero()), snapshot.DefaultLimits(), 1, 0, nil,
		func(content []byte, limits snapshot.Limits) (snapshot.ValidatedSnapshot, error) {
			validated, err := snapshot.Parse(content, limits)
			cancel()
			return validated, err
		}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if result := core.Publish(ctx, File, []byte(valid)); result.Outcome != Timeout {
		t.Fatalf("result = %#v", result)
	}
	if core.holder.Active().Generation() != 0 {
		t.Fatal("cancelled candidate was installed")
	}
}

func TestMetricsObserverUsesSharedEnumAndStateCore(t *testing.T) {
	now := time.Unix(123, 0)
	registry, err := selfmetric.New(buildinfo.Info{Version: "test", Revision: "test"}, selfmetric.FinalWaitImmediate, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	observer, err := NewMetricsObserver(registry, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	core := mustCore(t, snapshot.NewHolder(snapshot.Zero()), 1, 0, observer)
	if result := core.Publish(context.Background(), HTTP, []byte(valid)); result.Outcome != Accepted {
		t.Fatal(result)
	}
	if result := core.Publish(context.Background(), HTTP, nil); result.Reason != snapshot.ReasonEmptyPayload {
		t.Fatal(result)
	}
	view := registry.View()
	assertGauge(t, view, selfmetric.SnapshotGeneration, nil, 1)
	assertGauge(t, view, selfmetric.IngestionInflight, map[string]string{"transport": "http"}, 0)
	assertGauge(t, view, selfmetric.IngestionLastSuccess, map[string]string{"transport": "http"}, 123)
	assertCounter(t, view, selfmetric.SnapshotPublicationsTotal, map[string]string{"transport": "http", "outcome": "accepted"}, 1)
	assertCounter(t, view, selfmetric.SnapshotRejectionsTotal, map[string]string{"transport": "http", "reason": "empty_payload"}, 1)
}

func TestEnumParityWithNormativeRegistries(t *testing.T) {
	for index, transport := range Transports {
		if string(transport) != selfmetric.Transports[index] {
			t.Fatalf("transport[%d] = %q, selfmetric = %q", index, transport, selfmetric.Transports[index])
		}
	}
	for index, outcome := range Outcomes {
		if string(outcome) != selfmetric.PublicationOutcomes[index] {
			t.Fatalf("outcome[%d] = %q, selfmetric = %q", index, outcome, selfmetric.PublicationOutcomes[index])
		}
	}
	wantSocket := TransportFailures[1:9]
	for index, reason := range wantSocket {
		if string(reason) != selfmetric.SocketFrameReasons[index] {
			t.Fatalf("socket reason[%d] = %q, selfmetric = %q", index, reason, selfmetric.SocketFrameReasons[index])
		}
	}
}

func TestCoreLinearizesConcurrentInstallations(t *testing.T) {
	core := mustCore(t, snapshot.NewHolder(snapshot.Zero()), 4, 28, nil)
	var wait sync.WaitGroup
	results := make(chan Result, 32)
	for index := 0; index < 32; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- core.Publish(context.Background(), Unix, []byte(valid))
		}()
	}
	wait.Wait()
	close(results)
	seen := make(map[uint64]bool)
	for result := range results {
		if result.Outcome != Accepted || seen[result.Generation] {
			t.Fatalf("result = %#v", result)
		}
		seen[result.Generation] = true
	}
	if core.holder.Active().Generation() != 32 {
		t.Fatalf("generation = %d", core.holder.Active().Generation())
	}
}

func mustCore(t *testing.T, holder *snapshot.Holder, concurrent, pending int, observer Observer) *Core {
	t.Helper()
	core, err := New(holder, snapshot.DefaultLimits(), concurrent, pending, observer)
	if err != nil {
		t.Fatal(err)
	}
	return core
}

func assertGauge(t *testing.T, view selfmetric.View, name string, labels map[string]string, want float64) {
	t.Helper()
	for _, family := range view.Families {
		if family.Name != name {
			continue
		}
		for _, sample := range family.Samples {
			if labelsMatch(sample.Labels, labels) && sample.Gauge == want {
				return
			}
		}
	}
	t.Fatalf("gauge %s%v != %v", name, labels, want)
}

func assertCounter(t *testing.T, view selfmetric.View, name string, labels map[string]string, want uint64) {
	t.Helper()
	for _, family := range view.Families {
		if family.Name != name {
			continue
		}
		for _, sample := range family.Samples {
			if labelsMatch(sample.Labels, labels) && sample.Counter == want {
				return
			}
		}
	}
	t.Fatalf("counter %s%v != %d", name, labels, want)
}

func labelsMatch(labels []selfmetric.Label, want map[string]string) bool {
	if len(labels) != len(want) {
		return false
	}
	for _, label := range labels {
		if want[label.Name] != label.Value {
			return false
		}
	}
	return true
}
