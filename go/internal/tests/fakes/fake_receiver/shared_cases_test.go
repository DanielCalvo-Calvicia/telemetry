package fake_receiver_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes/fake_receiver"
)

const shared_cases_path = "../../../../../integration-tests/fixtures/http/receiver_script_examples.json"

type shared_file struct {
	Cases []shared_case `json:"cases"`
}

type shared_case struct {
	Description string `json:"description"`
	Config      struct {
		ExpectedBearer string `json:"expected_bearer"`
		ExpectedAPIKey *struct {
			Header string `json:"header"`
			Value  string `json:"value"`
		} `json:"expected_api_key"`
		DefaultStatus int `json:"default_status"`
	} `json:"config"`
	Script []struct {
		Status     int    `json:"status"`
		RetryAfter string `json:"retry_after"`
		Location   string `json:"location"`
		DelayMs    int    `json:"delay_ms"`
		Close      bool   `json:"close"`
		Truncate   bool   `json:"truncate"`
	} `json:"script"`
	Requests []struct {
		Body    string            `json:"body"`
		Gzip    bool              `json:"gzip"`
		Headers map[string]string `json:"headers"`
	} `json:"requests"`
	Expected []struct {
		Status       json.RawMessage   `json:"status"`
		Headers      map[string]string `json:"headers"`
		BodyContains string            `json:"body_contains"`
		MinElapsedMs int               `json:"min_elapsed_ms"`
	} `json:"expected"`
	ExpectedRecorded struct {
		Requests     int      `json:"requests"`
		Events       int      `json:"events"`
		Invalid      []string `json:"invalid"`
		GzipRequests int      `json:"gzip_requests"`
	} `json:"expected_recorded"`
}

func Test_the_shared_cases_give_the_same_answers_and_records(t *testing.T) {
	content, read_error := os.ReadFile(shared_cases_path)
	if read_error != nil {
		t.Fatal(read_error)
	}
	var file shared_file
	if decode_error := json.Unmarshal(content, &file); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(file.Cases) == 0 {
		t.Fatal("receiver_script_examples.json holds no case")
	}
	for _, case_item := range file.Cases {
		t.Run(case_item.Description, func(t *testing.T) { run_shared_case(t, case_item) })
	}
}

func run_shared_case(t *testing.T, case_item shared_case) {
	config := fake_receiver.Config{DefaultStatus: case_item.Config.DefaultStatus, ExpectedBearer: case_item.Config.ExpectedBearer}
	if case_item.Config.ExpectedAPIKey != nil {
		config.ExpectedAPIKeyHeader = case_item.Config.ExpectedAPIKey.Header
		config.ExpectedAPIKeyValue = case_item.Config.ExpectedAPIKey.Value
	}
	receiver := fake_receiver.New(config)
	defer receiver.Close()
	for _, scripted := range case_item.Script {
		receiver.Script(fake_receiver.Answer{
			Status: scripted.Status, RetryAfter: scripted.RetryAfter, Location: scripted.Location,
			Delay: time.Duration(scripted.DelayMs) * time.Millisecond, Close: scripted.Close, Truncate: scripted.Truncate,
		})
	}
	transport := &http.Transport{DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport:     transport,
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	if len(case_item.Requests) != len(case_item.Expected) {
		t.Fatalf("the case has %d requests and %d expected answers", len(case_item.Requests), len(case_item.Expected))
	}
	for position, sent := range case_item.Requests {
		body := []byte(sent.Body)
		if sent.Gzip {
			var compressed bytes.Buffer
			writer := gzip.NewWriter(&compressed)
			_, _ = writer.Write(body)
			_ = writer.Close()
			body = compressed.Bytes()
		}
		request, request_error := http.NewRequest(http.MethodPost, receiver.URL(), bytes.NewReader(body))
		if request_error != nil {
			t.Fatal(request_error)
		}
		request.Header.Set("Content-Type", "application/json")
		if sent.Gzip {
			request.Header.Set("Content-Encoding", "gzip")
		}
		for name, value := range sent.Headers {
			request.Header.Set(name, value)
		}
		started := time.Now()
		response, do_error := client.Do(request)
		expected := case_item.Expected[position]
		var got_status string
		var response_text string
		if do_error != nil {
			got_status = "connection_error"
		} else {
			response_body, body_error := io.ReadAll(response.Body)
			response.Body.Close()
			response_text = string(response_body)
			if body_error != nil {
				got_status = "body_error"
			} else {
				got_status = strconv.Itoa(response.StatusCode)
			}
		}
		elapsed := time.Since(started)
		if got_status != strings.Trim(string(expected.Status), `"`) {
			t.Errorf("request %d: expected status %s, got %s (%v)", position, expected.Status, got_status, do_error)
			continue
		}
		for name, value := range expected.Headers {
			if response == nil || response.Header.Get(name) != value {
				t.Errorf("request %d: header %s must be %q", position, name, value)
			}
		}
		if !strings.Contains(response_text, expected.BodyContains) {
			t.Errorf("request %d: the body %q must contain %q", position, response_text, expected.BodyContains)
		}
		if elapsed < time.Duration(expected.MinElapsedMs)*time.Millisecond {
			t.Errorf("request %d: answered after %v, at least %d ms expected", position, elapsed, expected.MinElapsedMs)
		}
	}

	records := receiver.Requests()
	if len(records) != case_item.ExpectedRecorded.Requests {
		t.Fatalf("recorded %d requests, expected %d", len(records), case_item.ExpectedRecorded.Requests)
	}
	events, gzipped := 0, 0
	var reasons []string
	for _, record := range records {
		events += len(record.Events)
		if record.Gzipped {
			gzipped++
		}
		if record.InvalidReason != "" {
			reasons = append(reasons, record.InvalidReason)
		}
	}
	if events != case_item.ExpectedRecorded.Events || len(receiver.Events()) != events {
		t.Errorf("recorded %d events, expected %d", events, case_item.ExpectedRecorded.Events)
	}
	if gzipped != case_item.ExpectedRecorded.GzipRequests {
		t.Errorf("recorded %d gzipped requests, expected %d", gzipped, case_item.ExpectedRecorded.GzipRequests)
	}
	if len(reasons) != len(case_item.ExpectedRecorded.Invalid) {
		t.Fatalf("invalid reasons %q, expected %d of them", reasons, len(case_item.ExpectedRecorded.Invalid))
	}
	for position, part := range case_item.ExpectedRecorded.Invalid {
		if !strings.Contains(reasons[position], part) {
			t.Errorf("reason %q must contain %q", reasons[position], part)
		}
	}
}
