package managedserver

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedprotocol"
)

func TestUnixServerEndToEndConcurrentClients(t *testing.T) {
	server, registry, socket, cancel := startServer(t, 64)
	defer cancel()
	defer server.Close()

	descriptor := request(t, socket, `{"version":1,"op":"declare","name":"jobs","help":"Jobs.","type":"counter","label_names":[],"buckets":[]}`)
	if descriptor.Outcome != managed.OutcomeCommitted {
		t.Fatalf("declare = %+v", descriptor)
	}
	var wait sync.WaitGroup
	results := make(chan managedprotocol.Response, 32)
	for index := 0; index < 32; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- request(t, socket, `{"version":1,"op":"counter_add","name":"jobs","labels":{},"value":"1"}`)
		}()
	}
	wait.Wait()
	close(results)
	for response := range results {
		if response.Outcome != managed.OutcomeCommitted {
			t.Fatalf("response = %+v", response)
		}
	}
	if got := onlyValue(t, registry.Read(), "jobs"); got != 32 {
		t.Fatalf("jobs = %v", got)
	}
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o620 {
		t.Fatalf("socket mode = %o", info.Mode().Perm())
	}
}

func TestUnixServerPartialDisconnectAndUnknownOutcome(t *testing.T) {
	server, registry, socket, cancel := startServer(t, 8)
	defer cancel()
	defer server.Close()
	if got := request(t, socket, `{"version":1,"op":"declare","name":"jobs","help":"","type":"counter","label_names":[],"buckets":[]}`); got.Outcome != managed.OutcomeCommitted {
		t.Fatal(got)
	}

	partial, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = partial.Write([]byte(`{"version":1`))
	time.Sleep(150 * time.Millisecond)
	_ = partial.Close()
	if registry.Read().Generation != 1 {
		t.Fatal("partial request reached registry")
	}

	connection, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = connection.Write([]byte(`{"version":1,"op":"counter_add","name":"jobs","labels":{},"value":"1"}` + "\n"))
	_ = connection.Close()
	deadline := time.Now().Add(time.Second)
	for onlyValue(t, registry.Read(), "jobs") != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := onlyValue(t, registry.Read(), "jobs"); got != 1 {
		t.Fatalf("complete request disconnected before ACK = %v", got)
	}
}

func TestUnixServerAdmissionClosureAndCleanup(t *testing.T) {
	server, _, socket, cancel := startServer(t, 4)
	server.CloseAdmission()
	response := request(t, socket, `{"version":1,"op":"gauge_set","name":"depth","labels":{},"value":"1"}`)
	if response.Outcome != managed.OutcomeClosed {
		t.Fatalf("closed response = %+v", response)
	}
	cancel()
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatalf("socket remains after cleanup: %v", err)
	}
}

func TestUnixServerBoundedConnectionDrain(t *testing.T) {
	server, _, socket, cancel := startServer(t, 1)
	defer cancel()
	connection, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte(`{"version":1`)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for len(server.connections) != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(server.connections) != 1 {
		t.Fatal("connection was not admitted before closure")
	}
	server.CloseAdmission()
	ctx, stop := context.WithTimeout(context.Background(), time.Millisecond)
	if server.Drain(ctx) {
		t.Fatal("partial connection drained before its bounded read completed")
	}
	stop()
	_ = connection.Close()
	ctx, stop = context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if !server.Drain(ctx) {
		t.Fatal("closed partial connection did not drain")
	}
}

func TestUnixServerRejectsUnsafePathsAndConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "managed.sock")
	if err := os.WriteFile(path, []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	owner, _ := managed.NewOwner(managed.NewRegistry(), 1)
	defer owner.Close()
	configuration := testConfig(path)
	if _, err := Listen(configuration, owner); err == nil {
		t.Fatal("pre-existing path accepted")
	}
	if content, _ := os.ReadFile(path); string(content) != "owned" {
		t.Fatal("pre-existing object was modified")
	}
	for _, change := range []func(*Config){
		func(c *Config) { c.Path = "relative.sock" },
		func(c *Config) { c.Mode = 0o666 },
		func(c *Config) { c.Connections = 0 },
		func(c *Config) { c.ReadTimeout = 0 },
	} {
		candidate := testConfig(filepath.Join(t.TempDir(), "managed.sock"))
		change(&candidate)
		if _, err := Listen(candidate, owner); err == nil {
			t.Fatalf("invalid config accepted: %+v", candidate)
		}
	}
}

func startServer(t *testing.T, connections int) (*Server, *managed.Registry, string, context.CancelFunc) {
	t.Helper()
	registry := managed.NewRegistry()
	owner, err := managed.NewOwner(registry, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(owner.Close)
	path := filepath.Join(t.TempDir(), "runtime", "managed.sock")
	configuration := testConfig(path)
	configuration.Connections = connections
	server, err := Listen(configuration, owner)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = server.Serve(ctx) }()
	return server, registry, path, cancel
}

func testConfig(path string) Config {
	return Config{Path: path, Mode: 0o620, Connections: 8, FrameBytes: 8 << 10, ReadTimeout: 100 * time.Millisecond, WriteTimeout: time.Second}
}

func request(t *testing.T, socket, payload string) managedprotocol.Response {
	t.Helper()
	connection, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := connection.Write([]byte(payload + "\n")); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(connection).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var response managedprotocol.Response
	if err := json.Unmarshal(line, &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func onlyValue(t *testing.T, snapshot managed.Snapshot, familyName string) float64 {
	t.Helper()
	family := snapshot.Families[familyName]
	if len(family.Series) == 0 {
		return 0
	}
	if len(family.Series) != 1 {
		t.Fatalf("series = %d", len(family.Series))
	}
	for _, series := range family.Series {
		return series.Value
	}
	return 0
}
