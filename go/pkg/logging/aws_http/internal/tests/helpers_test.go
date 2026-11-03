package aws_http_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http"
)

// secret_looking_text is text that looks like a secret and is not one: no result and no error may ever hold it.
const secret_looking_text = "placeholder-token"

// endpoint_path is the path of the add_log endpoint that the tests use.
const endpoint_path = "/api/v1/cloudwatch/logs/add"

// received_request is what the local receiver saw of one request.
type received_request struct {
	Method  string
	Path    string
	Query   string
	Headers http.Header
	Body    string
}

// recording_receiver is a local HTTP receiver on 127.0.0.1 that records every request and answers the status of the
// position of the request (the last status repeats). It is not the fake receiver of the core: that one only takes the
// batch body, and add_log has its own body.
type recording_receiver struct {
	server   *httptest.Server
	statuses []int
	mutex    sync.Mutex
	requests []received_request
}

// new_receiver starts a receiver that answers the statuses in order. For a 3xx it also sets a Location to /moved, which
// must never receive a request.
func new_receiver(t *testing.T, statuses ...int) *recording_receiver {
	t.Helper()
	receiver := &recording_receiver{statuses: statuses}
	receiver.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		receiver.mutex.Lock()
		position := len(receiver.requests)
		receiver.requests = append(receiver.requests, received_request{
			Method: request.Method, Path: request.URL.Path, Query: request.URL.RawQuery, Headers: request.Header.Clone(), Body: string(body),
		})
		receiver.mutex.Unlock()
		status := receiver.statuses[len(receiver.statuses)-1]
		if position < len(receiver.statuses) {
			status = receiver.statuses[position]
		}
		if status >= 300 && status < 400 {
			writer.Header().Set("Location", "/moved")
		}
		writer.WriteHeader(status)
	}))
	t.Cleanup(receiver.server.Close)
	return receiver
}

func (receiver *recording_receiver) received() []received_request {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()
	return append([]received_request(nil), receiver.requests...)
}

// settings_for_server returns the default settings of the adapter pointed at the receiver, with the group and the stream
// of the shared fixtures.
func settings_for_server(t *testing.T, server_url string) aws_http.Settings {
	t.Helper()
	parsed_url, parse_error := url.Parse(server_url)
	if parse_error != nil {
		t.Fatal(parse_error)
	}
	settings := aws_http.DefaultSettings()
	settings.Host = parsed_url.Scheme + "://" + parsed_url.Hostname()
	settings.Port = parsed_url.Port()
	settings.Path = endpoint_path
	settings.LogGroupName = "my-log-group"
	settings.LogStreamName = "my-log-stream"
	return settings
}

func new_destination(t *testing.T, settings aws_http.Settings) *aws_http.Destination {
	t.Helper()
	destination, build_error := aws_http.NewDestination(settings)
	if build_error != nil {
		t.Fatal(build_error)
	}
	return destination
}

// closed_server_url returns the URL of a local server that was started and closed: nothing answers there.
func closed_server_url() string {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server_url := server.URL
	server.Close()
	return server_url
}

// make_log_events returns n valid log.entry.recorded events of the severity.
func make_log_events(count int, severity value_objects.Severity) []value_objects.Envelope {
	events := make([]value_objects.Envelope, 0, count)
	for position := 0; position < count; position++ {
		events = append(events, value_objects.Envelope{
			SchemaVersion: "0.1",
			EventId:       "01900000-0000-7000-8000-000000000001",
			EventType:     "log.entry.recorded",
			OccurredAt:    "2026-10-28T10:00:00.123456Z",
			RecordedAt:    "2026-10-28T10:00:00.123456Z",
			Resource:      value_objects.Resource{ServiceName: "sensor-hub", InstanceId: "hub-01"},
			Severity:      severity,
			Data:          map[string]any{"category": "http", "message": "hello"},
		})
	}
	return events
}

// make_metric_event returns one event that is not a log event.
func make_metric_event() value_objects.Envelope {
	event := make_log_events(1, value_objects.SeverityInfo)[0]
	event.EventType = "metric.counter.incremented"
	event.Data = map[string]any{"name": "x", "value": 1}
	return event
}
