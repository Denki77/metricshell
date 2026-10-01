package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const application = `{"schema_version":1,"families":[{"name":"example_jobs","help":"Completed example jobs.","type":"counter","series":[{"labels":{},"value":"3"}]},{"name":"example_queue_depth","help":"Queued example jobs.","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`
const zero = `{"schema_version":1,"families":[]}`

func main() {
	transport := required("FIXTURE_TRANSPORT")
	payloadName := required("FIXTURE_PAYLOAD")
	payload, valid := selectPayload(payloadName)

	var err error
	switch transport {
	case "file":
		err = publishFile(payload)
	case "unix":
		err = publishUnix(payloadName, payload, valid)
	case "http":
		err = publishHTTP(payload, valid)
	default:
		err = fmt.Errorf("unknown transport %q", transport)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("snapshot published transport=%s payload=%s\n", transport, payloadName)
}

func required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fmt.Fprintf(os.Stderr, "%s is required\n", name)
		os.Exit(2)
	}
	return value
}

func selectPayload(name string) ([]byte, bool) {
	switch name {
	case "application":
		return []byte(application), true
	case "zero":
		return []byte(zero), true
	case "malformed":
		return []byte(`{"schema_version":1,"families":[`), false
	default:
		fmt.Fprintf(os.Stderr, "unknown payload %q\n", name)
		os.Exit(2)
		return nil, false
	}
}

func publishFile(payload []byte) error {
	path := envOr("FIXTURE_FILE_PATH", "/run/metricshell/snapshot.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + ".candidate"
	if err := os.WriteFile(temporary, append(payload, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func publishUnix(id string, payload []byte, valid bool) error {
	path := envOr("FIXTURE_UNIX_PATH", "/run/metricshell/ingest.sock")
	var connection net.Conn
	var err error
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		connection, err = net.DialTimeout("unix", path, time.Second)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	defer connection.Close()
	publicationID := "example_" + id
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	frames := fmt.Sprintf("MSP/1 SNAPSHOT_BEGIN %s 1 %d\nMSP/1 SNAPSHOT_PART %s 0 %s\nMSP/1 SNAPSHOT_COMMIT %s\n", publicationID, len(payload), publicationID, encoded, publicationID)
	if _, err = connection.Write([]byte(frames)); err != nil {
		return err
	}
	reader := bufio.NewReader(connection)
	responses := make([]string, 0, 3)
	for len(responses) < 3 {
		if err = connection.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			return err
		}
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return readErr
		}
		responses = append(responses, line)
	}
	last := responses[len(responses)-1]
	if valid && !strings.HasPrefix(last, "MSP/1 ACK ") {
		return fmt.Errorf("expected ACK, got %q", last)
	}
	if !valid && !strings.Contains(last, " NACK ") {
		return fmt.Errorf("expected NACK, got %q", last)
	}
	return nil
}

func publishHTTP(payload []byte, valid bool) error {
	url := envOr("FIXTURE_HTTP_URL", "http://http:9091/v1/metrics")
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		_ = response.Body.Close()
		if valid && response.StatusCode/100 == 2 {
			return nil
		}
		if !valid && response.StatusCode == http.StatusBadRequest {
			return nil
		}
		return fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	return fmt.Errorf("HTTP ingestion endpoint did not become ready")
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
