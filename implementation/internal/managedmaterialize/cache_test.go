package managedmaterialize

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

func TestManagedAndDirectSnapshotNumericEquivalence(t *testing.T) {
	registry := managed.NewRegistry()
	for _, descriptor := range []managed.Descriptor{
		{Name: "counter", Type: managed.Counter},
		{Name: "gauge", Type: managed.Gauge},
		{Name: "distribution", Type: managed.Histogram, Buckets: []float64{-10, -1, 0, 1, 10, math.Inf(1)}},
	} {
		if _, err := registry.Declare(descriptor); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := registry.Apply(managed.Operation{Kind: managed.CounterAdd, Name: "counter", Value: math.Inf(1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Apply(managed.Operation{Kind: managed.GaugeSet, Name: "gauge", Value: math.Copysign(0, -1)}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{-20, -5, -1, 0, 0.5, 5, 20} {
		if _, err := registry.Apply(managed.Operation{Kind: managed.HistogramObserve, Name: "distribution", Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	managedBody, err := Encode(registry.Read())
	if err != nil {
		t.Fatal(err)
	}
	managedSnapshot, err := snapshot.Parse(managedBody, snapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	directSnapshot, err := snapshot.Parse([]byte(`{"schema_version":1,"families":[`+
		`{"name":"counter","help":"","type":"counter","series":[{"labels":{},"value":"+Inf"}]},`+
		`{"name":"gauge","help":"","type":"gauge","series":[{"labels":{},"value":"-0"}]},`+
		`{"name":"distribution","help":"","type":"histogram","series":[{"labels":{},"histogram":{"count":"7","sum":"-0.5","buckets":[`+
		`{"le":"-10","count":"1"},{"le":"-1","count":"3"},{"le":"0","count":"4"},{"le":"1","count":"5"},{"le":"10","count":"6"},{"le":"+Inf","count":"7"}]}}]}`+
		`]}`), snapshot.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(managedSnapshot.Canonical(), directSnapshot.Canonical()) {
		t.Fatalf("managed and direct differ:\nmanaged %s\ndirect  %s", managedSnapshot.Canonical(), directSnapshot.Canonical())
	}
}

func TestCacheHitStaleRebuildAndReaderOwnership(t *testing.T) {
	registry := managed.NewRegistry()
	declareGauge(t, registry, "value")
	cache, err := New(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := cache.Materialize()
	if err != nil || first.Hit {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := cache.Materialize()
	if err != nil || !second.Hit || second.Representation.Generation != first.Representation.Generation {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	issued := first.Representation.Bytes()
	issued[0] = 'x'
	if second.Representation.Bytes()[0] == 'x' {
		t.Fatal("reader mutated cached representation")
	}
	if _, err := registry.Apply(managed.Operation{Kind: managed.GaugeSet, Name: "value", Labels: map[string]string{}, Value: 2}); err != nil {
		t.Fatal(err)
	}
	third, err := cache.Materialize()
	if err != nil || third.Hit || third.Representation.Generation == first.Representation.Generation {
		t.Fatalf("third=%+v err=%v", third, err)
	}
	if _, err := snapshot.Parse(third.Representation.Bytes(), snapshot.DefaultLimits()); err != nil {
		t.Fatalf("materialized body is not a valid Core candidate: %v", err)
	}
}

func TestConcurrentMissesCoalesce(t *testing.T) {
	registry := managed.NewRegistry()
	declareGauge(t, registry, "value")
	var encodes atomic.Int64
	cache, err := New(registry, func(input managed.Snapshot) ([]byte, error) {
		encodes.Add(1)
		return Encode(input)
	})
	if err != nil {
		t.Fatal(err)
	}
	const readers = 32
	var wait sync.WaitGroup
	wait.Add(readers)
	for index := 0; index < readers; index++ {
		go func() {
			defer wait.Done()
			result, err := cache.Materialize()
			if err != nil || result.Representation.Generation != 1 {
				t.Errorf("result=%+v err=%v", result, err)
			}
		}()
	}
	wait.Wait()
	if encodes.Load() != 1 {
		t.Fatalf("encodes=%d, want 1", encodes.Load())
	}
}

func TestRebuildFailurePreservesPreviousEntry(t *testing.T) {
	registry := managed.NewRegistry()
	declareGauge(t, registry, "value")
	cache, err := New(registry, func(input managed.Snapshot) ([]byte, error) {
		if input.Generation > 1 {
			return nil, errors.New("injected encoding failure")
		}
		return Encode(input)
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := cache.Materialize()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Apply(managed.Operation{Kind: managed.GaugeSet, Name: "value", Labels: map[string]string{}, Value: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Materialize(); err == nil {
		t.Fatal("injected failure was accepted")
	}
	cached, ok := cache.Cached()
	if !ok || cached.Generation != first.Representation.Generation || string(cached.Bytes()) != string(first.Representation.Bytes()) {
		t.Fatalf("cached=%+v ok=%v", cached, ok)
	}
}

func TestConcurrentMutationNeverProducesMixedGeneration(t *testing.T) {
	registry := managed.NewRegistry()
	declareGauge(t, registry, "left")
	declareGauge(t, registry, "right")
	cache, err := New(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for value := 1; value <= 200; value++ {
			_, applyErr := registry.ApplyBatch([]managed.Operation{
				{Kind: managed.GaugeSet, Name: "left", Labels: map[string]string{}, Value: float64(value)},
				{Kind: managed.GaugeSet, Name: "right", Labels: map[string]string{}, Value: float64(value)},
			})
			if applyErr != nil {
				t.Error(applyErr)
				return
			}
		}
	}()
	for {
		result, err := cache.Materialize()
		if err != nil {
			t.Fatal(err)
		}
		values := decodeValues(t, result.Representation.Bytes())
		if values["left"] != values["right"] {
			t.Fatalf("mixed generation %d: %+v", result.Representation.Generation, values)
		}
		select {
		case <-done:
			return
		default:
		}
	}
}

func declareGauge(t *testing.T, registry *managed.Registry, name string) {
	t.Helper()
	if _, err := registry.Declare(managed.Descriptor{Name: name, Type: managed.Gauge}); err != nil {
		t.Fatal(err)
	}
}

func decodeValues(t *testing.T, body []byte) map[string]string {
	t.Helper()
	var decoded struct {
		Families []struct {
			Name   string `json:"name"`
			Series []struct {
				Value string `json:"value"`
			} `json:"series"`
		} `json:"families"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	result := map[string]string{"left": "0", "right": "0"}
	for _, family := range decoded.Families {
		if len(family.Series) != 0 {
			if _, err := strconv.ParseFloat(family.Series[0].Value, 64); err != nil {
				t.Fatal(err)
			}
			result[family.Name] = family.Series[0].Value
		}
	}
	return result
}
