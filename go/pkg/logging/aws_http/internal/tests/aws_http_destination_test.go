package aws_http_test

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
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
	client_helpers "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http/error"
)

func Test_the_destination_satisfies_both_ports_of_the_core(t *testing.T) {
	receiver := new_receiver(t, 200)
	destination := new_destination(t, settings_for_server(t, receiver.server.URL))
	var log_port outbound.LogSinkPort = destination
	var event_port outbound.EventSinkPort = destination
	if result := log_port.Send(context.Background(), nil); result != (dtos.SendResult{}) {
		t.Fatalf("%+v", result)
	}
	if result := event_port.Send(context.Background(), make_log_events(1, value_objects.SeverityInfo)); result != (dtos.SendResult{Accepted: 1}) {
		t.Fatalf("%+v", result)
	}
}

func Test_each_status_gives_its_class_and_reason(t *testing.T) {
	cases := []struct {
		status   int
		expected dtos.SendResult
	}{
		{200, dtos.SendResult{Accepted: 2}},
		{204, dtos.SendResult{Accepted: 2}},
		{503, dtos.SendResult{Failed: 2, Reason: "status_503"}},
		{429, dtos.SendResult{Failed: 2, Reason: "status_429"}},
		{408, dtos.SendResult{Failed: 2, Reason: "status_408"}},
		{400, dtos.SendResult{Dropped: 2, Reason: "status_400"}},
		{302, dtos.SendResult{Dropped: 2, Reason: "status_302"}},
	}
	for _, test_case := range cases {
		t.Run(http.StatusText(test_case.status), func(t *testing.T) {
			receiver := new_receiver(t, test_case.status)
			destination := new_destination(t, settings_for_server(t, receiver.server.URL))

			delivery := destination.Deliver(context.Background(), make_log_events(2, value_objects.SeverityError))

			if delivery.Result != test_case.expected {
				t.Fatalf("%+v, expected %+v", delivery.Result, test_case.expected)
			}
			if delivery.Outcome.StatusCode != test_case.status || delivery.Sent != 2 || delivery.Skipped != 0 {
				t.Fatalf("%+v", delivery)
			}
			requests := receiver.received()
			if len(requests) != 2 {
				t.Fatalf("one request per event expected, got %d", len(requests))
			}
			for _, request := range requests {
				if request.Path == "/moved" {
					t.Errorf("a redirect was followed")
				}
			}
		})
	}
}

func Test_a_server_that_is_closed_fails_the_events_with_network_error(t *testing.T) {
	settings := settings_for_server(t, closed_server_url())
	destination := new_destination(t, settings)
	delivery := destination.Deliver(context.Background(), make_log_events(3, value_objects.SeverityError))
	if delivery.Result != (dtos.SendResult{Failed: 3, Reason: "network_error"}) {
		t.Fatalf("%+v", delivery.Result)
	}
	if delivery.Outcome.Class != client_helpers.ClassTransient || delivery.Outcome.Reason != "network_error" {
		t.Fatalf("%+v", delivery.Outcome)
	}
}

func Test_a_slow_server_fails_the_event_with_timeout_in_about_the_timeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()
	defer close(release)
	settings := settings_for_server(t, server.URL)
	settings.TimeoutMS = 200
	destination := new_destination(t, settings)

	started := time.Now()
	result := destination.Send(context.Background(), make_log_events(1, value_objects.SeverityError))

	if result != (dtos.SendResult{Failed: 1, Reason: "timeout"}) {
		t.Fatalf("%+v", result)
	}
	if elapsed := time.Since(started); elapsed >= 3*time.Second {
		t.Fatalf("it took %s, the timeout is 200 ms", elapsed)
	}
}

func Test_a_mixed_batch_counts_each_event_once_and_keeps_the_first_reason(t *testing.T) {
	// Five events: log, metric, log, log, log. The four requests answer 200, 503, 400, 200.
	receiver := new_receiver(t, 200, 503, 400, 200)
	destination := new_destination(t, settings_for_server(t, receiver.server.URL))
	events := make_log_events(4, value_objects.SeverityWarn)
	events = append(events[:1], append([]value_objects.Envelope{make_metric_event()}, events[1:]...)...)

	delivery := destination.Deliver(context.Background(), events)

	if delivery.Result != (dtos.SendResult{Accepted: 3, Dropped: 1, Failed: 1, Reason: "status_503"}) {
		t.Fatalf("%+v", delivery.Result)
	}
	if delivery.Sent != 4 || delivery.Skipped != 1 {
		t.Fatalf("%+v", delivery)
	}
	if delivery.Outcome.Class != client_helpers.ClassSuccess {
		t.Fatalf("the outcome is the one of the last request: %+v", delivery.Outcome)
	}
	if len(receiver.received()) != 4 {
		t.Fatalf("the metric must not be sent: %d requests", len(receiver.received()))
	}
}

func Test_the_reason_is_the_one_of_the_first_event_that_was_not_accepted(t *testing.T) {
	receiver := new_receiver(t, 400, 503)
	destination := new_destination(t, settings_for_server(t, receiver.server.URL))
	result := destination.Send(context.Background(), make_log_events(2, value_objects.SeverityInfo))
	if result != (dtos.SendResult{Dropped: 1, Failed: 1, Reason: "status_400"}) {
		t.Fatalf("%+v", result)
	}
}

func Test_only_events_that_are_not_log_events_make_no_request_and_a_zero_outcome(t *testing.T) {
	receiver := new_receiver(t, 200)
	destination := new_destination(t, settings_for_server(t, receiver.server.URL))
	delivery := destination.Deliver(context.Background(), []value_objects.Envelope{make_metric_event(), make_metric_event()})
	if delivery.Outcome != (http_client.Outcome{}) || delivery.Sent != 0 || delivery.Skipped != 2 || delivery.Result != (dtos.SendResult{Accepted: 2}) {
		t.Fatalf("%+v", delivery)
	}
	if len(receiver.received()) != 0 {
		t.Fatal("no request expected")
	}
	if empty_delivery := destination.Deliver(context.Background(), nil); empty_delivery != (aws_http.Delivery{}) {
		t.Fatalf("%+v", empty_delivery)
	}
}

func Test_an_event_that_cannot_be_translated_is_dropped_with_no_request(t *testing.T) {
	receiver := new_receiver(t, 200)
	destination := new_destination(t, settings_for_server(t, receiver.server.URL))
	bad_time := make_log_events(1, value_objects.SeverityInfo)[0]
	bad_time.OccurredAt = "yesterday"
	bad_text := make_log_events(1, value_objects.SeverityInfo)[0]
	bad_text.Data = map[string]any{"message": "broken \xff text"}
	bad_attributes := make_log_events(1, value_objects.SeverityInfo)[0]
	bad_attributes.Attributes = value_objects.Attributes{"channel": make(chan int)}
	good := make_log_events(1, value_objects.SeverityInfo)[0]

	delivery := destination.Deliver(context.Background(), []value_objects.Envelope{bad_time, bad_text, bad_attributes, good})

	if delivery.Result != (dtos.SendResult{Accepted: 1, Dropped: 3, Reason: "serialization_failed"}) {
		t.Fatalf("%+v", delivery.Result)
	}
	if delivery.Sent != 1 || len(receiver.received()) != 1 {
		t.Fatalf("only the good event is sent: %+v", delivery)
	}
}

func Test_a_request_over_the_limit_is_dropped_and_the_server_gets_no_request(t *testing.T) {
	receiver := new_receiver(t, 200)
	tiny_limits, limits_error := value_objects.NewLimits(32, 64, 256, 100, 200)
	if limits_error != nil {
		t.Fatal(limits_error)
	}
	destination, build_error := aws_http.NewDestinationWithLimits(settings_for_server(t, receiver.server.URL), tiny_limits)
	if build_error != nil {
		t.Fatal(build_error)
	}
	delivery := destination.Deliver(context.Background(), make_log_events(2, value_objects.SeverityError))
	if delivery.Result != (dtos.SendResult{Dropped: 2, Reason: "batch_too_large"}) {
		t.Fatalf("%+v", delivery.Result)
	}
	if len(receiver.received()) != 0 {
		t.Fatal("a request over the limit must not be sent")
	}
}

func Test_the_bearer_token_and_the_api_key_travel_in_the_header_and_nowhere_else(t *testing.T) {
	receiver := new_receiver(t, 200)
	bearer_settings := settings_for_server(t, receiver.server.URL)
	bearer_settings.BearerToken = secret_looking_text
	new_destination(t, bearer_settings).Send(context.Background(), make_log_events(1, value_objects.SeverityInfo))

	key_settings := settings_for_server(t, receiver.server.URL)
	key_settings.APIKeyHeader = "X-Api-Key"
	key_settings.APIKeyValue = secret_looking_text
	new_destination(t, key_settings).Send(context.Background(), make_log_events(1, value_objects.SeverityInfo))

	requests := receiver.received()
	if len(requests) != 2 {
		t.Fatalf("%d requests", len(requests))
	}
	if requests[0].Headers.Get("Authorization") != "Bearer "+secret_looking_text {
		t.Errorf("the bearer token is missing: %q", requests[0].Headers.Get("Authorization"))
	}
	if requests[1].Headers.Get("X-Api-Key") != secret_looking_text {
		t.Errorf("the API key is missing: %q", requests[1].Headers.Get("X-Api-Key"))
	}
	for _, request := range requests {
		if strings.Contains(request.Body, secret_looking_text) {
			t.Errorf("a secret is in the body: %s", request.Body)
		}
	}
}

func Test_no_result_and_no_error_holds_a_secret(t *testing.T) {
	events := make_log_events(2, value_objects.SeverityError)
	check := func(label string, texts ...string) {
		for _, text := range texts {
			if strings.Contains(text, secret_looking_text) {
				t.Errorf("%s holds a secret: %s", label, text)
			}
		}
	}

	for _, status := range []int{400, 429, 503} {
		receiver := new_receiver(t, status)
		settings := settings_for_server(t, receiver.server.URL)
		settings.BearerToken = secret_looking_text
		delivery := new_destination(t, settings).Deliver(context.Background(), events)
		check("a status result", delivery.Result.Reason, delivery.Outcome.Reason, string(delivery.Outcome.Class))
	}

	api_settings := settings_for_server(t, closed_server_url())
	api_settings.APIKeyHeader = "X-Api-Key"
	api_settings.APIKeyValue = secret_looking_text
	delivery := new_destination(t, api_settings).Deliver(context.Background(), events)
	check("a network failure", delivery.Result.Reason, delivery.Outcome.Reason)

	// Every field invalid, the two secrets set (and in the wrong place).
	invalid_settings := aws_http.Settings{
		Host: "ftp://" + secret_looking_text, Port: secret_looking_text, Path: secret_looking_text,
		BearerToken: secret_looking_text, APIKeyHeader: "bad header", APIKeyValue: secret_looking_text + " x",
	}
	_, build_error := aws_http.NewDestination(invalid_settings)
	if build_error == nil {
		t.Fatal("the settings are invalid")
	}
	check("the validation error", build_error.Error())
	var typed_error *config.ValidationError
	if !errors.As(build_error, &typed_error) {
		t.Fatalf("%T", build_error)
	}
	for _, problem := range typed_error.Problems {
		check("a problem", problem.Field, problem.Reason, problem.Code)
	}

	var nil_destination *aws_http.Destination
	nil_delivery := nil_destination.Deliver(context.Background(), events)
	check("the nil destination", nil_delivery.Result.Reason, adapter_error.MapDestinationError(nil).Error())
}

func Test_authentication_over_plain_http_to_a_host_that_is_not_local_is_a_problem_of_the_host(t *testing.T) {
	settings := aws_http.DefaultSettings()
	settings.Host = "http://logs.example.invalid"
	settings.Port = "80"
	settings.Path = endpoint_path
	settings.LogGroupName = "my-log-group"
	settings.LogStreamName = "my-log-stream"
	settings.BearerToken = secret_looking_text
	var typed_error *config.ValidationError
	if !errors.As(aws_http.ValidateSettings(settings), &typed_error) || len(typed_error.Problems) != 1 || typed_error.Problems[0].Field != "host" {
		t.Fatalf("%v", aws_http.ValidateSettings(settings))
	}
	settings.Host = "https://logs.example.invalid"
	if validation_error := aws_http.ValidateSettings(settings); validation_error != nil {
		t.Fatalf("https allows authentication: %v", validation_error)
	}
}

func Test_the_problems_of_the_five_fields_come_before_the_ones_of_the_client_in_the_order_of_the_schema(t *testing.T) {
	settings := aws_http.Settings{
		Host: "localhost", Port: "0", Path: "x", TimeoutMS: 0, BearerToken: "has space", MaxAttempts: 0, RetryBaseDelayMS: 5, RetryMaxDelayMS: 1,
	}
	var typed_error *config.ValidationError
	if !errors.As(aws_http.ValidateSettings(settings), &typed_error) {
		t.Fatal("expected a *config.ValidationError")
	}
	fields := []string{}
	for _, problem := range typed_error.Problems {
		fields = append(fields, problem.Field)
		if problem.Code != value_objects.ErrorCodeConfigFieldInvalid {
			t.Errorf("%s: %s", problem.Field, problem.Code)
		}
	}
	expected := []string{"host", "port", "path", "log_group_name", "log_stream_name", "timeout_ms", "bearer_token", "max_attempts", "retry_max_delay_ms"}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatalf("%v, expected %v", fields, expected)
	}
}

func Test_host_port_and_path_rules(t *testing.T) {
	valid := aws_http.DefaultSettings()
	valid.Host, valid.Port, valid.Path = "http://localhost", "8080", "/p"
	valid.LogGroupName, valid.LogStreamName = "my-log-group", "my-log-stream"
	cases := []struct {
		name   string
		change func(settings *aws_http.Settings)
		field  string
	}{
		{"the valid settings", func(*aws_http.Settings) {}, ""},
		{"https", func(settings *aws_http.Settings) { settings.Host = "https://logs.example.invalid" }, ""},
		{"an IPv6 host", func(settings *aws_http.Settings) { settings.Host = "http://[::1]" }, ""},
		{"port 1", func(settings *aws_http.Settings) { settings.Port = "1" }, ""},
		{"port 65535", func(settings *aws_http.Settings) { settings.Port = "65535" }, ""},
		{"an empty host", func(settings *aws_http.Settings) { settings.Host = "" }, "host"},
		{"only the scheme", func(settings *aws_http.Settings) { settings.Host = "http://" }, "host"},
		{"another scheme", func(settings *aws_http.Settings) { settings.Host = "ftp://localhost" }, "host"},
		{"a path in the host", func(settings *aws_http.Settings) { settings.Host = "http://localhost/api" }, "host"},
		{"a query in the host", func(settings *aws_http.Settings) { settings.Host = "http://localhost?a=b" }, "host"},
		{"a fragment in the host", func(settings *aws_http.Settings) { settings.Host = "http://localhost#a" }, "host"},
		{"a space in the host", func(settings *aws_http.Settings) { settings.Host = "http://local host" }, "host"},
		{"a port of zero", func(settings *aws_http.Settings) { settings.Port = "0" }, "port"},
		{"a port above the range", func(settings *aws_http.Settings) { settings.Port = "65536" }, "port"},
		{"a negative port", func(settings *aws_http.Settings) { settings.Port = "-1" }, "port"},
		{"a port with a sign", func(settings *aws_http.Settings) { settings.Port = "+80" }, "port"},
		{"an empty port", func(settings *aws_http.Settings) { settings.Port = "" }, "port"},
		{"a port too long for a number", func(settings *aws_http.Settings) { settings.Port = strings.Repeat("9", 40) }, "port"},
		{"an empty path", func(settings *aws_http.Settings) { settings.Path = "" }, "path"},
		{"a path without the slash", func(settings *aws_http.Settings) { settings.Path = "p" }, "path"},
		{"an empty log group", func(settings *aws_http.Settings) { settings.LogGroupName = "" }, "log_group_name"},
		{"an empty log stream", func(settings *aws_http.Settings) { settings.LogStreamName = "" }, "log_stream_name"},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			settings := valid
			test_case.change(&settings)
			validation_error := aws_http.ValidateSettings(settings)
			if test_case.field == "" {
				if validation_error != nil {
					t.Fatalf("%v", validation_error)
				}
				return
			}
			var typed_error *config.ValidationError
			if !errors.As(validation_error, &typed_error) || len(typed_error.Problems) != 1 || typed_error.Problems[0].Field != test_case.field {
				t.Fatalf("%v", validation_error)
			}
		})
	}
}

func Test_a_host_that_the_client_cannot_parse_is_a_problem_of_the_host_and_not_of_the_url(t *testing.T) {
	settings := aws_http.DefaultSettings()
	settings.Host, settings.Port, settings.Path = "http://%zz", "80", "/p"
	settings.LogGroupName, settings.LogStreamName = "my-log-group", "my-log-stream"
	var typed_error *config.ValidationError
	if !errors.As(aws_http.ValidateSettings(settings), &typed_error) || len(typed_error.Problems) != 1 || typed_error.Problems[0].Field != "host" {
		t.Fatalf("%v", aws_http.ValidateSettings(settings))
	}
}

func Test_sixteen_goroutines_can_send_on_one_destination(t *testing.T) {
	receiver := new_receiver(t, 200)
	destination := new_destination(t, settings_for_server(t, receiver.server.URL))
	var wait_group sync.WaitGroup
	results := make([]dtos.SendResult, 16)
	for position := range results {
		wait_group.Add(1)
		go func() {
			defer wait_group.Done()
			results[position] = destination.Send(context.Background(), make_log_events(2, value_objects.SeverityWarn))
		}()
	}
	wait_group.Wait()
	for position, result := range results {
		if result != (dtos.SendResult{Accepted: 2}) {
			t.Errorf("goroutine %d: %+v", position, result)
		}
	}
	if len(receiver.received()) != 32 {
		t.Fatalf("expected 32 requests, got %d", len(receiver.received()))
	}
}

func Test_a_nil_destination_fails_every_event_without_panicking(t *testing.T) {
	var destination *aws_http.Destination
	result := destination.Send(context.Background(), make_log_events(3, value_objects.SeverityError))
	if result != (dtos.SendResult{Failed: 3, Reason: "destination_failed"}) {
		t.Fatalf("%+v", result)
	}
	if delay_ms, retry := destination.NextDelay(0, http_client.Outcome{Class: client_helpers.ClassTransient}, 0.5); delay_ms != 0 || retry {
		t.Fatalf("a nil destination never retries: %d %v", delay_ms, retry)
	}
}

// panicking_value is a value whose JSON form panics: it lets the test make the translation panic.
type panicking_value struct{}

func (panicking_value) MarshalJSON() ([]byte, error) {
	panic("the value does not want to be serialized")
}

func Test_a_panic_while_translating_is_recovered_as_destination_failed(t *testing.T) {
	receiver := new_receiver(t, 200)
	destination := new_destination(t, settings_for_server(t, receiver.server.URL))
	events := make_log_events(3, value_objects.SeverityError)
	events[1].Attributes = value_objects.Attributes{"boom": panicking_value{}}

	delivery := destination.Deliver(context.Background(), events)

	// The first event was accepted before the panic; the one that panicked and the one after it fail.
	if delivery.Result != (dtos.SendResult{Accepted: 1, Failed: 2, Reason: "destination_failed"}) {
		t.Fatalf("%+v", delivery.Result)
	}
	if len(receiver.received()) != 1 {
		t.Fatalf("only the first event was sent: %d", len(receiver.received()))
	}
}

func Test_next_delay_delegates_to_the_shared_client(t *testing.T) {
	settings := settings_for_server(t, "http://127.0.0.1:1")
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
	defaults := aws_http.DefaultSettings()
	expected := aws_http.Settings{TimeoutMS: 3000, MaxAttempts: 5, RetryBaseDelayMS: 200, RetryMaxDelayMS: 30000}
	if defaults != expected {
		t.Fatalf("%+v", defaults)
	}
	defaults.Host, defaults.Port, defaults.Path = "https://logs.example.invalid", "443", "/add"
	defaults.SkipTLSVerification = true
	client_settings := defaults.ClientSettings()
	if client_settings.URL != "https://logs.example.invalid:443/add" || client_settings.Method != "POST" || client_settings.Compress ||
		!client_settings.SkipTLSVerification || client_settings.TimeoutMS != 3000 || client_settings.MaxAttempts != 5 {
		t.Fatalf("%+v", client_settings)
	}
}

func Test_a_secret_never_shows_in_the_text_of_the_settings_error(t *testing.T) {
	_, build_error := aws_http.NewDestination(aws_http.Settings{BearerToken: secret_looking_text, APIKeyHeader: "X-Key", APIKeyValue: secret_looking_text})
	if build_error == nil || strings.Contains(build_error.Error(), secret_looking_text) {
		t.Fatalf("%v", build_error)
	}
}
