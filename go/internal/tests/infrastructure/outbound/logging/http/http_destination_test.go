package http_destination_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	http_destination "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http/helpers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
	client_helpers "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
)

// closed_server_url returns the URL of a local server that was started and closed: nothing answers there.
func closed_server_url() string {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()
	return url
}

func Test_result_from_outcome_table(t *testing.T) {
	success := http_client.Outcome{Class: client_helpers.ClassSuccess, StatusCode: 200}
	transient := http_client.Outcome{Class: client_helpers.ClassTransient, StatusCode: 503, Reason: "status_503"}
	permanent := http_client.Outcome{Class: client_helpers.ClassPermanent, StatusCode: 400, Reason: "status_400"}
	cases := []struct {
		name     string
		outcome  http_client.Outcome
		sent     int
		filtered int
		expected dtos.SendResult
	}{
		{"success", success, 3, 0, dtos.SendResult{Accepted: 3}},
		{"success with filtered", success, 3, 2, dtos.SendResult{Accepted: 5}},
		{"transient", transient, 3, 0, dtos.SendResult{Failed: 3, Reason: "status_503"}},
		{"transient with filtered", transient, 3, 2, dtos.SendResult{Accepted: 2, Failed: 3, Reason: "status_503"}},
		{"permanent", permanent, 3, 0, dtos.SendResult{Dropped: 3, Reason: "status_400"}},
		{"permanent with filtered", permanent, 3, 2, dtos.SendResult{Accepted: 2, Dropped: 3, Reason: "status_400"}},
		{"zero sent only filtered", http_client.Outcome{}, 0, 4, dtos.SendResult{Accepted: 4}},
		{"zero sent and zero filtered", http_client.Outcome{}, 0, 0, dtos.SendResult{}},
		{"an outcome with no class is a failure", http_client.Outcome{Reason: "network_error"}, 1, 0, dtos.SendResult{Failed: 1, Reason: "network_error"}},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			if got := helpers.ResultFromOutcome(test_case.outcome, test_case.sent, test_case.filtered); got != test_case.expected {
				t.Fatalf("got %+v, expected %+v", got, test_case.expected)
			}
		})
	}
}

func Test_deliver_returns_the_outcome_and_the_counts(t *testing.T) {
	receiver := new_receiver(t, http.StatusServiceUnavailable)
	destination := new_destination(t, settings_for(receiver.server.URL))
	events := append(make_events(2, value_objects.SeverityInfo), make_events(3, value_objects.SeverityDebug)...)

	delivery := destination.Deliver(context.Background(), events)

	if delivery.Sent != 2 || delivery.Filtered != 3 {
		t.Fatalf("sent %d filtered %d", delivery.Sent, delivery.Filtered)
	}
	if delivery.Outcome.Class != client_helpers.ClassTransient || delivery.Outcome.StatusCode != 503 || delivery.Outcome.Reason != "status_503" {
		t.Fatalf("outcome %+v", delivery.Outcome)
	}
	if delivery.Result != (dtos.SendResult{Accepted: 3, Failed: 2, Reason: "status_503"}) {
		t.Fatalf("result %+v", delivery.Result)
	}
	if len(receiver.received()) != 1 {
		t.Fatalf("exactly one request expected, got %d", len(receiver.received()))
	}
}

func Test_a_delivery_with_nothing_to_send_makes_no_request_and_has_a_zero_outcome(t *testing.T) {
	receiver := new_receiver(t, http.StatusOK)
	destination := new_destination(t, settings_for(receiver.server.URL))
	delivery := destination.Deliver(context.Background(), make_events(2, value_objects.SeverityTrace))
	if delivery.Outcome != (http_client.Outcome{}) || delivery.Sent != 0 || delivery.Filtered != 2 || delivery.Result.Accepted != 2 {
		t.Fatalf("%+v", delivery)
	}
	if len(receiver.received()) != 0 {
		t.Fatal("no request expected")
	}
}

func Test_a_server_that_is_closed_fails_the_events_with_network_error(t *testing.T) {
	destination := new_destination(t, settings_for(closed_server_url()))
	result := destination.Send(context.Background(), make_events(3, value_objects.SeverityError))
	if result != (dtos.SendResult{Failed: 3, Reason: "network_error"}) {
		t.Fatalf("%+v", result)
	}
}

func Test_a_slow_server_fails_the_events_with_timeout_in_about_the_timeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()
	defer close(release)
	settings := settings_for(server.URL)
	settings.TimeoutMS = 200
	destination := new_destination(t, settings)

	started := time.Now()
	result := destination.Send(context.Background(), make_events(2, value_objects.SeverityError))

	if result != (dtos.SendResult{Failed: 2, Reason: "timeout"}) {
		t.Fatalf("%+v", result)
	}
	if elapsed := time.Since(started); elapsed >= 3*time.Second {
		t.Fatalf("it took %s, the timeout is 200 ms", elapsed)
	}
}

func Test_a_batch_over_the_limit_is_dropped_and_the_server_gets_no_request(t *testing.T) {
	receiver := new_receiver(t, http.StatusOK)
	tiny_limits, limits_error := value_objects.NewLimits(32, 64, 256, 100, 200)
	if limits_error != nil {
		t.Fatal(limits_error)
	}
	destination, build_error := http_destination.NewHTTPDestination(settings_for(receiver.server.URL), tiny_limits)
	if build_error != nil {
		t.Fatal(build_error)
	}
	result := destination.Send(context.Background(), make_events(4, value_objects.SeverityError))
	if result != (dtos.SendResult{Dropped: 4, Reason: "batch_too_large"}) {
		t.Fatalf("%+v", result)
	}
	if len(receiver.received()) != 0 {
		t.Fatal("a batch over the limit must not be sent")
	}
}

func Test_no_result_and_no_error_holds_a_secret_or_the_query_of_the_url(t *testing.T) {
	events := make_events(2, value_objects.SeverityError)
	check := func(label string, texts ...string) {
		for _, text := range texts {
			if strings.Contains(text, secret_looking_text) || strings.Contains(text, "token=") {
				t.Errorf("%s holds a secret or the query: %s", label, text)
			}
		}
	}

	// A bearer token and a query with the same text, on every failing path.
	for _, status := range []int{400, 429, 503} {
		receiver := new_receiver(t, status)
		settings := settings_for(receiver.server.URL + "/ingest?token=" + secret_looking_text)
		settings.BearerToken = secret_looking_text
		delivery := new_destination(t, settings).Deliver(context.Background(), events)
		check("a status result", delivery.Result.Reason, strings.TrimSpace(strings.Join([]string{
			delivery.Outcome.Reason, string(delivery.Outcome.Class)}, " ")))
		if got := receiver.received(); len(got) != 1 || got[0].Query != "token="+secret_looking_text {
			t.Errorf("the query of the configured URL must be sent as configured: %+v", got)
		}
	}

	// An API key over a closed server and over a timeout.
	api_settings := settings_for(closed_server_url() + "/ingest?token=" + secret_looking_text)
	api_settings.APIKeyHeader = "X-Api-Key"
	api_settings.APIKeyValue = secret_looking_text
	delivery := new_destination(t, api_settings).Deliver(context.Background(), events)
	check("a network failure", delivery.Result.Reason, delivery.Outcome.Reason)

	// The refusal of the settings and the nil destination.
	invalid_settings := settings_for("ftp://logs.example.invalid/?token=" + secret_looking_text)
	invalid_settings.BearerToken = secret_looking_text
	invalid_settings.MinimumSeverity = secret_looking_text
	_, build_error := http_destination.NewHTTPDestination(invalid_settings, value_objects.DefaultLimits())
	if build_error == nil {
		t.Fatal("the settings are invalid")
	}
	check("the validation error", build_error.Error())
	var typed_error *config.ValidationError
	if !errors.As(build_error, &typed_error) {
		t.Fatalf("%T", build_error)
	}
	for _, problem := range typed_error.Problems {
		check("a problem", problem.Field, problem.Reason, string(problem.Code))
	}
	var nil_destination *http_destination.HTTPDestination
	nil_delivery := nil_destination.Deliver(context.Background(), events)
	check("the nil destination", nil_delivery.Result.Reason, adapter_error.MapDestinationError(nil).Error())
}

func Test_sixteen_goroutines_can_send_on_one_destination(t *testing.T) {
	receiver := new_receiver(t, http.StatusOK)
	destination := new_destination(t, settings_for(receiver.server.URL))
	var wait_group sync.WaitGroup
	results := make([]dtos.SendResult, 16)
	for position := range results {
		wait_group.Add(1)
		go func() {
			defer wait_group.Done()
			results[position] = destination.Send(context.Background(), make_events(2, value_objects.SeverityWarn))
		}()
	}
	wait_group.Wait()
	for position, result := range results {
		if result != (dtos.SendResult{Accepted: 2}) {
			t.Errorf("goroutine %d: %+v", position, result)
		}
	}
	if len(receiver.received()) != 16 {
		t.Fatalf("expected 16 requests, got %d", len(receiver.received()))
	}
}

func Test_a_nil_destination_fails_every_event_without_panicking(t *testing.T) {
	var destination *http_destination.HTTPDestination
	result := destination.Send(context.Background(), make_events(3, value_objects.SeverityError))
	if result != (dtos.SendResult{Failed: 3, Reason: "destination_failed"}) {
		t.Fatalf("%+v", result)
	}
	if delay_ms, retry := destination.NextDelay(0, http_client.Outcome{Class: client_helpers.ClassTransient}, 0.5); delay_ms != 0 || retry {
		t.Fatalf("a nil destination never retries: %d %v", delay_ms, retry)
	}
}

// panicking_value is a value whose JSON form panics: it lets the test make the serialization panic.
type panicking_value struct{}

func (panicking_value) MarshalJSON() ([]byte, error) {
	panic("the value does not want to be serialized")
}

func Test_a_panic_while_building_the_body_is_recovered_as_destination_failed(t *testing.T) {
	receiver := new_receiver(t, http.StatusOK)
	destination := new_destination(t, settings_for(receiver.server.URL))
	events := make_events(3, value_objects.SeverityError)
	events[1].Data = map[string]any{"boom": panicking_value{}}

	delivery := destination.Deliver(context.Background(), events)

	// The panic comes from the serialization inside the shared client call of the destination: the destination recovers
	// it and every event it had not counted fails with destination_failed.
	if delivery.Result != (dtos.SendResult{Failed: 3, Reason: "destination_failed"}) {
		t.Fatalf("%+v", delivery.Result)
	}
	if len(receiver.received()) != 0 {
		t.Fatal("nothing must be sent")
	}
}

func Test_an_event_that_cannot_be_serialized_is_a_permanent_drop(t *testing.T) {
	receiver := new_receiver(t, http.StatusOK)
	destination := new_destination(t, settings_for(receiver.server.URL))
	events := make_events(2, value_objects.SeverityError)
	events[0].Data = map[string]any{"channel": make(chan int)}
	result := destination.Send(context.Background(), events)
	if result != (dtos.SendResult{Dropped: 2, Reason: "serialization_failed"}) {
		t.Fatalf("%+v", result)
	}
}

func Test_next_delay_delegates_to_the_shared_client(t *testing.T) {
	settings := settings_for("http://127.0.0.1:1")
	settings.MaxAttempts = 3
	destination := new_destination(t, settings)
	client, client_error := http_client.NewClient(settings.ClientSettings(), value_objects.DefaultLimits())
	if client_error != nil {
		t.Fatal(client_error)
	}
	outcomes := []http_client.Outcome{
		{Class: client_helpers.ClassTransient, Reason: "status_503"},
		{Class: client_helpers.ClassTransient, StatusCode: 429, RetryAfterMilliseconds: 7000, RetryAfterOK: true, Reason: "status_429"},
		{Class: client_helpers.ClassPermanent, Reason: "status_400"},
		{Class: client_helpers.ClassSuccess},
	}
	for _, outcome := range outcomes {
		for attempt := 0; attempt < 4; attempt++ {
			for _, random := range []float64{0, 0.5, 1} {
				expected_delay, expected_retry := client.NextDelay(attempt, outcome, random)
				delay, retry := destination.NextDelay(attempt, outcome, random)
				if delay != expected_delay || retry != expected_retry {
					t.Errorf("attempt %d %+v random %v: %d %v, expected %d %v", attempt, outcome, random, delay, retry, expected_delay, expected_retry)
				}
			}
		}
	}
	if _, retry := destination.NextDelay(0, outcomes[0], 0.5); !retry {
		t.Error("a transient outcome on the first attempt must retry")
	}
	if _, retry := destination.NextDelay(2, outcomes[0], 0.5); retry {
		t.Error("the last attempt must not retry")
	}
}

func Test_default_settings_and_the_client_settings(t *testing.T) {
	defaults := http_destination.DefaultSettings()
	if defaults.MinimumSeverity != "INFO" || defaults.URL != "" || defaults.Method != "POST" {
		t.Fatalf("%+v", defaults)
	}
	if !reflect.DeepEqual(defaults.ClientSettings(), http_client.DefaultSettings()) {
		t.Fatalf("the client part of the defaults must be the defaults of the client: %+v", defaults.ClientSettings())
	}
}

func Test_a_problem_of_the_client_comes_before_the_one_of_the_severity(t *testing.T) {
	settings := http_destination.DefaultSettings()
	settings.MinimumSeverity = "LOUD"
	settings.Method = "GET"
	validation_error := http_destination.ValidateSettings(settings)
	var typed_error *config.ValidationError
	if !errors.As(validation_error, &typed_error) || len(typed_error.Problems) != 3 {
		t.Fatalf("%v", validation_error)
	}
	fields := []string{typed_error.Problems[0].Field, typed_error.Problems[1].Field, typed_error.Problems[2].Field}
	if !reflect.DeepEqual(fields, []string{"url", "method", "minimum_severity"}) {
		t.Fatalf("%v", fields)
	}
	for _, problem := range typed_error.Problems {
		if problem.Code != value_objects.ErrorCodeConfigFieldInvalid {
			t.Errorf("%s: %s", problem.Field, problem.Code)
		}
	}
}

func Test_the_destination_error_joins_the_sentinel_and_the_cause(t *testing.T) {
	cause := errors.New("the cause")
	mapped := adapter_error.MapDestinationError(cause)
	if !errors.Is(mapped, adapter_error.ErrDestinationUnusable) || !errors.Is(mapped, cause) {
		t.Fatalf("%v", mapped)
	}
}

func Test_the_send_method_serves_both_ports(t *testing.T) {
	// Send is the same call for the log port and the event port: the compile-time assertions of the adapter hold, and
	// the call answers like Deliver.
	receiver := new_receiver(t, http.StatusOK)
	destination := new_destination(t, settings_for(receiver.server.URL))
	result := destination.Send(context.Background(), nil)
	if result != (dtos.SendResult{}) {
		t.Fatalf("%+v", result)
	}
}
