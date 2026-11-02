package http_destination_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	http_destination "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http"
)

// secret_looking_text is text that looks like a secret and is not one: no result and no error may ever hold it.
const secret_looking_text = "placeholder-secret-not-real"

// received_request is what the local receiver saw of one request, with the body already decompressed.
type received_request struct {
	Method  string
	Path    string
	Query   string
	Headers http.Header
	Body    string
}

// recording_receiver is a local HTTP receiver on 127.0.0.1 that records every request and answers a status.
type recording_receiver struct {
	server   *httptest.Server
	mutex    sync.Mutex
	requests []received_request
}

// new_receiver starts a receiver that answers the status. For a 3xx it also sets a Location to /moved, which must never
// receive a request.
func new_receiver(t *testing.T, status int) *recording_receiver {
	t.Helper()
	receiver := &recording_receiver{}
	receiver.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var reader io.Reader = request.Body
		if request.Header.Get("Content-Encoding") == "gzip" {
			gzip_reader, gzip_error := gzip.NewReader(request.Body)
			if gzip_error != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			reader = gzip_reader
		}
		body, _ := io.ReadAll(reader)
		receiver.mutex.Lock()
		receiver.requests = append(receiver.requests, received_request{
			Method: request.Method, Path: request.URL.Path, Query: request.URL.RawQuery, Headers: request.Header.Clone(), Body: string(body),
		})
		receiver.mutex.Unlock()
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

// settings_for returns the default settings of the destination pointed at a URL.
func settings_for(url string) http_destination.Settings {
	settings := http_destination.DefaultSettings()
	settings.URL = url
	return settings
}

func new_destination(t *testing.T, settings http_destination.Settings) *http_destination.HTTPDestination {
	t.Helper()
	destination, build_error := http_destination.NewHTTPDestination(settings, value_objects.DefaultLimits())
	if build_error != nil {
		t.Fatal(build_error)
	}
	return destination
}

// make_events returns n valid envelopes of the severity.
func make_events(count int, severity value_objects.Severity) []value_objects.Envelope {
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
