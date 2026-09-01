package httpingest

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

const validDocument = `{"schema_version":1,"families":[{"name":"jobs","help":"","type":"gauge","series":[{"labels":{},"value":"1"}]}]}`

type fakePublisher struct{ result ingestion.Result }

func (publisher fakePublisher) Publish(context.Context, ingestion.Transport, []byte) ingestion.Result {
	return publisher.result
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorReader) Close() error             { return nil }

func TestHTTPAcceptedIdentityAndGzip(t *testing.T) {
	holder := snapshot.NewHolder(snapshot.Zero())
	core, _ := ingestion.New(holder, snapshot.DefaultLimits(), 4, 0, nil)
	handler, _ := NewHandler(DefaultConfig(), core)

	response := request(t, handler, http.MethodPost, Path, "application/json", "identity", []byte(validDocument))
	if response.Code != http.StatusOK || response.Body.String() != "{\"schema_version\":1,\"status\":\"ack\",\"generation\":1}\n" {
		t.Fatalf("identity: code=%d body=%q", response.Code, response.Body.String())
	}
	response = request(t, handler, http.MethodPost, Path, MediaType+";version=1", "gzip", gzipBytes(t, []byte(validDocument)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"generation":2`) {
		t.Fatalf("gzip: code=%d body=%q", response.Code, response.Body.String())
	}
}

func TestHTTPMethodPathMediaTypeAndEncoding(t *testing.T) {
	handler, _ := NewHandler(DefaultConfig(), fakePublisher{})
	tests := []struct {
		name, method, path, mediaType, encoding string
		status                                  int
		code                                    string
	}{
		{"path", http.MethodPost, "/unknown", "application/json", "", 404, ""},
		{"method", http.MethodGet, Path, "application/json", "", 405, "method"},
		{"media", http.MethodPost, Path, "text/plain", "", 415, "media_type"},
		{"vendor version", http.MethodPost, Path, MediaType + ";version=2", "", 415, "media_type"},
		{"encoding", http.MethodPost, Path, "application/json", "br", 415, "encoding"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, handler, test.method, test.path, test.mediaType, test.encoding, []byte(validDocument))
			if response.Code != test.status || (test.code != "" && !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`)) {
				t.Fatalf("code=%d body=%q", response.Code, response.Body.String())
			}
		})
	}
}

func TestHTTPExactCandidateStatusMapping(t *testing.T) {
	for _, reason := range snapshot.RejectionReasons {
		handler, _ := NewHandler(DefaultConfig(), fakePublisher{result: ingestion.Result{Transport: ingestion.HTTP, Outcome: ingestion.Rejected, Reason: reason}})
		response := request(t, handler, http.MethodPost, Path, "application/json", "", []byte(validDocument))
		if response.Code != rejectionStatus(reason) || !strings.Contains(response.Body.String(), `"code":"`+string(reason)+`"`) {
			t.Fatalf("reason=%s code=%d body=%q", reason, response.Code, response.Body.String())
		}
	}
	for _, test := range []struct {
		outcome ingestion.Outcome
		status  int
		code    string
	}{{ingestion.Busy, 429, "busy"}, {ingestion.Timeout, 408, "timeout"}, {ingestion.InternalError, 500, "internal"}} {
		handler, _ := NewHandler(DefaultConfig(), fakePublisher{result: ingestion.Result{Transport: ingestion.HTTP, Outcome: test.outcome}})
		response := request(t, handler, http.MethodPost, Path, "application/json", "", []byte(validDocument))
		if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("outcome=%s code=%d body=%q", test.outcome, response.Code, response.Body.String())
		}
	}
}

func TestHTTPWireDecodedAndMalformedGzipLimits(t *testing.T) {
	configuration := DefaultConfig()
	configuration.WireBytes = 64
	configuration.DecodedBytes = 128
	handler, _ := NewHandler(configuration, fakePublisher{})

	response := request(t, handler, http.MethodPost, Path, "application/json", "", bytes.Repeat([]byte{'x'}, 65))
	if response.Code != 413 {
		t.Fatalf("wire status = %d", response.Code)
	}
	response = request(t, handler, http.MethodPost, Path, "application/json", "gzip", gzipBytes(t, bytes.Repeat([]byte{' '}, 129)))
	if response.Code != 413 {
		t.Fatalf("gzip bomb status = %d", response.Code)
	}
	response = request(t, handler, http.MethodPost, Path, "application/json", "gzip", []byte("not gzip"))
	if response.Code != 400 || !strings.Contains(response.Body.String(), `"code":"malformed"`) {
		t.Fatalf("malformed gzip status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestHTTPReadCancellationReturnsTimeout(t *testing.T) {
	handler, _ := NewHandler(DefaultConfig(), fakePublisher{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, Path, nil).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Body = errorReader{}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 408 || !strings.Contains(response.Body.String(), `"code":"timeout"`) {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestHTTPLoopbackValidationAndServerTimeouts(t *testing.T) {
	for _, address := range []string{"127.0.0.1:9091", "[::1]:9091", "localhost:9091"} {
		if err := ValidateListenAddress(address); err != nil {
			t.Fatalf("%s: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:9091", ":9091", "[::]:9091", "bad"} {
		if err := ValidateListenAddress(address); err == nil {
			t.Fatalf("accepted non-loopback %q", address)
		}
	}
	handler, _ := NewHandler(DefaultConfig(), fakePublisher{})
	configuration := DefaultConfig()
	server, err := NewServer("127.0.0.1:9091", handler, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if server.ReadHeaderTimeout != configuration.ReadHeaderTimeout || server.ReadTimeout != configuration.ReadTimeout || server.WriteTimeout != configuration.WriteTimeout || server.IdleTimeout != configuration.IdleTimeout || server.MaxHeaderBytes != configuration.MaxHeaderBytes {
		t.Fatal("server did not retain bounded timeouts/header limit")
	}
}

func TestHTTPConcurrentAcceptanceUsesCoreLinearOrder(t *testing.T) {
	holder := snapshot.NewHolder(snapshot.Zero())
	core, _ := ingestion.New(holder, snapshot.DefaultLimits(), 8, 24, nil)
	handler, _ := NewHandler(DefaultConfig(), core)
	const publications = 32
	var wait sync.WaitGroup
	responses := make(chan string, publications)
	for index := 0; index < publications; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response := request(t, handler, http.MethodPost, Path, "application/json", "", []byte(validDocument))
			responses <- response.Body.String()
		}()
	}
	wait.Wait()
	close(responses)
	seen := make(map[string]bool)
	for body := range responses {
		if !strings.Contains(body, `"status":"ack"`) || seen[body] {
			t.Fatalf("duplicate/non-ack response %q", body)
		}
		seen[body] = true
	}
	if holder.Active().Generation() != publications {
		t.Fatalf("generation = %d", holder.Active().Generation())
	}
}

func request(t *testing.T, handler http.Handler, method, path, mediaType, encoding string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(content))
	if mediaType != "" {
		request.Header.Set("Content-Type", mediaType)
	}
	if encoding != "" {
		request.Header.Set("Content-Encoding", encoding)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func gzipBytes(t *testing.T, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
