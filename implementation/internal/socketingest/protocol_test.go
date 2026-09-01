package socketingest

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

const document = `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`

type protocolObserver struct {
	mu       sync.Mutex
	failures []ingestion.TransportFailure
	expired  int
}

func (observer *protocolObserver) FrameRejected(failure ingestion.TransportFailure) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.failures = append(observer.failures, failure)
}
func (observer *protocolObserver) TransactionExpired() {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.expired++
}
func (observer *protocolObserver) ConnectionChanged(int) {}

func TestGoldenOneAndMultipartTranscripts(t *testing.T) {
	handler, holder := newProtocol(t, nil)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(document))
	responses := exchange(t, handler,
		fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN one 1 %d\n", len(document)),
		fmt.Sprintf("MSP/1 SNAPSHOT_PART one 0 %s\n", encoded),
		"MSP/1 SNAPSHOT_COMMIT one\n",
	)
	want := []string{"MSP/1 FRAME_ACCEPTED one BEGIN\n", "MSP/1 FRAME_ACCEPTED one 0\n", "MSP/1 ACK one 1\n"}
	assertTranscript(t, responses, want)

	cut := len(document) / 2
	responses = exchange(t, handler,
		fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN two 2 %d\n", len(document)),
		fmt.Sprintf("MSP/1 SNAPSHOT_PART two 0 %s\n", base64.RawURLEncoding.EncodeToString([]byte(document[:cut]))),
		fmt.Sprintf("MSP/1 SNAPSHOT_PART two 1 %s\n", base64.RawURLEncoding.EncodeToString([]byte(document[cut:]))),
		"MSP/1 SNAPSHOT_COMMIT two\n",
	)
	assertTranscript(t, responses, []string{"MSP/1 FRAME_ACCEPTED two BEGIN\n", "MSP/1 FRAME_ACCEPTED two 0\n", "MSP/1 FRAME_ACCEPTED two 1\n", "MSP/1 ACK two 2\n"})
	if holder.Active().Generation() != 2 {
		t.Fatalf("generation = %d", holder.Active().Generation())
	}
}

func TestProtocolRejectsEncodingOrderDuplicatesMissingAndSize(t *testing.T) {
	tests := []struct {
		name   string
		frames []string
		last   string
	}{
		{"padded", []string{"MSP/1 SNAPSHOT_BEGIN x 1 1\n", "MSP/1 SNAPSHOT_PART x 0 YQ==\n"}, "MSP/1 NACK x malformed\n"},
		{"alphabet", []string{"MSP/1 SNAPSHOT_BEGIN x 1 1\n", "MSP/1 SNAPSHOT_PART x 0 +w\n"}, "MSP/1 NACK x transaction_invalid\n"},
		{"out of order", []string{"MSP/1 SNAPSHOT_BEGIN x 2 2\n", "MSP/1 SNAPSHOT_PART x 1 YQ\n"}, "MSP/1 NACK x missing_part\n"},
		{"duplicate", []string{"MSP/1 SNAPSHOT_BEGIN x 2 2\n", "MSP/1 SNAPSHOT_PART x 0 YQ\n", "MSP/1 SNAPSHOT_PART x 0 Yg\n"}, "MSP/1 NACK x duplicate_part\n"},
		{"missing", []string{"MSP/1 SNAPSHOT_BEGIN x 2 2\n", "MSP/1 SNAPSHOT_PART x 0 YQ\n", "MSP/1 SNAPSHOT_COMMIT x\n"}, "MSP/1 NACK x missing_part\n"},
		{"declared mismatch", []string{"MSP/1 SNAPSHOT_BEGIN x 1 2\n", "MSP/1 SNAPSHOT_PART x 0 YQ\n", "MSP/1 SNAPSHOT_COMMIT x\n"}, "MSP/1 NACK x transaction_invalid\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, holder := newProtocol(t, nil)
			responses := exchange(t, handler, test.frames...)
			if got := responses[len(responses)-1]; got != test.last {
				t.Fatalf("response = %q, want %q", got, test.last)
			}
			if holder.Active().Generation() != 0 {
				t.Fatal("invalid transaction changed state")
			}
		})
	}
}

func TestProtocolCandidateNACKAndTransactionExpiry(t *testing.T) {
	observer := &protocolObserver{}
	handler, holder := newProtocol(t, observer)
	clock := time.Unix(10, 0)
	reads := 0
	handler.now = func() time.Time {
		reads++
		if reads > 1 {
			return clock.Add(6 * time.Second)
		}
		return clock
	}
	responses := exchange(t, handler, "MSP/1 SNAPSHOT_BEGIN old 1 1\n", "MSP/1 SNAPSHOT_PART old 0 YQ\n")
	if responses[1] != "MSP/1 NACK old transaction_expired\n" || observer.expired != 1 {
		t.Fatalf("responses=%q expired=%d", responses, observer.expired)
	}

	handler.now = time.Now
	invalid := []byte(`{}`)
	responses = exchange(t, handler,
		fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN bad 1 %d\n", len(invalid)),
		fmt.Sprintf("MSP/1 SNAPSHOT_PART bad 0 %s\n", base64.RawURLEncoding.EncodeToString(invalid)),
		"MSP/1 SNAPSHOT_COMMIT bad\n",
	)
	if responses[2] != "MSP/1 NACK bad malformed\n" || holder.Active().Generation() != 0 {
		t.Fatalf("responses=%q generation=%d", responses, holder.Active().Generation())
	}
}

func TestFrameBoundAndCapacityFormula(t *testing.T) {
	configuration := DefaultConfig()
	if got := EffectiveDecodedCapacity(configuration.FrameBytes, configuration.Parts); got < configuration.SnapshotBytes {
		t.Fatalf("default capacity = %d", got)
	}
	configuration.SnapshotBytes = EffectiveDecodedCapacity(configuration.FrameBytes, configuration.Parts)
	if err := configuration.Validate(); err != nil {
		t.Fatalf("exact capacity rejected: %v", err)
	}
	configuration.SnapshotBytes++
	if err := configuration.Validate(); err == nil {
		t.Fatal("capacity below snapshot limit accepted")
	}

	handler, _ := newProtocol(t, nil)
	oversized := strings.Repeat("x", handler.configuration.FrameBytes) + "\n"
	responses := exchange(t, handler, oversized, "MSP/1 UNKNOWN invalid\n")
	if responses[0] != "MSP/1 NACK invalid frame_limit\n" || responses[1] != "MSP/1 NACK invalid malformed\n" {
		t.Fatalf("responses = %q", responses)
	}
}

func TestDefaultConfigurationTransfersOneMiBDecodedCandidate(t *testing.T) {
	handler, holder := newProtocol(t, nil)
	zero := `{"schema_version":1,"families":[]}`
	content := []byte(strings.Repeat(" ", (1<<20)-len(zero)) + zero)
	const partBytes = 6_000
	partCount := (len(content) + partBytes - 1) / partBytes
	frames := []string{fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN capacity %d %d\n", partCount, len(content))}
	for index := 0; index < partCount; index++ {
		end := min(len(content), (index+1)*partBytes)
		encoded := base64.RawURLEncoding.EncodeToString(content[index*partBytes : end])
		frames = append(frames, fmt.Sprintf("MSP/1 SNAPSHOT_PART capacity %d %s\n", index, encoded))
	}
	frames = append(frames, "MSP/1 SNAPSHOT_COMMIT capacity\n")
	responses := exchange(t, handler, frames...)
	if got := responses[len(responses)-1]; got != "MSP/1 ACK capacity 1\n" {
		t.Fatalf("commit response = %q", got)
	}
	if holder.Active().Generation() != 1 || !holder.Active().Validated().IsZeroSeries() {
		t.Fatal("one MiB candidate was not atomically installed")
	}
}

func TestConcurrentCommitsHaveUniqueLinearGenerations(t *testing.T) {
	handler, holder := newProtocol(t, nil)
	const count = 24
	var wait sync.WaitGroup
	generations := make(chan string, count)
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			id := fmt.Sprintf("p%d", index)
			responses := exchange(t, handler,
				fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN %s 1 %d\n", id, len(document)),
				fmt.Sprintf("MSP/1 SNAPSHOT_PART %s 0 %s\n", id, base64.RawURLEncoding.EncodeToString([]byte(document))),
				fmt.Sprintf("MSP/1 SNAPSHOT_COMMIT %s\n", id),
			)
			generations <- responses[2]
		}(index)
	}
	wait.Wait()
	close(generations)
	seen := make(map[string]bool)
	for response := range generations {
		parts := strings.Fields(response)
		if len(parts) != 4 || parts[1] != "ACK" || seen[parts[3]] {
			t.Fatalf("response = %q", response)
		}
		seen[parts[3]] = true
	}
	if holder.Active().Generation() != count {
		t.Fatalf("generation = %d", holder.Active().Generation())
	}
}

func TestUnixListenerModeAndPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ingest.sock")
	holder := snapshot.NewHolder(snapshot.Zero())
	core, _ := ingestion.New(holder, snapshot.DefaultLimits(), 4, 0, nil)
	server, err := Listen(path, DefaultConfig(), core, nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o660 {
		t.Fatalf("mode=%v err=%v", info.Mode().Perm(), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	connection, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(connection)
	frames := []string{
		fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN live 1 %d\n", len(document)),
		fmt.Sprintf("MSP/1 SNAPSHOT_PART live 0 %s\n", base64.RawURLEncoding.EncodeToString([]byte(document))),
		"MSP/1 SNAPSHOT_COMMIT live\n",
	}
	for _, frame := range frames {
		connection.Write([]byte(frame))
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	}
	connection.Close()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if holder.Active().Generation() != 1 {
		t.Fatal("socket publication was not installed")
	}
}

func newProtocol(t *testing.T, observer Observer) (*Protocol, *snapshot.Holder) {
	t.Helper()
	holder := snapshot.NewHolder(snapshot.Zero())
	core, err := ingestion.New(holder, snapshot.DefaultLimits(), 64, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	configuration := DefaultConfig()
	configuration.Transactions = 32
	handler, err := NewProtocol(configuration, core, observer)
	if err != nil {
		t.Fatal(err)
	}
	return handler, holder
}

func exchange(t *testing.T, handler *Protocol, frames ...string) []string {
	t.Helper()
	server, client := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- handler.ServeConnection(ctx, server) }()
	reader := bufio.NewReader(client)
	responses := make([]string, 0, len(frames))
	for _, frame := range frames {
		if _, err := client.Write([]byte(frame)); err != nil {
			t.Fatal(err)
		}
		response, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		responses = append(responses, response)
	}
	client.Close()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return responses
}

func assertTranscript(t *testing.T, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("transcript=%q want=%q", got, want)
	}
}
