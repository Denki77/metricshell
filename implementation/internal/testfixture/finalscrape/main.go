package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	target := os.Getenv("FIXTURE_TARGET")
	if target == "" {
		target = "http://metricshell:9090/metrics"
	}
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	completed := 0
	failuresAfterCompletion := 0
	for time.Now().Before(deadline) {
		request, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			fail(err)
		}
		request.Header.Set("Accept", "application/openmetrics-text; version=1.0.0")
		response, err := client.Do(request)
		if err != nil {
			if completed > 0 {
				failuresAfterCompletion++
				if failuresAfterCompletion == 3 {
					write(map[string]any{"event": "fixture.final_scrape", "completed_responses": completed})
					return
				}
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		failuresAfterCompletion = 0
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			fail(fmt.Errorf("response read failed: %v / %v", readErr, closeErr))
		}
		if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "application/openmetrics-text;") || strings.Count(string(body), "# EOF\n") != 1 || !strings.HasSuffix(string(body), "# EOF\n") {
			fail(fmt.Errorf("invalid exposition response: status=%d content_type=%q", response.StatusCode, response.Header.Get("Content-Type")))
		}
		completed++
		time.Sleep(20 * time.Millisecond)
	}
	fail(fmt.Errorf("MetricShell did not finish after %d complete responses", completed))
}

func fail(err error) {
	write(map[string]any{"event": "fixture.failed", "error": err.Error()})
	os.Exit(1)
}

func write(record map[string]any) {
	if err := json.NewEncoder(os.Stdout).Encode(record); err != nil {
		os.Exit(1)
	}
}
