package managedclient

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/managed"
	"github.com/Denki77/metricshell/implementation/internal/managedserver"
)

func TestClientAcceptedRejectedReconnectAndLabels(t *testing.T) {
	socket, stop := startManagedServer(t)
	defer stop()
	configuration := Config{SocketPath: socket, Timeout: time.Second}
	requests := []map[string]any{
		{"version": 1, "op": "declare", "name": "jobs", "help": "Jobs.", "type": "counter", "label_names": []string{"worker"}, "buckets": []string{}},
		{"version": 1, "op": "counter_add", "name": "jobs", "labels": map[string]string{"worker": "a=b c"}, "value": "1"},
	}
	for _, request := range requests {
		frame, _ := EncodeRequest(request)
		if result := Send(context.Background(), configuration, frame); result.Category != Accepted {
			t.Fatalf("result = %+v", result)
		}
	}
	frame, _ := EncodeRequest(map[string]any{"version": 1, "op": "counter_add", "name": "missing", "labels": map[string]string{}, "value": "1"})
	if result := Send(context.Background(), configuration, frame); result.Category != Rejected || result.Reason != string(managed.ReasonUndeclared) {
		t.Fatalf("rejection = %+v", result)
	}
}

func TestClientTransportProtocolAndUnknown(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.sock")
	frame := []byte("{}\n")
	if result := Send(context.Background(), Config{SocketPath: missing, Timeout: 100 * time.Millisecond}, frame); result.Category != Transport {
		t.Fatalf("transport = %+v", result)
	}
	for _, test := range []struct {
		name     string
		response string
		want     Category
	}{
		{name: "protocol", response: "not-json\n", want: Protocol},
		{name: "unknown", response: "", want: Unknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			socket, stop := oneShotServer(t, test.response)
			defer stop()
			result := Send(context.Background(), Config{SocketPath: socket, Timeout: time.Second}, frame)
			if result.Category != test.want {
				t.Fatalf("result = %+v, want %s", result, test.want)
			}
		})
	}
}

func TestCLIExitCodesAndLocalValidation(t *testing.T) {
	socket, stop := startManagedServer(t)
	defer stop()
	lookup := func(string) (string, bool) { return "", false }
	var stdout, stderr bytes.Buffer
	code := RunCLI([]string{"--socket=" + socket, "declare", "jobs", "counter", "Jobs.", "worker"}, &stdout, &stderr, lookup)
	if code != ExitAccepted || !strings.Contains(stdout.String(), `"category":"accepted"`) {
		t.Fatalf("declare code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	code = RunCLI([]string{"--socket=" + socket, "counter-add", "jobs", "1", "worker=a=b c"}, &stdout, &stderr, lookup)
	if code != ExitAccepted {
		t.Fatalf("add code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if code := RunCLI([]string{"counter-add", "jobs", "not-a-number"}, &stdout, &stderr, lookup); code != ExitLocal {
		t.Fatalf("local validation code = %d", code)
	}
	if code := RunCLI([]string{"--socket=" + filepath.Join(t.TempDir(), "missing"), "counter-add", "jobs", "1"}, &stdout, &stderr, lookup); code != ExitTransport {
		t.Fatalf("transport code = %d", code)
	}
}

func startManagedServer(t *testing.T) (string, func()) {
	t.Helper()
	registry := managed.NewRegistry()
	owner, err := managed.NewOwner(registry, 16)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "runtime")
	socket := filepath.Join(directory, "managed.sock")
	server, err := managedserver.Listen(managedserver.Config{
		Path: socket, Mode: 0o600, Connections: 8, FrameBytes: 8 << 10,
		ReadTimeout: time.Second, WriteTimeout: time.Second,
	}, owner)
	if err != nil {
		owner.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = server.Serve(ctx) }()
	return socket, func() { cancel(); _ = server.Close(); owner.Close() }
}

func oneShotServer(t *testing.T, response string) (string, func()) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(directory, "server.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = bufio.NewReader(connection).ReadBytes('\n')
		if response != "" {
			_, _ = connection.Write([]byte(response))
		}
	}()
	return socket, func() { _ = listener.Close(); <-done }
}
