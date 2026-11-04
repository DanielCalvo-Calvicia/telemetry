package aws_http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// answer is what the scripted receiver says to one request: a status and, when not empty, a Retry-After header.
type answer struct {
	status      int
	retry_after string
}

// new_scripted_server starts a local receiver that gives the answers in order (the last one repeats).
func new_scripted_server(t *testing.T, answers []answer) *httptest.Server {
	t.Helper()
	var mutex sync.Mutex
	position := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		mutex.Lock()
		next_answer := answers[min(position, len(answers)-1)]
		position++
		mutex.Unlock()
		if next_answer.retry_after != "" {
			writer.Header().Set("Retry-After", next_answer.retry_after)
		}
		writer.WriteHeader(next_answer.status)
	}))
	t.Cleanup(server.Close)
	return server
}

func Test_the_retry_after_is_the_one_of_the_last_event_that_was_not_accepted(t *testing.T) {
	cases := []struct {
		name     string
		answers  []answer
		expected dtos.SendResult
	}{
		{"one transient failure with a hint", []answer{{429, "3"}, {200, ""}, {200, ""}},
			dtos.SendResult{Accepted: 2, Failed: 1, Reason: "status_429", RetryAfterMS: 3000}},
		{"the last failure wins", []answer{{429, "3"}, {503, ""}, {429, "7"}},
			dtos.SendResult{Failed: 3, Reason: "status_429", RetryAfterMS: 7000}},
		{"a later failure with no hint clears the hint", []answer{{429, "4"}, {200, ""}, {503, ""}},
			dtos.SendResult{Accepted: 1, Failed: 2, Reason: "status_429"}},
		{"a failure that is not the last still counts when the next is accepted", []answer{{503, "9"}, {200, ""}, {200, ""}},
			dtos.SendResult{Accepted: 2, Failed: 1, Reason: "status_503", RetryAfterMS: 9000}},
		{"a permanent rejection last clears the hint", []answer{{503, "5"}, {400, ""}, {200, ""}},
			dtos.SendResult{Accepted: 1, Failed: 1, Dropped: 1, Reason: "status_503"}},
		{"no failure, no hint", []answer{{200, "5"}},
			dtos.SendResult{Accepted: 3}},
		{"a header that is not valid is no hint", []answer{{503, "soon"}},
			dtos.SendResult{Failed: 3, Reason: "status_503"}},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			server := new_scripted_server(t, test_case.answers)
			destination := new_destination(t, settings_for_server(t, server.URL))

			result := destination.Send(context.Background(), make_log_events(3, value_objects.SeverityError))

			if result != test_case.expected {
				t.Fatalf("got %+v, expected %+v", result, test_case.expected)
			}
		})
	}
}
