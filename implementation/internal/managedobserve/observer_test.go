package managedobserve

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/diagnostic"
	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedprotocol"
	"github.com/Denki77/metricshell/implementation/internal/selfmetric"
)

func TestObserverProjectsBoundedManagedOutcomesWithoutApplicationLabels(t *testing.T) {
	metrics, err := selfmetric.New(buildinfo.Info{Version: "test", Revision: "test"}, selfmetric.FinalWaitImmediate, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	registry := managed.NewRegistry()
	owner, _ := managed.NewOwner(registry, 1)
	defer owner.Close()
	observer := New(metrics, diagnostic.New(&logs, time.Now), registry, owner, func() string { return "running" })
	observer.Initialize()

	name := "application_secret_name"
	result := observer.Submit(context.Background(), managed.Mutation{Descriptor: &managed.Descriptor{Name: name, Type: managed.Gauge}})
	if result.Outcome != managed.OutcomeCommitted {
		t.Fatalf("result=%+v", result)
	}
	observer.ProtocolRejected(managedprotocol.CodeMalformedJSON)
	observer.Materialized(false, nil)
	observer.Frozen(true, result.Generation)
	observer.FinalInstalled("accepted", result.Generation)

	view := metrics.View()
	if counter(view, selfmetric.ManagedOperationsTotal, "outcome", "committed") != 1 ||
		counter(view, selfmetric.ManagedProtocolTotal, "code", "malformed_json") != 1 ||
		gauge(view, selfmetric.ManagedGeneration) != 1 || gauge(view, selfmetric.ManagedFamilies) != 1 {
		t.Fatalf("managed projection missing: %+v", view)
	}
	if strings.Contains(logs.String(), name) {
		t.Fatal("application-controlled metric name leaked into diagnostics")
	}
}

func counter(view selfmetric.View, familyName, labelName, labelValue string) uint64 {
	for _, family := range view.Families {
		if family.Name != familyName {
			continue
		}
		for _, sample := range family.Samples {
			if len(sample.Labels) == 1 && sample.Labels[0].Name == labelName && sample.Labels[0].Value == labelValue {
				return sample.Counter
			}
		}
	}
	return 0
}

func gauge(view selfmetric.View, familyName string) float64 {
	for _, family := range view.Families {
		if family.Name == familyName && len(family.Samples) == 1 {
			return family.Samples[0].Gauge
		}
	}
	return -1
}
