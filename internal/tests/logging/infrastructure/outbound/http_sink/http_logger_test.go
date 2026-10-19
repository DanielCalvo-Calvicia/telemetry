package http_sink_test

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
	http_sink "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/http_sink"
	external "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/http_sink/external"
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

// split_server_address returns the host (with scheme) and the port of a test server.
func split_server_address(t *testing.T, server *httptest.Server) (string, string) {
	t.Helper()
	server_url, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return server_url.Scheme + "://" + server_url.Hostname(), server_url.Port()
}

func new_logger(t *testing.T, server *httptest.Server, timeout time.Duration) *http_sink.HTTPLogger {
	t.Helper()
	host, port := split_server_address(t, server)
	http_logger, err := http_sink.NewHTTPLoggerWithTimeout(host, port, "/logs", timeout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return http_logger
}

func Test_send_posts_the_flat_json_payload_to_the_configured_path(t *testing.T) {
	var received_method, received_path, received_content_type string
	var received_entry dtos.LogEntryOutboundDTO
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		received_method, received_path, received_content_type = request.Method, request.URL.Path, request.Header.Get("Content-Type")
		request_body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(request_body, &received_entry)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := new_logger(t, server, time.Second).Send(context.Background(), reference_entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received_method != http.MethodPost || received_path != "/logs" || received_content_type != "application/json" {
		t.Errorf("request = %s %s %s", received_method, received_path, received_content_type)
	}
	if received_entry.Message != "stored" || received_entry.Level != "INFO" || received_entry.Depth != 3 || received_entry.Fields["bucket"] != "b" {
		t.Errorf("received entry = %+v", received_entry)
	}
}

func Test_a_status_of_400_or_above_is_a_delivery_failure_that_keeps_the_status(t *testing.T) {
	test_cases := []struct {
		name        string
		status_code int
		expect_fail bool
	}{
		{"ok", http.StatusOK, false},
		{"accepted", http.StatusAccepted, false},
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
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release_server
	}))
	defer server.Close()
	defer close(release_server)

	started_at := time.Now()
	err := new_logger(t, server, 50*time.Millisecond).Send(context.Background(), reference_entry)

	if !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
		t.Errorf("error = %v, want ErrLogSinkDeliveryFailed", err)
	}
	if time.Since(started_at) > 2*time.Second {
		t.Error("the send must be abandoned shortly after the timeout")
	}
}

func Test_an_unreachable_receiver_is_a_delivery_failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	http_logger := new_logger(t, server, time.Second)
	server.Close()

	if err := http_logger.Send(context.Background(), reference_entry); !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
		t.Errorf("error = %v, want ErrLogSinkDeliveryFailed", err)
	}
}

func Test_a_cancelled_context_is_a_delivery_failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer server.Close()
	cancelled_context, cancel := context.WithCancel(context.Background())
	cancel()

	if err := new_logger(t, server, time.Second).Send(cancelled_context, reference_entry); !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
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
	http_logger, err := http_sink.NewHTTPLogger("http://exa mple", "80", "/logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := http_logger.Send(context.Background(), reference_entry); !errors.Is(err, application.ErrLogSinkDeliveryFailed) {
		t.Errorf("error = %v, want ErrLogSinkDeliveryFailed", err)
	}
}

func Test_the_host_the_port_and_the_path_are_required(t *testing.T) {
	test_cases := []struct {
		name string
		host string
		port string
		path string
	}{
		{"missing host", "", "3000", "/logs"},
		{"missing port", "http://localhost", "", "/logs"},
		{"missing path", "http://localhost", "3000", ""},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			http_logger, err := http_sink.NewHTTPLogger(test_case.host, test_case.port, test_case.path)
			if !errors.Is(err, application.ErrLogSinkEndpointMissing) || http_logger != nil {
				t.Errorf("got %v, %v; want ErrLogSinkEndpointMissing and no logger", http_logger, err)
			}
		})
	}
}

func Test_the_standard_timeout_is_three_seconds(t *testing.T) {
	if http_sink.RequestTimeout != 3*time.Second {
		t.Errorf("RequestTimeout = %v, want 3s (Q18-15)", http_sink.RequestTimeout)
	}
	if _, err := http_sink.NewHTTPLogger("http://localhost", "3000", "/logs"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func Test_status_error_message_includes_the_status_code(t *testing.T) {
	if message := (external.StatusError{StatusCode: 502}).Error(); message != "log receiver answered with status 502" {
		t.Errorf("Error() = %q", message)
	}
}
