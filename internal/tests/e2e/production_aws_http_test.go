package e2e_test

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

// add_log_request is what the add_log endpoint of aws_microservice reads.
type add_log_request struct {
	LogGroupName  string `json:"log_group_name"`
	LogStreamName string `json:"log_stream_name"`
	Message       string `json:"message"`
	Timestamp     int64  `json:"timestamp"`
}

// logged_entry is the flat payload carried in the message of the request.
type logged_entry struct {
	Application string
	Category    string
	Function    string
	Indentation int
	Level       string
	Message     string
	Fields      map[string]any
}

// fake_aws_microservice answers like the add_log endpoint and records what it receives.
type fake_aws_microservice struct {
	server   *httptest.Server
	mutex    sync.Mutex
	paths    []string
	requests []add_log_request
}

func start_fake_aws_microservice(t *testing.T) *fake_aws_microservice {
	t.Helper()
	fake := &fake_aws_microservice{}
	fake.server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		var received add_log_request
		if err := json.Unmarshal(body, &received); err != nil || received.LogGroupName == "" || received.LogStreamName == "" || received.Message == "" || received.Timestamp == 0 {
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		fake.mutex.Lock()
		fake.paths = append(fake.paths, request.URL.Path)
		fake.requests = append(fake.requests, received)
		fake.mutex.Unlock()
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"logged":true}`))
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *fake_aws_microservice) config(t *testing.T) logging.Config {
	t.Helper()
	server_url, err := url.Parse(fake.server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return logging.Config{
		AWSHTTPEnabled:   true,
		AWSHTTPHost:      server_url.Scheme + "://" + server_url.Hostname(),
		AWSHTTPPort:      server_url.Port(),
		AWSHTTPPath:      "/api/v1/cloudwatch/logs/add",
		AWSHTTPLogGroup:  "/snowfactory/e2e",
		AWSHTTPLogStream: "telemetry",
	}
}

// Test_a_service_in_production_sends_its_logs_to_the_add_log_endpoint plays a small service that uses only the public
// API, in production, against a fake aws_microservice.
func Test_a_service_in_production_sends_its_logs_to_the_add_log_endpoint(t *testing.T) {
	fake := start_fake_aws_microservice(t)
	ctx := context.Background()
	logger, shutdown, err := logging.New(ctx, "production", fake.config(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	request_ctx, err := logging.NewRequestContext(ctx, "e2e-service", "instance-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	service_ctx, _ := logging.NextIndentation(request_ctx)
	_ = logger.Debug(service_ctx, logging.Entry{Category: "service", Function: "Place", Message: "below the production minimum"})
	_ = logger.Info(service_ctx, logging.Entry{Category: "service", Function: "Place", Message: "order placed", Fields: map[string]any{"order_id": "o-1", "password": "must-be-masked"}})
	_ = logger.Warn(service_ctx, logging.Entry{Category: "service", Function: "Place", Message: "slow payment"})
	_ = logger.Error(service_ctx, logging.Entry{Category: "service", Function: "Place", Message: "email failed"})

	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if len(fake.requests) != 3 {
		t.Fatalf("requests = %d, want 3 (INFO, WARN, ERROR; the DEBUG entry is below the production minimum)", len(fake.requests))
	}
	wanted_levels := []string{"INFO", "WARN", "ERROR"}
	for index, request := range fake.requests {
		if fake.paths[index] != "/api/v1/cloudwatch/logs/add" || request.LogGroupName != "/snowfactory/e2e" || request.LogStreamName != "telemetry" {
			t.Errorf("request %d = %s %+v", index, fake.paths[index], request)
		}
		var entry logged_entry
		if err := json.Unmarshal([]byte(request.Message), &entry); err != nil {
			t.Fatalf("request %d: the message is not the encoded entry: %v", index, err)
		}
		if entry.Level != wanted_levels[index] || entry.Application != "e2e-service" || entry.Function != "Place" || entry.Indentation != logging.StartingIndentation+1 {
			t.Errorf("request %d entry = %+v", index, entry)
		}
	}

	var first logged_entry
	_ = json.Unmarshal([]byte(fake.requests[0].Message), &first)
	if first.Fields["order_id"] != "o-1" {
		t.Errorf("fields = %v, the order id must be kept", first.Fields)
	}
	if first.Fields["password"] == "must-be-masked" {
		t.Errorf("fields = %v, the password must be masked", first.Fields)
	}
}

func Test_the_same_service_in_development_also_sends_the_debug_entries(t *testing.T) {
	fake := start_fake_aws_microservice(t)
	ctx := context.Background()
	logger, shutdown, err := logging.New(ctx, "development", fake.config(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	_ = logger.Debug(ctx, logging.Entry{Category: "service", Function: "Place", Message: "checking the stock"})

	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if len(fake.requests) != 1 {
		t.Errorf("requests = %d, want 1", len(fake.requests))
	}
}

func Test_an_unreachable_endpoint_does_not_stop_the_service(t *testing.T) {
	fake := start_fake_aws_microservice(t)
	cfg := fake.config(t)
	fake.server.Close()
	ctx := context.Background()
	logger, shutdown, err := logging.New(ctx, "production", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	// The failure is reported by the logger on its console fallback; the call does not panic or hang.
	_ = logger.Info(ctx, logging.Entry{Category: "service", Function: "Place", Message: "order placed"})
}
