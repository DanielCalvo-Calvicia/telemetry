package logging_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/pkg/logging"
)

// recording_sink is a consumer-made sink.
type recording_sink struct {
	mutex   sync.Mutex
	entries []logging.SinkEntry
}

func (sink *recording_sink) Send(_ context.Context, entry logging.SinkEntry) error {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	sink.entries = append(sink.entries, entry)
	return nil
}

func server_address(t *testing.T, server *httptest.Server) (string, string) {
	t.Helper()
	server_url, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return server_url.Scheme + "://" + server_url.Hostname(), server_url.Port()
}

func Test_the_ready_made_sinks_are_built_from_their_plain_values(t *testing.T) {
	if logging.NewConsoleSink() == nil {
		t.Error("the console sink is nil")
	}
	if sink, err := logging.NewHTTPSink("http://localhost", "3000", "/logs"); err != nil || sink == nil {
		t.Errorf("http sink = %v, error = %v", sink, err)
	}
	if sink, err := logging.NewAWSHTTPSink("http://localhost", "8080", "/api/v1/cloudwatch/logs/add", "group", "stream"); err != nil || sink == nil {
		t.Errorf("aws http sink = %v, error = %v", sink, err)
	}
}

func Test_a_sink_with_a_missing_value_is_refused_and_is_a_nil_interface(t *testing.T) {
	http_sink, http_err := logging.NewHTTPSink("http://localhost", "", "/logs")
	aws_sink, aws_err := logging.NewAWSHTTPSink("http://localhost", "8080", "/p", "", "stream")

	if http_err == nil || http_sink != nil {
		t.Errorf("http sink = %v, error = %v", http_sink, http_err)
	}
	if aws_err == nil || aws_sink != nil {
		t.Errorf("aws http sink = %v, error = %v", aws_sink, aws_err)
	}
}

func Test_a_logger_built_over_the_sinks_sends_each_entry_to_all_of_them(t *testing.T) {
	var received_body []byte
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		received_body, _ = io.ReadAll(request.Body)
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	host, port := server_address(t, server)
	aws_sink, err := logging.NewAWSHTTPSink(host, port, "/api/v1/cloudwatch/logs/add", "group", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	own_sink := &recording_sink{}
	ctx := context.Background()

	logger, shutdown, err := logging.NewWithSinks(ctx, "production", nil, aws_sink, own_sink)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()
	_ = logger.Debug(ctx, logging.Entry{Message: "below the production minimum"})
	_ = logger.Info(ctx, logging.Entry{Category: "service", Function: "Place", Message: "order placed"})

	var request struct {
		LogGroupName  string `json:"log_group_name"`
		LogStreamName string `json:"log_stream_name"`
		Message       string `json:"message"`
	}
	if err := json.Unmarshal(received_body, &request); err != nil || request.LogGroupName != "group" || request.LogStreamName != "stream" || request.Message == "" {
		t.Errorf("request = %s, error = %v", received_body, err)
	}
	if len(own_sink.entries) != 1 || own_sink.entries[0].Message != "order placed" || own_sink.entries[0].Level != "INFO" {
		t.Errorf("own sink entries = %+v, want only the INFO entry", own_sink.entries)
	}
}

func Test_a_logger_over_sinks_needs_a_known_app_env_and_valid_mask_patterns(t *testing.T) {
	ctx := context.Background()

	if logger, _, err := logging.NewWithSinks(ctx, "staging", nil); err == nil || logger != nil {
		t.Errorf("logger = %v, error = %v for an unknown app env", logger, err)
	}
	if logger, _, err := logging.NewWithSinks(ctx, "production", []string{"("}); err == nil || logger != nil {
		t.Errorf("logger = %v, error = %v for a bad mask pattern", logger, err)
	}
}
