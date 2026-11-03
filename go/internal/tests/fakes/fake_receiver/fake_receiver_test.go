package fake_receiver_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes/fake_receiver"
)

const valid_batch = `{"schema_version":"0.1","events":[{"n":1},{"n":2}]}`

func post(t *testing.T, target string, body string, headers map[string]string) (int, string) {
	t.Helper()
	request, request_error := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if request_error != nil {
		t.Fatal(request_error)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, do_error := client.Do(request)
	if do_error != nil {
		t.Fatal(do_error)
	}
	defer response.Body.Close()
	text, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(text)
}

func new_receiver(t *testing.T, config fake_receiver.Config) *fake_receiver.Receiver {
	t.Helper()
	receiver := fake_receiver.New(config)
	t.Cleanup(receiver.Close)
	return receiver
}

func Test_a_valid_batch_is_recorded_with_its_request_details(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	status, _ := post(t, receiver.URL()+"/v1/batch?x=1", valid_batch, map[string]string{"X-Test": "one"})
	records := receiver.Requests()
	if status != 200 || len(records) != 1 {
		t.Fatalf("status %d, %d records", status, len(records))
	}
	record := records[0]
	if record.Method != "POST" || record.Path != "/v1/batch" || record.Query != "x=1" || record.Headers.Get("X-Test") != "one" ||
		!record.Valid || !record.Authenticated || record.AnsweredStatus != 200 || record.Gzipped || len(record.Events) != 2 ||
		string(record.Body) != valid_batch || string(record.RawBody) != valid_batch || record.ReceivedAt.IsZero() {
		t.Fatalf("unexpected record %+v", record)
	}
	events := receiver.Events()
	if len(events) != 2 || string(events[1]) != `{"n":2}` {
		t.Fatalf("events %q", events)
	}
}

func Test_invalid_bodies_are_answered_400_with_the_reason(t *testing.T) {
	cases := map[string]string{
		"the body is not valid JSON":     `{nope`,
		"the body is not a JSON object":  `[1]`,
		"the body is not a JSON object ": `null`,
		"schema_version is missing":      `{"events":[]}`,
		"events is missing":              `{"schema_version":"0.1"}`,
		"an unknown key: extra":          `{"schema_version":"0.1","events":[],"extra":1}`,
		"schema_version must be 0.1":     `{"schema_version":"0.2","events":[]}`,
		"schema_version must be 0.1 ":    `{"schema_version":1,"events":[]}`,
		"events must be a list":          `{"schema_version":"0.1","events":{}}`,
		"events must be a list ":         `{"schema_version":"0.1","events":null}`,
		"every event must be an object":  `{"schema_version":"0.1","events":[{},1]}`,
	}
	receiver := new_receiver(t, fake_receiver.Config{})
	for reason, body := range cases {
		reason = strings.TrimSpace(reason)
		status, text := post(t, receiver.URL(), body, nil)
		if status != 400 || text != reason {
			t.Errorf("body %s: got %d %q, expected 400 %q", body, status, text, reason)
		}
	}
	for _, record := range receiver.Requests() {
		if record.Valid || record.InvalidReason == "" || record.Events != nil || record.AnsweredStatus != 400 {
			t.Errorf("a rejected request must be recorded as invalid: %+v", record)
		}
	}
}

func Test_a_body_that_is_not_gzip_is_invalid(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	status, text := post(t, receiver.URL(), "plain", map[string]string{"Content-Encoding": "gzip"})
	if status != 400 || text != "the body is not valid gzip" {
		t.Fatalf("%d %q", status, text)
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write([]byte(strings.Repeat("a", 1000)))
	_ = writer.Close()
	truncated := compressed.String()[:compressed.Len()-12]
	status, _ = post(t, receiver.URL(), truncated, map[string]string{"Content-Encoding": "gzip"})
	if status != 400 {
		t.Fatalf("a truncated gzip must be refused, got %d", status)
	}
}

func Test_a_gzipped_batch_is_decompressed_and_recorded(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write([]byte(valid_batch))
	_ = writer.Close()
	status, _ := post(t, receiver.URL(), compressed.String(), map[string]string{"Content-Encoding": "GZIP"})
	record := receiver.Requests()[0]
	if status != 200 || !record.Gzipped || string(record.Body) != valid_batch || bytes.Equal(record.RawBody, record.Body) {
		t.Fatalf("%d %+v", status, record)
	}
}

func Test_authentication_is_checked_and_a_failure_is_recorded_without_using_the_script(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{
		ExpectedBearer: "placeholder-token", ExpectedAPIKeyHeader: "X-Key", ExpectedAPIKeyValue: "placeholder-key",
	})
	receiver.Script(fake_receiver.Answer{Status: 503})
	wrong := []map[string]string{
		nil,
		{"Authorization": "Bearer placeholder-token"},
		{"X-Key": "placeholder-key"},
		{"Authorization": "Bearer other", "X-Key": "placeholder-key"},
	}
	for _, headers := range wrong {
		if status, _ := post(t, receiver.URL(), valid_batch, headers); status != 401 {
			t.Errorf("headers %v: expected 401, got %d", headers, status)
		}
	}
	good := map[string]string{"Authorization": "Bearer placeholder-token", "X-Key": "placeholder-key"}
	if status, _ := post(t, receiver.URL(), valid_batch, good); status != 503 {
		t.Fatalf("the script must still be there, got %d", status)
	}
	records := receiver.Requests()
	if len(records) != 5 || records[0].Authenticated || records[0].Valid || records[0].InvalidReason != "" || records[0].AnsweredStatus != 401 ||
		!records[4].Authenticated || records[4].AnsweredStatus != 503 {
		t.Fatalf("records %+v", records)
	}
}

func Test_scripted_answers_are_used_only_by_valid_requests_and_in_order(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{DefaultStatus: 202})
	receiver.Script(fake_receiver.Answer{Status: 429, RetryAfter: "7"})
	receiver.Script(fake_receiver.Answer{}, fake_receiver.Answer{Status: 302, Location: "/else"})
	if status, _ := post(t, receiver.URL(), "bad", nil); status != 400 {
		t.Fatalf("got %d", status)
	}
	expected := []int{429, 200, 302, 202}
	for position, want := range expected {
		if status, _ := post(t, receiver.URL(), valid_batch, nil); status != want {
			t.Errorf("request %d: expected %d, got %d", position, want, status)
		}
	}
}

func Test_a_closed_connection_and_a_truncated_answer_are_recorded(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	receiver.Script(fake_receiver.Answer{Close: true}, fake_receiver.Answer{Truncate: true, RetryAfter: "1", Location: "/x"})
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	if _, do_error := client.Post(receiver.URL(), "application/json", strings.NewReader(valid_batch)); do_error == nil {
		t.Fatal("a closed connection must be a transport error")
	}
	response, do_error := client.Post(receiver.URL(), "application/json", strings.NewReader(valid_batch))
	if do_error != nil {
		t.Fatal(do_error)
	}
	_, read_error := io.ReadAll(response.Body)
	response.Body.Close()
	if read_error == nil || response.Header.Get("Retry-After") != "1" || response.Header.Get("Location") != "/x" {
		t.Fatalf("the truncated body must fail to read: %v %v", read_error, response.Header)
	}
	records := receiver.Requests()
	if len(records) != 2 || records[0].AnsweredStatus != 0 || records[1].AnsweredStatus != 200 {
		t.Fatalf("records %+v", records)
	}
}

func Test_a_slow_answer_waits_and_close_releases_it(t *testing.T) {
	receiver := fake_receiver.New(fake_receiver.Config{})
	receiver.Script(fake_receiver.Answer{Delay: time.Minute})
	finished := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
		response, do_error := client.Post(receiver.URL(), "application/json", strings.NewReader(valid_batch))
		if do_error == nil {
			response.Body.Close()
		}
		finished <- do_error
	}()
	if !receiver.WaitForEvents(2, 5*time.Second) {
		t.Fatal("the request was not recorded while it waits")
	}
	started := time.Now()
	receiver.Close()
	receiver.Close()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not release the waiting request")
	}
	if time.Since(started) > 8*time.Second {
		t.Fatal("Close took too long")
	}
}

func Test_a_delay_shorter_than_the_wait_is_served(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	receiver.Script(fake_receiver.Answer{Delay: 50 * time.Millisecond, Status: 204})
	started := time.Now()
	status, _ := post(t, receiver.URL(), valid_batch, nil)
	if status != 204 || time.Since(started) < 45*time.Millisecond {
		t.Fatalf("%d after %v", status, time.Since(started))
	}
}

func Test_wait_for_events_sees_events_from_another_goroutine_and_times_out_otherwise(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	if !receiver.WaitForEvents(0, 0) {
		t.Fatal("zero events are always there")
	}
	started := time.Now()
	if receiver.WaitForEvents(1, 60*time.Millisecond) {
		t.Fatal("no event arrived")
	}
	if time.Since(started) < 55*time.Millisecond {
		t.Fatal("it must wait for the timeout")
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		post(t, receiver.URL(), valid_batch, nil)
	}()
	if !receiver.WaitForEvents(2, 5*time.Second) {
		t.Fatal("the events must be seen")
	}
	if receiver.WaitForEvents(3, 20*time.Millisecond) {
		t.Fatal("only two events exist")
	}
}

func Test_reset_clears_the_script_and_the_records(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	receiver.Script(fake_receiver.Answer{Status: 500})
	post(t, receiver.URL(), valid_batch, nil)
	receiver.Script(fake_receiver.Answer{Status: 500})
	receiver.Reset()
	if len(receiver.Requests()) != 0 || len(receiver.Events()) != 0 {
		t.Fatal("the records must be gone")
	}
	if status, _ := post(t, receiver.URL(), valid_batch, nil); status != 200 {
		t.Fatalf("the script must be gone, got %d", status)
	}
}

func Test_concurrent_senders_are_all_recorded(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	var group sync.WaitGroup
	for sender := 0; sender < 16; sender++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for repeat := 0; repeat < 5; repeat++ {
				post(t, receiver.URL(), valid_batch, nil)
			}
		}()
	}
	group.Wait()
	if len(receiver.Requests()) != 80 || len(receiver.Events()) != 160 {
		t.Fatalf("%d requests, %d events", len(receiver.Requests()), len(receiver.Events()))
	}
}

func Test_the_receiver_listens_only_on_the_loopback_address(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	parsed, parse_error := url.Parse(receiver.URL())
	if parse_error != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" {
		t.Fatalf("%q %v", receiver.URL(), parse_error)
	}
	addresses, _ := net.InterfaceAddrs()
	for _, address := range addresses {
		network, is_network := address.(*net.IPNet)
		if !is_network || network.IP.IsLoopback() || network.IP.To4() == nil {
			continue
		}
		connection, dial_error := net.DialTimeout("tcp", net.JoinHostPort(network.IP.String(), parsed.Port()), time.Second)
		if dial_error == nil {
			connection.Close()
			t.Fatalf("the receiver answers on %s", network.IP)
		}
	}
}

func Test_requests_returns_a_copy_and_headers_are_copied(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	post(t, receiver.URL(), valid_batch, map[string]string{"X-Test": "one"})
	first := receiver.Requests()
	first[0].Headers.Set("X-Test", "changed")
	first[0].RawBody[0] = 'X'
	first[0].Body[0] = 'X'
	first[0].Events[0][0] = 'X'
	events := receiver.Events()
	events[0][0] = 'X'
	second := receiver.Requests()
	if second[0].Headers.Get("X-Test") != "one" || string(second[0].Body) != valid_batch || string(second[0].RawBody) != valid_batch ||
		string(second[0].Events[0]) != `{"n":1}` || string(receiver.Events()[0]) != `{"n":1}` {
		t.Fatalf("the fake changed: %+v", second[0])
	}
}

func Test_an_oversized_body_is_answered_413_and_recorded_invalid(t *testing.T) {
	receiver := new_receiver(t, fake_receiver.Config{})
	receiver.Script(fake_receiver.Answer{Status: 503})
	big := strings.Repeat("a", 8<<20+10)
	status, text := post(t, receiver.URL(), big, nil)
	if status != 413 || !strings.Contains(text, "larger") {
		t.Fatalf("%d %q", status, text)
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write([]byte(big))
	_ = writer.Close()
	status, _ = post(t, receiver.URL(), compressed.String(), map[string]string{"Content-Encoding": "gzip"})
	if status != 413 {
		t.Fatalf("a body that is too big once decompressed must get 413, got %d", status)
	}
	records := receiver.Requests()
	if len(records) != 2 || records[0].Valid || records[0].InvalidReason == "" || records[0].AnsweredStatus != 413 || records[1].AnsweredStatus != 413 {
		t.Fatalf("%+v", records)
	}
	if status, _ := post(t, receiver.URL(), valid_batch, nil); status != 503 {
		t.Fatal("an oversized request must not use the script")
	}
}
