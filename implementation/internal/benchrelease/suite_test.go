package benchrelease

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/fileingest"
	"github.com/Denki77/metricshell/implementation/internal/httpingest"
	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
	"github.com/Denki77/metricshell/implementation/internal/socketingest"
)

const (
	repetitions = 30
	warmups     = 3
)

var benchmarkSnapshot = []byte(`{"schema_version":1,"families":[{"name":"benchrelease_value","help":"","type":"gauge","series":[{"labels":{"path":"release"},"value":"1"}]}]}`)

type suiteContract struct {
	Repetitions int
	Warmups     int
	Transports  []ingestion.Transport
	Profiles    []string
	Arch        []string
	CPUSet      string
	Memory      string
}

func TestBenchmarkSuiteContract(t *testing.T) {
	contract := suiteContract{
		Repetitions: repetitions,
		Warmups:     warmups,
		Transports:  []ingestion.Transport{ingestion.File, ingestion.Unix, ingestion.HTTP},
		Profiles:    []string{"idle", "load"},
		Arch:        []string{"linux/amd64", "linux/arm64"},
		CPUSet:      "0",
		Memory:      "64MiB",
	}
	if contract.Repetitions < 30 {
		t.Fatalf("repetitions = %d, want >= 30", contract.Repetitions)
	}
	for _, transport := range ingestion.Transports {
		if !slices.Contains(contract.Transports, transport) {
			t.Fatalf("transport %s missing from benchmark contract", transport)
		}
	}
	for _, profile := range []string{"idle", "load"} {
		if !slices.Contains(contract.Profiles, profile) {
			t.Fatalf("profile %s missing from benchmark contract", profile)
		}
	}
	if contract.CPUSet == "" || contract.Memory == "" || len(contract.Arch) != 2 {
		t.Fatalf("resources/arch matrix incomplete: %+v", contract)
	}
}

func BenchmarkReleaseSuite(b *testing.B) {
	for _, transport := range ingestion.Transports {
		b.Run(string(transport)+"/load", func(b *testing.B) {
			driver := newDriver(b, transport)
			for index := 0; index < warmups; index++ {
				driver.publish(b)
			}
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				driver.publish(b)
			}
		})
		b.Run(string(transport)+"/idle", func(b *testing.B) {
			driver := newDriver(b, transport)
			for index := 0; index < warmups; index++ {
				driver.observe(b)
			}
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				driver.observe(b)
			}
		})
	}
}

type driver struct {
	transport ingestion.Transport
	publish   func(testing.TB)
	observe   func(testing.TB)
}

func newDriver(tb testing.TB, transport ingestion.Transport) driver {
	tb.Helper()
	holder := snapshot.NewHolder(snapshot.Zero())
	core, err := ingestion.New(holder, snapshot.DefaultLimits(), 4, 0, nil)
	if err != nil {
		tb.Fatal(err)
	}
	switch transport {
	case ingestion.File:
		directory := tb.TempDir()
		path := filepath.Join(directory, "snapshot.json")
		reconciler, err := fileingest.New(fileingest.Config{Path: path, ReconcileInterval: time.Second, DecodedBytes: snapshot.DefaultLimits().DecodedBytes}, core, nil)
		if err != nil {
			tb.Fatal(err)
		}
		var sequence uint64
		return driver{
			transport: transport,
			publish: func(tb testing.TB) {
				tb.Helper()
				payload := nextSnapshot(&sequence)
				if err := os.WriteFile(path, payload, 0o600); err != nil {
					tb.Fatal(err)
				}
				if got := reconciler.Reconcile(context.Background(), fileingest.Event); got.Outcome != fileingest.Accepted {
					tb.Fatalf("file publish = %#v", got)
				}
			},
			observe: func(testing.TB) { _ = holder.Active().Generation() },
		}
	case ingestion.Unix:
		protocol, err := socketingest.NewProtocol(socketingest.DefaultConfig(), core, nil)
		if err != nil {
			tb.Fatal(err)
		}
		var sequence uint64
		return driver{
			transport: transport,
			publish: func(tb testing.TB) {
				tb.Helper()
				payload := nextSnapshot(&sequence)
				server, client := net.Pipe()
				done := make(chan error, 1)
				go func() { done <- protocol.ServeConnection(context.Background(), server) }()
				reader := bufio.NewReader(client)
				frames := []string{
					"MSP/1 SNAPSHOT_BEGIN bench 1 " + decimal(uint64(len(payload))) + "\n",
					"MSP/1 SNAPSHOT_PART bench 0 " + base64.RawURLEncoding.EncodeToString(payload) + "\n",
					"MSP/1 SNAPSHOT_COMMIT bench\n",
				}
				for _, frame := range frames {
					if _, err := client.Write([]byte(frame)); err != nil {
						tb.Fatal(err)
					}
					response, err := reader.ReadString('\n')
					if err != nil {
						tb.Fatal(err)
					}
					if frame == frames[len(frames)-1] && response != "MSP/1 ACK bench "+decimal(holder.Active().Generation())+"\n" {
						tb.Fatalf("socket response = %q", response)
					}
				}
				_ = client.Close()
				if err := <-done; err != nil {
					tb.Fatal(err)
				}
			},
			observe: func(testing.TB) { _ = holder.Active().Generation() },
		}
	case ingestion.HTTP:
		handler, err := httpingest.NewHandler(httpingest.DefaultConfig(), core)
		if err != nil {
			tb.Fatal(err)
		}
		var sequence uint64
		return driver{
			transport: transport,
			publish: func(tb testing.TB) {
				tb.Helper()
				request := httptest.NewRequest(http.MethodPost, httpingest.Path, bytes.NewReader(nextSnapshot(&sequence)))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					tb.Fatalf("http response = %d/%s", response.Code, response.Body.String())
				}
			},
			observe: func(testing.TB) { _ = holder.Active().Generation() },
		}
	default:
		tb.Fatalf("unsupported transport %s", transport)
		return driver{}
	}
}

func nextSnapshot(sequence *uint64) []byte {
	(*sequence)++
	return []byte(`{"schema_version":1,"families":[{"name":"benchrelease_value","help":"","type":"gauge","series":[{"labels":{"path":"release","sequence":"` + decimal(*sequence) + `"},"value":"` + decimal(*sequence) + `"}]}]}`)
}

func decimal(value uint64) string {
	if value == 0 {
		return "0"
	}
	var content [20]byte
	index := len(content)
	for value > 0 {
		index--
		content[index] = byte('0' + value%10)
		value /= 10
	}
	return string(content[index:])
}
