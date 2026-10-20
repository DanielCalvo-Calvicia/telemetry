package aws_http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	aws_http "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/aws_http"
	external "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/aws_http/external"
)

const (
	test_log_group  = "/snowfactory/test"
	test_log_stream = "telemetry"
	test_path       = "/api/v1/cloudwatch/logs/add"
)

var reference_entry = dtos.LogEntryOutboundDTO{
	ProducerID:    "producer",
	ApplicationID: "application-id",
	ProcessID:     "process-id",
	Application:   "app",
	Category:      "service",
	Function:      "Put",
	Indentation:   2,
	Depth:         3,
	Level:         "INFO",
	Message:       "stored",
	Fields:        map[string]any{"bucket": "b"},
	Timestamp:     time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
}

// add_log_request is what the add_log endpoint of aws_microservice reads.
type add_log_request struct {
	LogGroupName  string `json:"log_group_name"`
	LogStreamName string `json:"log_stream_name"`
	Message       string `json:"message"`
	Timestamp     int64  `json:"timestamp"`
}

func split_server_address(t *testing.T, server *httptest.Server) (string, string) {
	t.Helper()
	server_url, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return server_url.Scheme + "://" + server_url.Hostname(), server_url.Port()
}

func new_logger(t *testing.T, server *httptest.Server, timeout time.Duration) *aws_http.AWSHTTPLogger {
	t.Helper()
	host, port := split_server_address(t, server)
	aws_http_logger, err := aws_http.NewAWSHTTPLoggerWithTimeout(host, port, test_path, test_log_group, test_log_stream, timeout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return aws_http_logger
}

func Test_send_adapts_the_entry_to_the_add_log_contract(t *testing.T) {
	var received_method, received_path, received_content_type string
	var received_request add_log_request
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		received_method, received_path, received_content_type = request.Method, request.URL.Path, request.Header.Get("Content-Type")
		request_body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(request_body, &received_request)
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := new_logger(t, server, time.Second).Send(context.Background(), reference_entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received_method != http.MethodPost || received_path != test_path || received_content_type != "application/json" {
		t.Errorf("request = %s %s %s", received_method, received_path, received_content_type)
	}
	if received_request.LogGroupName != test_log_group || received_request.LogStreamName != test_log_stream {
		t.Errorf("target = %q / %q", received_request.LogGroupName, received_request.LogStreamName)
	}
	if received_request.Timestamp != reference_entry.Timestamp.UnixMilli() {
		t.Errorf("timestamp = %d, want %d", received_request.Timestamp, reference_entry.Timestamp.UnixMilli())
	}
	var message_entry dtos.LogEntryOutboundDTO
	if err := json.Unmarshal([]byte(received_request.Message), &message_entry); err != nil {
		t.Fatalf("the message is not the encoded entry: %v", err)
	}
	if message_entry.Message != "stored" || message_entry.Level != "INFO" || message_entry.Depth != 3 || message_entry.Fields["bucket"] != "b" {
		t.Errorf("message entry = %+v", message_entry)
	}
}

func Test_a_status_of_400_or_above_is_a_delivery_failure_that_keeps_the_status(t *testing.T) {
	test_cases := []struct {
		name        string
		status_code int
		expect_fail bool
	}{
		{"ok", http.StatusOK, false},
		{"bad request", http.StatusBadRequest, true},
		{"server error", http.StatusInternalServerError, true},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(test_case.status_code)
			}))
			defer server.Close()

			err := new_logger(t, server, time.Second).Send(context.Background(), reference_entry)

			if !test_case.expect_fail {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
				t.Errorf("error = %v, want ErrLogSinkDeliveryFailed", err)
			}
			var status_error external.StatusError
			if !errors.As(err, &status_error) || status_error.StatusCode != test_case.status_code {
				t.Errorf("error = %v, want the status %d kept as root cause", err, test_case.status_code)
			}
		})
	}
}

func Test_a_slow_receiver_is_abandoned_at_the_timeout(t *testing.T) {
	release_server := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { <-release_server }))
	defer server.Close()
	defer close(release_server)

	started_at := time.Now()
	err := new_logger(t, server, 50*time.Millisecond).Send(context.Background(), reference_entry)

	if !errors.Is(err, application.ErrLogSinkDeliveryFailed) || time.Since(started_at) > 2*time.Second {
		t.Errorf("error = %v, elapsed = %v", err, time.Since(started_at))
	}
}

func Test_an_unreachable_receiver_is_a_delivery_failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	aws_http_logger := new_logger(t, server, time.Second)
	server.Close()

	if err := aws_http_logger.Send(context.Background(), reference_entry); !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
		t.Errorf("error = %v, want ErrLogSinkDeliveryFailed", err)
	}
}

func Test_an_entry_that_cannot_be_encoded_is_a_delivery_failure_and_is_not_sent(t *testing.T) {
	requests_received := 0
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { requests_received++ }))
	defer server.Close()
	unencodable_entry := reference_entry
	unencodable_entry.Fields = map[string]any{"callback": func() {}}

	err := new_logger(t, server, time.Second).Send(context.Background(), unencodable_entry)

	if !errors.Is(err, application.ErrLogSinkDeliveryFailed) || requests_received != 0 {
		t.Errorf("error = %v, requests = %d", err, requests_received)
	}
}

func Test_an_invalid_endpoint_is_a_delivery_failure(t *testing.T) {
	aws_http_logger, err := aws_http.NewAWSHTTPLogger("http://exa mple", "80", test_path, test_log_group, test_log_stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := aws_http_logger.Send(context.Background(), reference_entry); !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
		t.Errorf("error = %v, want ErrLogSinkDeliveryFailed", err)
	}
}

func Test_the_endpoint_and_the_target_are_required(t *testing.T) {
	test_cases := []struct {
		name        string
		host        string
		port        string
		path        string
		log_group   string
		log_stream  string
		expectation error
	}{
		{"missing host", "", "8080", test_path, test_log_group, test_log_stream, application.ErrLogSinkEndpointMissing},
		{"missing port", "http://localhost", "", test_path, test_log_group, test_log_stream, application.ErrLogSinkEndpointMissing},
		{"missing path", "http://localhost", "8080", "", test_log_group, test_log_stream, application.ErrLogSinkEndpointMissing},
		{"missing log group", "http://localhost", "8080", test_path, "", test_log_stream, application.ErrLogSinkAWSTargetMissing},
		{"missing log stream", "http://localhost", "8080", test_path, test_log_group, "", application.ErrLogSinkAWSTargetMissing},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			aws_http_logger, err := aws_http.NewAWSHTTPLogger(test_case.host, test_case.port, test_case.path, test_case.log_group, test_case.log_stream)
			if !errors.Is(err, test_case.expectation) || aws_http_logger != nil {
				t.Errorf("got %v, %v; want %v and no logger", aws_http_logger, err, test_case.expectation)
			}
		})
	}
}

func Test_the_standard_timeout_is_three_seconds(t *testing.T) {
	if aws_http.RequestTimeout != 3*time.Second {
		t.Errorf("RequestTimeout = %v, want 3s", aws_http.RequestTimeout)
	}
	if _, err := aws_http.NewAWSHTTPLogger("http://localhost", "8080", test_path, test_log_group, test_log_stream); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func Test_status_error_message_includes_the_status_code(t *testing.T) {
	if message := (external.StatusError{StatusCode: 502}).Error(); message != "AWS log receiver answered with status 502" {
		t.Errorf("Error() = %q", message)
	}
}
