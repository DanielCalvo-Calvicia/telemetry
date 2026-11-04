package http_destination_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// new_receiver_with_header starts a local receiver that answers the status with one header.
func new_receiver_with_header(t *testing.T, status int, header_name string, header_value string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if header_name != "" {
			writer.Header().Set(header_name, header_value)
		}
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server
}

func Test_the_retry_after_of_a_transient_answer_reaches_the_result(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		retry_after string
		expected    dtos.SendResult
	}{
		{"429 with seconds", http.StatusTooManyRequests, "2", dtos.SendResult{Failed: 2, Reason: "status_429", RetryAfterMS: 2000}},
		{"503 with seconds", http.StatusServiceUnavailable, "1", dtos.SendResult{Failed: 2, Reason: "status_503", RetryAfterMS: 1000}},
		{"429 without the header", http.StatusTooManyRequests, "", dtos.SendResult{Failed: 2, Reason: "status_429"}},
		{"503 with a header that is not valid", http.StatusServiceUnavailable, "soon", dtos.SendResult{Failed: 2, Reason: "status_503"}},
		{"a permanent rejection with the header", http.StatusBadRequest, "5", dtos.SendResult{Dropped: 2, Reason: "status_400"}},
		{"a success with the header", http.StatusOK, "5", dtos.SendResult{Accepted: 2}},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			server := new_receiver_with_header(t, test_case.status, "Retry-After", test_case.retry_after)
			destination := new_destination(t, settings_for(server.URL))

			result := destination.Send(context.Background(), make_events(2, value_objects.SeverityError))

			if result != test_case.expected {
				t.Fatalf("got %+v, expected %+v", result, test_case.expected)
			}
		})
	}
}
