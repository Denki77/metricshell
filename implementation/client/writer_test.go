package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const candidate = `{"schema_version":1,"families":[]}`

type shortConnection struct {
	net.Conn
	maximum int
}

func (connection shortConnection) Write(content []byte) (int, error) {
	return connection.Conn.Write(content[:min(len(content), connection.maximum)])
}

func TestWriterHandlesForcedShortWrites(t *testing.T) {
	server, rawClient := net.Pipe()
	client := shortConnection{Conn: rawClient, maximum: 3}
	writer, err := NewWriter(client, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	done := serveAcknowledgements(server, nil)
	generation, err := writer.Publish(context.Background(), "short", []byte(candidate))
	if err != nil || generation != 1 {
		t.Fatalf("generation=%d err=%v", generation, err)
	}
	client.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestWriterSerializesConcurrentPublications(t *testing.T) {
	server, client := net.Pipe()
	writer, err := NewWriter(client, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	lines := make([]string, 0)
	done := serveAcknowledgements(server, func(line string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, line)
	})
	const publications = 32
	var wait sync.WaitGroup
	errorsFound := make(chan error, publications)
	for index := 0; index < publications; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, err := writer.Publish(context.Background(), fmt.Sprintf("id-%d", index), []byte(candidate))
			errorsFound <- err
		}(index)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	client.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(lines) != publications*3 {
		t.Fatalf("lines = %d", len(lines))
	}
	for offset := 0; offset < len(lines); offset += 3 {
		begin := strings.Fields(lines[offset])
		part := strings.Fields(lines[offset+1])
		commit := strings.Fields(lines[offset+2])
		if begin[1] != "SNAPSHOT_BEGIN" || part[1] != "SNAPSHOT_PART" || commit[1] != "SNAPSHOT_COMMIT" || begin[2] != part[2] || begin[2] != commit[2] {
			t.Fatalf("interleaved frames: %q", lines[offset:offset+3])
		}
	}
}

func TestWriterTypedServerAndConnectionFailures(t *testing.T) {
	tests := []struct {
		name  string
		serve func(net.Conn)
		kind  ErrorKind
		code  string
	}{
		{"nack", func(connection net.Conn) {
			reader := bufio.NewReader(connection)
			reader.ReadString('\n')
			connection.Write([]byte("MSP/1 NACK pub busy\n"))
		}, Rejected, "busy"},
		{"mismatch", func(connection net.Conn) {
			reader := bufio.NewReader(connection)
			reader.ReadString('\n')
			connection.Write([]byte("MSP/1 FRAME_ACCEPTED other BEGIN\n"))
		}, ResponseMismatch, ""},
		{"disconnect", func(connection net.Conn) { connection.Close() }, ClosedConnection, ""},
		{"timeout", func(connection net.Conn) { time.Sleep(100 * time.Millisecond); connection.Close() }, Timeout, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, connection := net.Pipe()
			configuration := DefaultConfig()
			configuration.ReadTimeout = 20 * time.Millisecond
			configuration.WriteTimeout = 20 * time.Millisecond
			writer, err := NewWriter(connection, configuration)
			if err != nil {
				t.Fatal(err)
			}
			go test.serve(server)
			_, err = writer.Publish(context.Background(), "pub", []byte(candidate))
			var failure *Error
			if !errors.As(err, &failure) || failure.Kind != test.kind || failure.Code != test.code {
				t.Fatalf("error = %#v, want kind=%s code=%s", err, test.kind, test.code)
			}
			connection.Close()
		})
	}
}

func TestWriterWaitForSerializationIsCancellable(t *testing.T) {
	server, connection := net.Pipe()
	writer, _ := NewWriter(connection, DefaultConfig())
	<-writer.gate
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := writer.Publish(ctx, "cancel", []byte(candidate)); errorKind(t, err) != Timeout {
		t.Fatalf("error = %v", err)
	}
	writer.gate <- struct{}{}
	server.Close()
	connection.Close()
}

func TestWriterRejectsPayloadBeyondPartCapacity(t *testing.T) {
	server, connection := net.Pipe()
	configuration := DefaultConfig()
	configuration.FrameBytes = 80
	configuration.Parts = 1
	writer, _ := NewWriter(connection, configuration)
	if _, err := writer.Publish(context.Background(), "x", []byte(strings.Repeat("x", 100))); errorKind(t, err) != InvalidInput {
		t.Fatalf("error = %v", err)
	}
	server.Close()
	connection.Close()
}

func serveAcknowledgements(connection net.Conn, observe func(string)) <-chan error {
	done := make(chan error, 1)
	go func() {
		defer connection.Close()
		reader := bufio.NewReader(connection)
		generation := uint64(0)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
					done <- nil
					return
				}
				done <- err
				return
			}
			if observe != nil {
				observe(strings.TrimSuffix(line, "\n"))
			}
			fields := strings.Fields(line)
			var response string
			switch fields[1] {
			case "SNAPSHOT_BEGIN":
				response = fmt.Sprintf("MSP/1 FRAME_ACCEPTED %s BEGIN\n", fields[2])
			case "SNAPSHOT_PART":
				response = fmt.Sprintf("MSP/1 FRAME_ACCEPTED %s %s\n", fields[2], fields[3])
			case "SNAPSHOT_COMMIT":
				generation++
				response = fmt.Sprintf("MSP/1 ACK %s %s\n", fields[2], strconv.FormatUint(generation, 10))
			}
			if _, err := connection.Write([]byte(response)); err != nil {
				done <- err
				return
			}
		}
	}()
	return done
}

func errorKind(t *testing.T, err error) ErrorKind {
	t.Helper()
	kind, ok := ErrorKindOf(err)
	if !ok {
		t.Fatalf("untyped error: %v", err)
	}
	return kind
}
