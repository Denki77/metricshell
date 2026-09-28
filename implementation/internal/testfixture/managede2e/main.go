package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type response struct {
	Outcome    string `json:"outcome"`
	Generation uint64 `json:"generation"`
}

func main() {
	path := os.Getenv("METRICSHELL_MANAGED_SOCKET_PATH")
	if path == "" {
		path = "/run/metricshell/managed.sock"
	}
	waitSocket(path)
	if os.Getenv("MANAGED_E2E_MODE") == "partial-shutdown" {
		partialShutdown(path)
	}
	if os.Getenv("MANAGED_E2E_MODE") == "restart" {
		result := request(path, `{"version":1,"op":"counter_add","name":"jobs","labels":{},"value":"1"}`)
		if result.Outcome != "rejected" || result.Generation != 0 {
			fail("restart did not begin at empty generation zero")
		}
		write(map[string]any{"event": "fixture.managed_restart", "empty_epoch": true})
		return
	}

	declared := request(path, `{"version":1,"op":"declare","name":"jobs","help":"Jobs.","type":"counter","label_names":[],"buckets":[]}`)
	if declared.Outcome != "committed" || declared.Generation != 1 {
		fail("declaration was not committed")
	}
	ackLoss(path, `{"version":1,"op":"counter_add","name":"jobs","labels":{},"value":"1"}`)

	const publishers = 32
	var wait sync.WaitGroup
	errors := make(chan string, publishers)
	for index := 0; index < publishers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result := request(path, `{"version":1,"op":"counter_add","name":"jobs","labels":{},"value":"1"}`)
			if result.Outcome != "committed" {
				errors <- result.Outcome
			}
		}()
	}
	wait.Wait()
	close(errors)
	if outcome := <-errors; outcome != "" {
		fail("concurrent publisher outcome " + outcome)
	}
	missing := request(path, `{"version":1,"op":"counter_add","name":"missing","labels":{},"value":"1"}`)
	if missing.Outcome != "rejected" || missing.Generation != publishers+2 {
		fail(fmt.Sprintf("semantic rejection changed generation: %+v", missing))
	}
	malformed(path)
	waitForLiveMetric("jobs_total 33")
	write(map[string]any{"event": "fixture.managed_e2e", "publishers": publishers, "generation": missing.Generation, "ack_loss_committed": true})
	if text := os.Getenv("MANAGED_E2E_EXIT"); text != "" {
		code, _ := strconv.Atoi(text)
		os.Exit(code)
	}
}

func partialShutdown(path string) {
	connection, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		fail(err.Error())
	}
	defer connection.Close()
	if _, err := connection.Write([]byte(`{"version":1`)); err != nil {
		fail(err.Error())
	}
	signal.Ignore(syscall.SIGTERM, syscall.SIGINT)
	write(map[string]any{"event": "fixture.partial_ready"})
	for {
		time.Sleep(time.Hour)
	}
}

func waitForLiveMetric(metric string) {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get("http://127.0.0.1:9090/metrics")
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readErr == nil && response.StatusCode == http.StatusOK && strings.Contains(string(body), metric) {
				write(map[string]any{"event": "fixture.managed_live", "metric": metric, "workload_running": true})
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	fail("managed metric did not become visible while workload was running")
}

func waitSocket(path string) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("unix", path, 20*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	fail("managed socket did not become ready")
}

func request(path, payload string) response {
	connection, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		fail(err.Error())
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	if _, err := fmt.Fprintln(connection, payload); err != nil {
		fail(err.Error())
	}
	var result response
	if err := json.NewDecoder(connection).Decode(&result); err != nil {
		fail(err.Error())
	}
	return result
}

func ackLoss(path, payload string) {
	connection, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		fail(err.Error())
	}
	_, _ = fmt.Fprintln(connection, payload)
	_ = connection.Close()
	time.Sleep(20 * time.Millisecond)
}

func malformed(path string) {
	connection, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		fail(err.Error())
	}
	defer connection.Close()
	_, _ = fmt.Fprintln(connection, "not-json")
	line, _ := bufio.NewReader(connection).ReadString('\n')
	if !contains(line, `"outcome":"protocol"`) {
		fail("malformed frame did not produce protocol rejection")
	}
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}

func fail(message string) {
	write(map[string]any{"event": "fixture.failed", "error": message})
	os.Exit(1)
}

func write(value map[string]any) {
	_ = json.NewEncoder(os.Stdout).Encode(value)
}
