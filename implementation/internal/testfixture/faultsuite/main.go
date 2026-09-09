package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	ingestionURL = "http://127.0.0.1:9091/v1/metrics"
	metricsURL   = "http://127.0.0.1:9090/metrics"
)

var validSnapshot = []byte(`{"schema_version":1,"families":[{"name":"faultsuite_value","help":"","type":"gauge","series":[{"labels":{"case":"ok"},"value":"1"}]}]}`)

func main() {
	mode := os.Getenv("FAULTSUITE_MODE")
	if mode == "" {
		mode = "malformed-flood"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	var err error
	switch mode {
	case "malformed-flood":
		err = malformedFlood(client)
	case "slow-disconnect":
		err = slowDisconnect(client)
	case "saturation":
		err = saturation()
	case "scrape-race":
		err = scrapeRace(client)
	case "oom":
		forcedOOM()
	default:
		err = fmt.Errorf("unknown fault suite mode %q", mode)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"event":"fixture.faultsuite","mode":%q,"ok":false,"error":%q}`+"\n", mode, err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, `{"event":"fixture.faultsuite","mode":%q,"ok":true}`+"\n", mode)
}

func malformedFlood(client *http.Client) error {
	for index := 0; index < 64; index++ {
		status, body, err := post(client, []byte(`{`))
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest || !strings.Contains(body, `"status":"nack"`) {
			return fmt.Errorf("malformed response %d/%s", status, body)
		}
	}
	status, body, err := post(client, validSnapshot)
	if err != nil {
		return err
	}
	if status != http.StatusOK || !strings.Contains(body, `"status":"ack"`) {
		return fmt.Errorf("recovery response %d/%s", status, body)
	}
	return nil
}

func slowDisconnect(client *http.Client) error {
	connection, err := net.DialTimeout("tcp", "127.0.0.1:9091", time.Second)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(connection, "POST /v1/metrics HTTP/1.1\r\nHost: 127.0.0.1\r\nContent-Type: application/json\r\nContent-Length: 64\r\n\r\n{"); err != nil {
		_ = connection.Close()
		return err
	}
	time.Sleep(50 * time.Millisecond)
	_ = connection.Close()
	status, body, err := post(client, validSnapshot)
	if err != nil {
		return err
	}
	if status != http.StatusOK || !strings.Contains(body, `"status":"ack"`) {
		return fmt.Errorf("post-disconnect recovery response %d/%s", status, body)
	}
	return nil
}

func saturation() error {
	client := &http.Client{Timeout: 5 * time.Second}
	payload := saturationSnapshot(750)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		statuses := make(chan int, 32)
		var wait sync.WaitGroup
		for index := 0; index < cap(statuses); index++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				status, _, err := post(client, payload)
				if err != nil {
					statuses <- 0
					return
				}
				statuses <- status
			}()
		}
		wait.Wait()
		close(statuses)
		accepted, busy := 0, 0
		for status := range statuses {
			switch status {
			case http.StatusOK:
				accepted++
			case http.StatusTooManyRequests:
				busy++
			case 0:
				return fmt.Errorf("request failed during saturation")
			}
		}
		if accepted > 0 && busy > 0 {
			return nil
		}
	}
	return fmt.Errorf("did not observe deterministic busy rejection")
}

func scrapeRace(client *http.Client) error {
	var wait sync.WaitGroup
	errors := make(chan error, 32)
	statuses := make(chan int, 16)
	for index := 0; index < 16; index++ {
		wait.Add(2)
		go func(index int) {
			defer wait.Done()
			payload := []byte(fmt.Sprintf(`{"schema_version":1,"families":[{"name":"faultsuite_race","help":"","type":"gauge","series":[{"labels":{"worker":"%d"},"value":"%d"}]}]}`, index, index))
			status, _, err := post(client, payload)
			statuses <- status
			if err != nil || (status != http.StatusOK && status != http.StatusTooManyRequests) {
				errors <- fmt.Errorf("publish %d status=%d err=%v", index, status, err)
			}
		}(index)
		go func() {
			defer wait.Done()
			response, err := client.Get(metricsURL)
			if err != nil {
				errors <- err
				return
			}
			defer response.Body.Close()
			_, _ = io.Copy(io.Discard, response.Body)
			if response.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("scrape status=%d", response.StatusCode)
			}
		}()
	}
	wait.Wait()
	close(errors)
	close(statuses)
	for err := range errors {
		if err != nil {
			return err
		}
	}
	accepted := 0
	for status := range statuses {
		if status == http.StatusOK {
			accepted++
		}
	}
	if accepted == 0 {
		return fmt.Errorf("race did not preserve any successful publication")
	}
	return nil
}

func forcedOOM() {
	blocks := make([][]byte, 0, 256)
	for {
		block := bytes.Repeat([]byte{1}, 4<<20)
		blocks = append(blocks, block)
		time.Sleep(10 * time.Millisecond)
	}
}

func post(client *http.Client, payload []byte) (int, string, error) {
	request, err := http.NewRequest(http.MethodPost, ingestionURL, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	return response.StatusCode, string(body), err
}

func saturationSnapshot(series int) []byte {
	type item struct {
		Labels map[string]string `json:"labels"`
		Value  string            `json:"value"`
	}
	values := make([]item, 0, series)
	for index := 0; index < series; index++ {
		values = append(values, item{Labels: map[string]string{"i": fmt.Sprintf("%d", index)}, Value: fmt.Sprintf("%d", index)})
	}
	document := struct {
		SchemaVersion int `json:"schema_version"`
		Families      []struct {
			Name   string `json:"name"`
			Help   string `json:"help"`
			Type   string `json:"type"`
			Series []item `json:"series"`
		} `json:"families"`
	}{SchemaVersion: 1}
	document.Families = append(document.Families, struct {
		Name   string `json:"name"`
		Help   string `json:"help"`
		Type   string `json:"type"`
		Series []item `json:"series"`
	}{Name: "faultsuite_saturation", Type: "gauge", Series: values})
	encoded, _ := json.Marshal(document)
	return encoded
}
