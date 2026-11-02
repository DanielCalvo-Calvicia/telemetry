package http_client_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
)

const secret_looking_text = "placeholder-secret-9f8e7d6c5b4a3210-not-real"

func new_test_client(t *testing.T, settings http_client.Settings) *http_client.Client {
	t.Helper()
	client, new_error := http_client.NewClient(settings, value_objects.DefaultLimits())
	if new_error != nil {
		t.Fatal(new_error)
	}
	return client
}

func settings_for(url string) http_client.Settings {
	settings := http_client.DefaultSettings()
	settings.URL = url
	return settings
}

func Test_every_status_gives_its_class_and_reason(t *testing.T) {
	cases := map[int]helpers.Class{
		200: helpers.ClassSuccess, 201: helpers.ClassSuccess, 204: helpers.ClassSuccess,
		400: helpers.ClassPermanent, 401: helpers.ClassPermanent, 403: helpers.ClassPermanent, 404: helpers.ClassPermanent,
		408: helpers.ClassTransient, 413: helpers.ClassPermanent, 429: helpers.ClassTransient,
		500: helpers.ClassTransient, 502: helpers.ClassTransient, 503: helpers.ClassTransient,
	}
	for status, expected_class := range cases {
		status := status
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(status)
		}))
		outcome := new_test_client(t, settings_for(server.URL)).Send(context.Background(), []byte("{}"))
		server.Close()
		expected_reason := fmt.Sprintf("status_%d", status)
		if expected_class == helpers.ClassSuccess {
			expected_reason = ""
		}
		if outcome.Class != expected_class || outcome.StatusCode != status || outcome.Reason != expected_reason {
			t.Errorf("status %d: %+v", status, outcome)
		}
	}
}

func Test_retry_after_of_a_429_is_parsed(t *testing.T) {
	retry_after_header := "7"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", retry_after_header)
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := new_test_client(t, settings_for(server.URL))

	outcome := client.Send(context.Background(), []byte("{}"))
	if outcome.Class != helpers.ClassTransient || !outcome.RetryAfterOK || outcome.RetryAfterMilliseconds != 7000 {
		t.Fatalf("%+v", outcome)
	}
	delay_ms, retry := client.NextDelay(0, outcome, 0)
	if !retry || delay_ms != 7000 {
		t.Fatalf("the Retry-After must win over the backoff: %d %v", delay_ms, retry)
	}

	retry_after_header = time.Now().UTC().Add(30 * time.Second).Format(http.TimeFormat)
	outcome = client.Send(context.Background(), []byte("{}"))
	if !outcome.RetryAfterOK || outcome.RetryAfterMilliseconds < 25000 || outcome.RetryAfterMilliseconds > 31000 {
		t.Fatalf("a date: %+v", outcome)
	}

	retry_after_header = "soon"
	outcome = client.Send(context.Background(), []byte("{}"))
	if outcome.RetryAfterOK {
		t.Fatalf("a broken value must not be ok: %+v", outcome)
	}
}

func Test_a_server_that_never_answers_ends_at_the_timeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-release:
		case <-request.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	settings := settings_for(server.URL)
	settings.TimeoutMS = 200

	started := time.Now()
	outcome := new_test_client(t, settings).Send(context.Background(), []byte("{}"))
	if elapsed := time.Since(started); elapsed > 3*time.Second || elapsed < 150*time.Millisecond {
		t.Fatalf("it ended after %s", elapsed)
	}
	if outcome.Class != helpers.ClassTransient || outcome.Reason != "timeout" {
		t.Fatalf("%+v", outcome)
	}
}

func Test_a_closed_server_is_a_network_error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()
	outcome := new_test_client(t, settings_for(url)).Send(context.Background(), []byte("{}"))
	if outcome.Class != helpers.ClassTransient || outcome.Reason != "network_error" {
		t.Fatalf("%+v", outcome)
	}
}

func Test_a_cancelled_context_is_not_a_panic_and_is_transient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	outcome := new_test_client(t, settings_for(server.URL)).Send(ctx, []byte("{}"))
	if outcome.Class != helpers.ClassTransient || outcome.Reason == "" {
		t.Fatalf("%+v", outcome)
	}
}

func Test_a_redirect_is_not_followed(t *testing.T) {
	var second_requests atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		second_requests.Add(1)
	}))
	defer second.Close()
	first := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, second.URL, http.StatusFound)
	}))
	defer first.Close()

	outcome := new_test_client(t, settings_for(first.URL)).Send(context.Background(), []byte("{}"))
	if outcome.Class != helpers.ClassPermanent || outcome.StatusCode != 302 || outcome.Reason != "status_302" {
		t.Fatalf("%+v", outcome)
	}
	if second_requests.Load() != 0 {
		t.Fatal("the redirect was followed")
	}
}

func Test_the_request_has_the_method_the_headers_and_the_body(t *testing.T) {
	type seen_request struct {
		method, content_type, content_encoding, authorization, api_key string
		body                                                           []byte
	}
	for _, method := range []string{"POST", "PUT", "PATCH"} {
		for _, compress := range []bool{false, true} {
			var seen seen_request
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				body, _ := io.ReadAll(request.Body)
				seen = seen_request{request.Method, request.Header.Get("Content-Type"), request.Header.Get("Content-Encoding"),
					request.Header.Get("Authorization"), request.Header.Get("X-Api-Key"), body}
			}))
			settings := settings_for(server.URL)
			settings.Method = method
			settings.Compress = compress
			settings.BearerToken = "tok-123"
			plain := []byte(`{"events":[],"schema_version":"0.1"}`)
			outcome := new_test_client(t, settings).Send(context.Background(), plain)
			server.Close()

			if outcome.Class != helpers.ClassSuccess {
				t.Fatalf("%+v", outcome)
			}
			if seen.method != method || seen.content_type != "application/json" || seen.authorization != "Bearer tok-123" || seen.api_key != "" {
				t.Fatalf("%+v", seen)
			}
			body := seen.body
			if compress {
				if seen.content_encoding != "gzip" {
					t.Fatalf("Content-Encoding is %q", seen.content_encoding)
				}
				reader, reader_error := gzip.NewReader(bytes.NewReader(body))
				if reader_error != nil {
					t.Fatal(reader_error)
				}
				body, _ = io.ReadAll(reader)
			} else if seen.content_encoding != "" {
				t.Fatalf("Content-Encoding is %q", seen.content_encoding)
			}
			if !bytes.Equal(body, plain) {
				t.Fatalf("the body is %q", body)
			}
		}
	}
}

func Test_the_api_key_goes_in_its_own_header(t *testing.T) {
	var api_key, authorization string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		api_key, authorization = request.Header.Get("X-Api-Key"), request.Header.Get("Authorization")
	}))
	defer server.Close()
	settings := settings_for(server.URL)
	settings.APIKeyHeader, settings.APIKeyValue = "X-Api-Key", "k3y"
	if outcome := new_test_client(t, settings).Send(context.Background(), []byte("{}")); outcome.Class != helpers.ClassSuccess {
		t.Fatalf("%+v", outcome)
	}
	if api_key != "k3y" || authorization != "" {
		t.Fatalf("%q %q", api_key, authorization)
	}
}

func Test_a_body_over_the_batch_limit_is_not_sent(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	settings := settings_for(server.URL)
	settings.Compress = true // the plain size counts, not the compressed one
	too_large := bytes.Repeat([]byte("a"), value_objects.DefaultLimits().MaxBatchBytes()+1)
	outcome := new_test_client(t, settings).Send(context.Background(), too_large)
	if outcome.Class != helpers.ClassPermanent || outcome.Reason != "batch_too_large" || requests.Load() != 0 {
		t.Fatalf("%+v with %d requests", outcome, requests.Load())
	}
	exact := bytes.Repeat([]byte("a"), value_objects.DefaultLimits().MaxBatchBytes())
	if outcome := new_test_client(t, settings).Send(context.Background(), exact); outcome.Class != helpers.ClassSuccess || requests.Load() != 1 {
		t.Fatalf("the exact limit must be sent: %+v", outcome)
	}
}

func Test_tls_is_verified_by_default_and_can_be_turned_off(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()

	// A long timeout: the handshake of a test certificate can be slow when the machine is loaded by many test packages.
	verified_settings := settings_for(server.URL)
	verified_settings.TimeoutMS = 20000
	outcome := new_test_client(t, verified_settings).Send(context.Background(), []byte("{}"))
	if outcome.Class != helpers.ClassTransient || outcome.Reason != "tls_verification_failed" || calls.Load() != 0 {
		t.Fatalf("%+v with %d calls", outcome, calls.Load())
	}

	settings := settings_for(server.URL)
	settings.TimeoutMS = 20000
	settings.SkipTLSVerification = true
	outcome = new_test_client(t, settings).Send(context.Background(), []byte("{}"))
	if outcome.Class != helpers.ClassSuccess || calls.Load() != 1 {
		t.Fatalf("%+v with %d calls", outcome, calls.Load())
	}
}

func Test_no_secret_and_no_url_appear_in_an_outcome(t *testing.T) {
	failing_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer failing_server.Close()
	sleeping_release := make(chan struct{})
	sleeping_server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-sleeping_release:
		case <-request.Context().Done():
		}
	}))
	defer sleeping_server.Close()
	defer close(sleeping_release)
	tls_server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer tls_server.Close()
	closed_server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closed_url := closed_server.URL
	closed_server.Close()

	query := "?token=" + secret_looking_text
	for name, url := range map[string]string{"500": failing_server.URL, "timeout": sleeping_server.URL, "tls": tls_server.URL, "closed": closed_url} {
		for _, use_bearer := range []bool{true, false} {
			settings := settings_for(url + "/path" + query)
			settings.TimeoutMS = 200
			if use_bearer {
				settings.BearerToken = secret_looking_text
			} else {
				settings.APIKeyHeader, settings.APIKeyValue = "X-Api-Key", secret_looking_text
			}
			// plain http to 127.0.0.1 is local, so the authentication is allowed
			outcome := new_test_client(t, settings).Send(context.Background(), []byte("{}"))
			text := fmt.Sprintf("%+v %v %#v", outcome, outcome, outcome)
			for _, forbidden := range []string{secret_looking_text, "token=", "127.0.0.1", "/path"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("%s: the outcome holds %q: %s", name, forbidden, text)
				}
			}
			if outcome.Class == helpers.ClassSuccess {
				t.Errorf("%s: expected a failure, got %+v", name, outcome)
			}
		}
	}
}

func Test_send_never_panics(t *testing.T) {
	var no_client *http_client.Client
	outcome := no_client.Send(context.Background(), []byte("{}"))
	if outcome.Class != helpers.ClassTransient || outcome.Reason != http_client.ReasonClientFailed {
		t.Fatalf("%+v", outcome)
	}
}

func Test_send_batch_sends_the_canonical_body(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		received, _ = io.ReadAll(request.Body)
	}))
	defer server.Close()
	events := []value_objects.Envelope{
		{EventId: "01900000-0000-7000-8000-000000000001", EventType: "log.entry.recorded", SchemaVersion: "0.1", Data: map[string]any{"message": "m1"}},
		{EventId: "01900000-0000-7000-8000-000000000002", EventType: "log.entry.recorded", SchemaVersion: "0.1", Data: map[string]any{"message": "m2"}},
	}
	outcome := new_test_client(t, settings_for(server.URL)).SendBatch(context.Background(), events)
	if outcome.Class != helpers.ClassSuccess {
		t.Fatalf("%+v", outcome)
	}
	expected, _ := helpers.BuildBatchBody(events)
	if !bytes.Equal(received, expected) || !strings.HasPrefix(string(received), `{"events":[{"data":{"message":"m1"}`) || !strings.Contains(string(received), `"message":"m2"`) {
		t.Fatalf("the server received %s", received)
	}
}

func Test_send_batch_with_a_value_that_cannot_be_serialized_is_permanent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	events := []value_objects.Envelope{{Data: map[string]any{"bad": func() {}}}}
	outcome := new_test_client(t, settings_for(server.URL)).SendBatch(context.Background(), events)
	if outcome.Class != helpers.ClassPermanent || outcome.Reason != http_client.ReasonSerializationFailed {
		t.Fatalf("%+v", outcome)
	}
}

func Test_next_delay_follows_the_settings(t *testing.T) {
	settings := settings_for("https://logs.example.invalid/ingest")
	client := new_test_client(t, settings)
	if client.MaxAttempts() != 5 {
		t.Fatalf("max attempts %d", client.MaxAttempts())
	}
	transient := http_client.Outcome{Class: helpers.ClassTransient}
	if delay_ms, retry := client.NextDelay(0, transient, 0.5); !retry || delay_ms != 150 {
		t.Fatalf("%d %v", delay_ms, retry)
	}
	if _, retry := client.NextDelay(4, transient, 0.5); retry {
		t.Fatal("the last attempt must not retry")
	}
	for _, class := range []helpers.Class{helpers.ClassPermanent, helpers.ClassSuccess} {
		if _, retry := client.NextDelay(0, http_client.Outcome{Class: class}, 0.5); retry {
			t.Fatalf("%s must not retry", class)
		}
	}
	larger := http_client.Outcome{Class: helpers.ClassTransient, RetryAfterMilliseconds: 9000, RetryAfterOK: true}
	if delay_ms, retry := client.NextDelay(0, larger, 1); !retry || delay_ms != 9000 {
		t.Fatalf("%d %v", delay_ms, retry)
	}
}

func Test_a_response_with_an_unreadable_body_is_transient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", "1000")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("short"))
	}))
	defer server.Close()
	outcome := new_test_client(t, settings_for(server.URL)).Send(context.Background(), []byte("{}"))
	if outcome.Class != helpers.ClassTransient || outcome.Reason != "response_unreadable" || outcome.StatusCode != 200 {
		t.Fatalf("%+v", outcome)
	}
}

func Test_sixteen_goroutines_share_one_client(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	settings := settings_for(server.URL)
	settings.Compress = true
	client := new_test_client(t, settings)

	var group sync.WaitGroup
	outcomes := make([]http_client.Outcome, 16)
	for index := 0; index < 16; index++ {
		group.Add(1)
		go func(position int) {
			defer group.Done()
			outcomes[position] = client.Send(context.Background(), []byte("{}"))
		}(index)
	}
	group.Wait()
	for position, outcome := range outcomes {
		if outcome.Class != helpers.ClassSuccess {
			t.Errorf("goroutine %d: %+v", position, outcome)
		}
	}
	if requests.Load() != 16 {
		t.Fatalf("%d requests", requests.Load())
	}
}
