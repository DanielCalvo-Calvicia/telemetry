package http_client_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
)

func Test_gzip_is_deterministic(t *testing.T) {
	first, first_error := helpers.CompressGzip([]byte("the same body"))
	second, second_error := helpers.CompressGzip([]byte("the same body"))
	if first_error != nil || second_error != nil || !bytes.Equal(first, second) {
		t.Fatal("two compressions of the same body differ")
	}
	if !bytes.Equal(first[4:8], []byte{0, 0, 0, 0}) {
		t.Fatalf("the modification time of the header is not zero: %v", first[4:8])
	}
}

func Test_a_nil_list_of_events_gives_an_empty_list(t *testing.T) {
	body, build_error := helpers.BuildBatchBody(nil)
	if build_error != nil || string(body) != `{"events":[],"schema_version":"0.1"}` {
		t.Fatalf("got %s, %v", body, build_error)
	}
}

func Test_classify_status_edges(t *testing.T) {
	expectations := map[int]helpers.Class{
		-1: helpers.ClassTransient, 0: helpers.ClassTransient, 99: helpers.ClassTransient, 100: helpers.ClassTransient,
		199: helpers.ClassTransient, 200: helpers.ClassSuccess, 299: helpers.ClassSuccess, 300: helpers.ClassPermanent,
		399: helpers.ClassPermanent, 400: helpers.ClassPermanent, 407: helpers.ClassPermanent, 408: helpers.ClassTransient,
		409: helpers.ClassPermanent, 428: helpers.ClassPermanent, 429: helpers.ClassTransient, 499: helpers.ClassPermanent,
		500: helpers.ClassTransient, 599: helpers.ClassTransient, 600: helpers.ClassTransient, 1000: helpers.ClassTransient,
	}
	for status, expected := range expectations {
		if got := helpers.ClassifyStatus(status); got != expected {
			t.Errorf("status %d: got %s, expected %s", status, got, expected)
		}
	}
}

func Test_retry_after_edges(t *testing.T) {
	now := time.Date(2026, 10, 21, 7, 27, 0, 0, time.UTC)
	cases := []struct {
		header   string
		expected int64
		ok       bool
	}{
		{"99999999999999999999999999", helpers.MaxRetryAfterSeconds * 1000, true},
		{"+5", 0, false},
		{"5 s", 0, false},
		{"   ", 0, false},
		{"Wed, 21 Oct 2026 07:27:30 GMT", 30000, true},
	}
	for _, retry_case := range cases {
		milliseconds, ok := helpers.ParseRetryAfter(retry_case.header, now)
		if ok != retry_case.ok || milliseconds != retry_case.expected {
			t.Errorf("%q: got %d %v, expected %d %v", retry_case.header, milliseconds, ok, retry_case.expected, retry_case.ok)
		}
	}
}

func Test_backoff_edges(t *testing.T) {
	cases := []struct {
		name                     string
		attempt, base_ms, max_ms int
		random                   float64
		expected                 int64
	}{
		{"random below zero is cut", 0, 200, 30000, -3, 100},
		{"random above one is cut", 0, 200, 30000, 7, 200},
		{"not a number is zero", 0, 200, 30000, nan(), 100},
		{"the ceiling is the maximum", 20, 200, 30000, 1, 30000},
		{"no overflow with a huge attempt", 100000, 200, 30000, 0, 15000},
		{"a base above the maximum is cut", 0, 5000, 1000, 1, 1000},
		{"a negative attempt is the first", -4, 200, 30000, 0, 100},
	}
	for _, backoff_case := range cases {
		if got := helpers.BackoffWithJitter(backoff_case.attempt, backoff_case.base_ms, backoff_case.max_ms, backoff_case.random); got != backoff_case.expected {
			t.Errorf("%s: got %d, expected %d", backoff_case.name, got, backoff_case.expected)
		}
	}
}

func nan() float64 {
	zero := 0.0
	return zero / zero
}

func Test_retry_delay_table(t *testing.T) {
	cases := []struct {
		name              string
		attempt           int
		class             helpers.Class
		retry_after_ms    int64
		retry_after_ok    bool
		expected_retry    bool
		expected_delay_ms int64
	}{
		{"transient first attempt", 0, helpers.ClassTransient, 0, false, true, 100},
		{"the last attempt does not retry", 4, helpers.ClassTransient, 0, false, false, 0},
		{"past the last attempt", 9, helpers.ClassTransient, 0, false, false, 0},
		{"permanent never retries", 0, helpers.ClassPermanent, 0, false, false, 0},
		{"success never retries", 0, helpers.ClassSuccess, 0, false, false, 0},
		{"a larger Retry-After wins", 0, helpers.ClassTransient, 5000, true, true, 5000},
		{"a smaller Retry-After loses", 0, helpers.ClassTransient, 10, true, true, 100},
		{"a Retry-After that is not ok is ignored", 0, helpers.ClassTransient, 5000, false, true, 100},
	}
	for _, delay_case := range cases {
		delay_ms, retry := helpers.RetryDelay(delay_case.attempt, 5, delay_case.class, delay_case.retry_after_ms, delay_case.retry_after_ok, 200, 30000, 0)
		if retry != delay_case.expected_retry || delay_ms != delay_case.expected_delay_ms {
			t.Errorf("%s: got %d %v", delay_case.name, delay_ms, retry)
		}
	}
}

func Test_header_text_rules(t *testing.T) {
	if !helpers.IsVisibleASCII("") || !helpers.IsVisibleASCII("a!~") || helpers.IsVisibleASCII("a b") || helpers.IsVisibleASCII("a\x7f") || helpers.IsVisibleASCII("ñ") {
		t.Fatal("IsVisibleASCII is wrong")
	}
	if helpers.IsHTTPToken("") || !helpers.IsHTTPToken("X-Api-Key_1.2~|`^'*+!#$%&") || helpers.IsHTTPToken("X Api") || helpers.IsHTTPToken("X:Api") || helpers.IsHTTPToken("é") {
		t.Fatal("IsHTTPToken is wrong")
	}
}

func Test_default_settings(t *testing.T) {
	settings := http_client.DefaultSettings()
	if settings.Method != "POST" || settings.TimeoutMS != 3000 || settings.MaxAttempts != 5 || settings.RetryBaseDelayMS != 200 || settings.RetryMaxDelayMS != 30000 {
		t.Fatalf("%+v", settings)
	}
	if settings.URL != "" || settings.BearerToken != "" || settings.APIKeyValue != "" || settings.Compress || settings.SkipTLSVerification {
		t.Fatalf("a default that must be empty is not: %+v", settings)
	}
}

func Test_validation_reports_every_problem_in_the_order_of_the_schema_with_fixed_reasons(t *testing.T) {
	secret := "s3cr3t-value-that-must-not-appear"
	settings := http_client.Settings{
		URL: "http://logs.example.invalid/ingest?token=" + secret, Method: "GET", TimeoutMS: 0,
		BearerToken: "bad token " + secret, APIKeyHeader: "bad header", APIKeyValue: secret + " x",
	}
	validation_error := http_client.ValidateSettings(settings)
	var typed *config.ValidationError
	if !errors.As(validation_error, &typed) {
		t.Fatalf("got %v", validation_error)
	}
	fields := []string{}
	for _, problem := range typed.Problems {
		fields = append(fields, problem.Field)
	}
	expected_fields := "url,method,timeout_ms,bearer_token,api_key_header,api_key_value,max_attempts,retry_base_delay_ms,retry_max_delay_ms"
	if strings.Join(fields, ",") != expected_fields {
		t.Fatalf("fields %v", fields)
	}
	if strings.Contains(validation_error.Error(), secret) || strings.Contains(validation_error.Error(), "logs.example.invalid") {
		t.Fatalf("the error text holds a configured value: %v", validation_error)
	}
	if typed.Code() != value_objects.ErrorCodeConfigFieldInvalid {
		t.Fatalf("code %s", typed.Code())
	}
}

func Test_more_settings_rules(t *testing.T) {
	base := http_client.DefaultSettings()
	base.URL = "https://logs.example.invalid/ingest"
	cases := []struct {
		name     string
		change   func(settings *http_client.Settings)
		expected []string
	}{
		{"localhost over http with a token is allowed", func(s *http_client.Settings) { s.URL = "http://127.0.0.1:1/x"; s.BearerToken = "t" }, nil},
		{"ipv6 loopback over http with a token is allowed", func(s *http_client.Settings) { s.URL = "http://[::1]:1/x"; s.BearerToken = "t" }, nil},
		{"LOCALHOST uppercase is allowed", func(s *http_client.Settings) { s.URL = "http://LOCALHOST/x"; s.APIKeyHeader = "K"; s.APIKeyValue = "v" }, nil},
		{"a url that does not parse", func(s *http_client.Settings) { s.URL = "http://a b/\x7f" }, []string{"url"}},
		{"a token with a control character", func(s *http_client.Settings) { s.BearerToken = "a\x01" }, []string{"bearer_token"}},
		{"a bearer token and an API key value alone", func(s *http_client.Settings) { s.BearerToken = "t"; s.APIKeyValue = "v" }, []string{"api_key_header"}},
		{"a key header and a bearer token without value", func(s *http_client.Settings) { s.BearerToken = "t"; s.APIKeyHeader = "K" }, []string{"api_key_header", "api_key_value"}},
		{"an API key value with a space", func(s *http_client.Settings) { s.APIKeyHeader = "K"; s.APIKeyValue = "a b" }, []string{"api_key_value"}},
		{"a zero base", func(s *http_client.Settings) { s.RetryBaseDelayMS = 0 }, []string{"retry_base_delay_ms"}},
		{"a zero maximum", func(s *http_client.Settings) { s.RetryMaxDelayMS = 0 }, []string{"retry_max_delay_ms"}},
		{"a zero base and a smaller maximum", func(s *http_client.Settings) { s.RetryBaseDelayMS = 0; s.RetryMaxDelayMS = -1 }, []string{"retry_base_delay_ms", "retry_max_delay_ms"}},
		{"port 99999", func(s *http_client.Settings) { s.URL = "https://h:99999/x" }, []string{"url"}},
		{"port 0", func(s *http_client.Settings) { s.URL = "https://h:0/x" }, []string{"url"}},
		{"port 65536", func(s *http_client.Settings) { s.URL = "https://h:65536/x" }, []string{"url"}},
		{"port 1", func(s *http_client.Settings) { s.URL = "https://h:1/x" }, nil},
		{"port 65535", func(s *http_client.Settings) { s.URL = "https://h:65535/x" }, nil},
		{"ipv6 with port 70000", func(s *http_client.Settings) { s.URL = "https://[::1]:70000/x" }, []string{"url"}},
		{"an empty port is no port", func(s *http_client.Settings) { s.URL = "https://h:/x" }, nil},
		{"PATCH", func(s *http_client.Settings) { s.Method = "PATCH" }, nil},
		{"a lower case method", func(s *http_client.Settings) { s.Method = "post" }, []string{"method"}},
	}
	for _, settings_case := range cases {
		settings := base
		settings_case.change(&settings)
		validation_error := http_client.ValidateSettings(settings)
		got := []string{}
		var typed *config.ValidationError
		if errors.As(validation_error, &typed) {
			for _, problem := range typed.Problems {
				got = append(got, problem.Field)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(append([]string{}, settings_case.expected...)) {
			t.Errorf("%s: got %v, expected %v", settings_case.name, got, settings_case.expected)
		}
	}
}

func Test_the_transport_errors_have_fixed_reasons_and_keep_the_cause(t *testing.T) {
	cause := errors.New("Post https://host/?token=SECRET: boom")
	transport_error := adapter_error.MapTransportError(cause)
	if transport_error.Error() != adapter_error.ReasonNetworkError || transport_error.Reason() != adapter_error.ReasonNetworkError {
		t.Fatalf("got %s", transport_error.Error())
	}
	if !errors.Is(transport_error, cause) {
		t.Fatal("the cause is lost")
	}
	if adapter_error.MapTransportError(context.DeadlineExceeded).Reason() != adapter_error.ReasonTimeout {
		t.Fatal("a deadline is a timeout")
	}
	if adapter_error.MapReadError(cause).Reason() != adapter_error.ReasonResponseUnreadable {
		t.Fatal("a read error is response_unreadable")
	}
}

func Test_a_client_with_invalid_settings_is_not_made(t *testing.T) {
	_, new_error := http_client.NewClient(http_client.Settings{}, value_objects.DefaultLimits())
	var typed *config.ValidationError
	if !errors.As(new_error, &typed) {
		t.Fatalf("got %v", new_error)
	}
}
