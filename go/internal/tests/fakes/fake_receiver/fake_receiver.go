// Package fake_receiver plays the receiver of the HTTP destination in tests (T-04.8). It listens only on 127.0.0.1,
// accepts the batch body {"schema_version", "events"} (REQ-OUT-010), decodes gzip, checks the authentication the test
// configured, answers with a script the test fills and records every request so a test can assert what arrived.
//
// It is a test helper and never shipped code. It is not a receiver design: it asserts the contract of the shared HTTP
// client (T-04.4) and nothing more. Python has the same fake with the same rules (integration-tests/fixtures/http).
package fake_receiver

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// max_body_bytes is the largest body (compressed, and again once decompressed) the fake reads.
	max_body_bytes = 8 << 20
	// max_drain_bytes is how much of an oversized body the fake discards, so the client can read the 413.
	max_drain_bytes = 64 << 20
	// truncated_announced_bytes and truncated_sent_bytes shape the answer of a truncated body.
	truncated_announced_bytes = 1000
	truncated_sent_bytes      = 10
	// poll_interval is the pause between two checks of WaitForEvents.
	poll_interval = 2 * time.Millisecond
)

// Config is what a test configures: the authentication the fake expects and the status it answers when the script is
// empty. Empty fields mean that nothing is checked.
type Config struct {
	// ExpectedBearer, when not empty, requires "Authorization: Bearer <ExpectedBearer>".
	ExpectedBearer string
	// ExpectedAPIKeyHeader and ExpectedAPIKeyValue, when the header is not empty, require that header with that value.
	ExpectedAPIKeyHeader string
	ExpectedAPIKeyValue  string
	// DefaultStatus is the status of a valid batch when the script is empty; 0 means 200.
	DefaultStatus int
}

// Answer is one scripted answer. A valid, authenticated request consumes the next one in order.
type Answer struct {
	// Status is the status code; 0 means 200.
	Status int
	// RetryAfter, when not empty, is sent as the Retry-After header.
	RetryAfter string
	// Location, when not empty, is sent as the Location header.
	Location string
	// Delay is waited before answering; Close releases it.
	Delay time.Duration
	// Close closes the connection without writing any answer.
	Close bool
	// Truncate writes the status line and headers announcing 1000 bytes of body, sends 10 and closes the connection.
	Truncate bool
}

// Request is what the fake recorded about one request.
type Request struct {
	Method string
	Path   string
	Query  string
	// Headers is a copy of the request headers.
	Headers http.Header
	// RawBody is the body as it arrived; Body is the same body decompressed when Content-Encoding is gzip.
	RawBody []byte
	Body    []byte
	Gzipped bool
	// Events holds the events of a valid batch, nil when the request is not valid.
	Events []json.RawMessage
	// Valid is true when the body is a batch of REQ-OUT-010; InvalidReason says why it is not (empty for a request
	// that failed authentication, which is not validated).
	Valid         bool
	InvalidReason string
	Authenticated bool
	// AnsweredStatus is the status the fake answered; 0 when it closed the connection without an answer.
	AnsweredStatus int
	ReceivedAt     time.Time
}

// Receiver is the fake receiver. It is safe for concurrent use.
type Receiver struct {
	config   Config
	listener net.Listener
	server   *http.Server
	done     chan struct{}
	once     sync.Once
	mutex    sync.Mutex
	script   []Answer
	requests []Request
}

// New starts a fake receiver bound only to 127.0.0.1 on a free port. The caller must Close it. It panics when the
// local address cannot be bound, because a test cannot continue without it.
func New(config Config) *Receiver {
	listener, listen_error := net.Listen("tcp", "127.0.0.1:0")
	if listen_error != nil {
		panic(fmt.Sprintf("fake_receiver: cannot listen on 127.0.0.1: %v", listen_error))
	}
	receiver := &Receiver{config: config, listener: listener, done: make(chan struct{})}
	receiver.server = &http.Server{Handler: http.HandlerFunc(receiver.handle), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = receiver.server.Serve(listener) }()
	return receiver
}

// URL returns the address of the fake, "http://127.0.0.1:<port>".
func (receiver *Receiver) URL() string {
	return "http://" + receiver.listener.Addr().String()
}

// Script appends answers to the script; each valid, authenticated request consumes one, first in first out.
func (receiver *Receiver) Script(answers ...Answer) {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()
	receiver.script = append(receiver.script, answers...)
}

// Reset clears the script and the recorded requests.
func (receiver *Receiver) Reset() {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()
	receiver.script = nil
	receiver.requests = nil
}

// Requests returns a deep copy of the recorded requests, in arrival order.
func (receiver *Receiver) Requests() []Request {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()
	copies := make([]Request, 0, len(receiver.requests))
	for _, request := range receiver.requests {
		copies = append(copies, copy_request(request))
	}
	return copies
}

// Events returns the events of the valid batches, in arrival order.
func (receiver *Receiver) Events() []json.RawMessage {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()
	var events []json.RawMessage
	for _, request := range receiver.requests {
		for _, event := range request.Events {
			events = append(events, append(json.RawMessage(nil), event...))
		}
	}
	return events
}

// WaitForEvents reports whether at least count events of valid batches arrived before the timeout.
func (receiver *Receiver) WaitForEvents(count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if receiver.event_count() >= count {
			return true
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		time.Sleep(min(poll_interval, remaining))
	}
}

// Close stops the fake and releases a request that waits on a delay. It can be called more than once.
func (receiver *Receiver) Close() {
	receiver.once.Do(func() {
		close(receiver.done)
		_ = receiver.server.Close()
	})
}

func (receiver *Receiver) event_count() int {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()
	total := 0
	for _, request := range receiver.requests {
		total += len(request.Events)
	}
	return total
}

func copy_request(request Request) Request {
	copied := request
	copied.Headers = request.Headers.Clone()
	copied.RawBody = append([]byte(nil), request.RawBody...)
	copied.Body = append([]byte(nil), request.Body...)
	if request.Events != nil {
		copied.Events = make([]json.RawMessage, len(request.Events))
		for position, event := range request.Events {
			copied.Events[position] = append(json.RawMessage(nil), event...)
		}
	}
	return copied
}

func (receiver *Receiver) handle(writer http.ResponseWriter, http_request *http.Request) {
	record := Request{
		Method:     http_request.Method,
		Path:       http_request.URL.Path,
		Query:      http_request.URL.RawQuery,
		Headers:    http_request.Header.Clone(),
		Gzipped:    strings.EqualFold(strings.TrimSpace(http_request.Header.Get("Content-Encoding")), "gzip"),
		ReceivedAt: time.Now().UTC(),
	}
	record.RawBody = read_limited(http_request.Body)
	oversized := len(record.RawBody) > max_body_bytes
	var gzip_failed bool
	switch {
	case oversized:
	case record.Gzipped:
		record.Body, oversized, gzip_failed = gunzip(record.RawBody)
	default:
		record.Body = record.RawBody
	}
	record.Authenticated = receiver.authenticated(http_request)

	var answer Answer
	switch {
	case !record.Authenticated:
		answer = Answer{Status: http.StatusUnauthorized}
	case oversized:
		record.InvalidReason = "the body is larger than 8 MiB"
		answer = Answer{Status: http.StatusRequestEntityTooLarge}
	case gzip_failed:
		record.InvalidReason = "the body is not valid gzip"
		answer = Answer{Status: http.StatusBadRequest}
	default:
		record.Events, record.InvalidReason = validate_batch(record.Body)
		record.Valid = record.InvalidReason == ""
		answer = Answer{Status: http.StatusBadRequest}
	}
	if oversized {
		record.RawBody, record.Body = nil, nil
	}

	receiver.mutex.Lock()
	if record.Valid {
		answer = receiver.next_answer()
	}
	if answer.Status == 0 {
		answer.Status = http.StatusOK
	}
	if !answer.Close {
		record.AnsweredStatus = answer.Status
	}
	receiver.requests = append(receiver.requests, record)
	receiver.mutex.Unlock()

	receiver.answer(writer, answer, record)
}

// next_answer takes the next scripted answer or builds the default one. The mutex must be held.
func (receiver *Receiver) next_answer() Answer {
	if len(receiver.script) == 0 {
		return Answer{Status: receiver.config.DefaultStatus}
	}
	next := receiver.script[0]
	receiver.script = receiver.script[1:]
	return next
}

func (receiver *Receiver) authenticated(http_request *http.Request) bool {
	if receiver.config.ExpectedBearer != "" && http_request.Header.Get("Authorization") != "Bearer "+receiver.config.ExpectedBearer {
		return false
	}
	if receiver.config.ExpectedAPIKeyHeader != "" &&
		http_request.Header.Get(receiver.config.ExpectedAPIKeyHeader) != receiver.config.ExpectedAPIKeyValue {
		return false
	}
	return true
}

func (receiver *Receiver) answer(writer http.ResponseWriter, answer Answer, record Request) {
	if answer.Delay > 0 {
		timer := time.NewTimer(answer.Delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-receiver.done:
			return
		}
	}
	if answer.Close || answer.Truncate {
		receiver.answer_on_connection(writer, answer)
		return
	}
	header := writer.Header()
	if answer.RetryAfter != "" {
		header.Set("Retry-After", answer.RetryAfter)
	}
	if answer.Location != "" {
		header.Set("Location", answer.Location)
	}
	var text string
	switch answer.Status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		text = record.InvalidReason
	case http.StatusUnauthorized:
		text = "unauthorized"
	}
	if text != "" {
		header.Set("Content-Type", "text/plain; charset=utf-8")
	}
	writer.WriteHeader(answer.Status)
	_, _ = io.WriteString(writer, text)
}

// answer_on_connection takes the connection over: it closes it without a word, or writes a short body after a long
// Content-Length and closes it.
func (receiver *Receiver) answer_on_connection(writer http.ResponseWriter, answer Answer) {
	connection, _, hijack_error := http.NewResponseController(writer).Hijack()
	if hijack_error != nil {
		return
	}
	defer connection.Close()
	if answer.Close {
		return
	}
	buffer := bufio.NewWriter(connection)
	fmt.Fprintf(buffer, "HTTP/1.1 %d %s\r\n", answer.Status, http.StatusText(answer.Status))
	if answer.RetryAfter != "" {
		fmt.Fprintf(buffer, "Retry-After: %s\r\n", answer.RetryAfter)
	}
	if answer.Location != "" {
		fmt.Fprintf(buffer, "Location: %s\r\n", answer.Location)
	}
	fmt.Fprintf(buffer, "Content-Length: %s\r\nConnection: close\r\n\r\n", strconv.Itoa(truncated_announced_bytes))
	buffer.WriteString(strings.Repeat("x", truncated_sent_bytes))
	_ = buffer.Flush()
}

// read_limited reads at most one byte more than the limit, then discards a bounded part of the rest.
func read_limited(body io.Reader) []byte {
	content, _ := io.ReadAll(io.LimitReader(body, max_body_bytes+1))
	if len(content) > max_body_bytes {
		_, _ = io.Copy(io.Discard, io.LimitReader(body, max_drain_bytes))
	}
	return content
}

// gunzip decompresses a body. It reports whether the result is over the limit and whether the body is not gzip.
func gunzip(compressed []byte) (body []byte, oversized bool, failed bool) {
	reader, reader_error := gzip.NewReader(bytes.NewReader(compressed))
	if reader_error != nil {
		return nil, false, true
	}
	defer reader.Close()
	body, read_error := io.ReadAll(io.LimitReader(reader, max_body_bytes+1))
	if read_error != nil {
		return nil, false, true
	}
	return body, len(body) > max_body_bytes, false
}

// validate_batch checks the body against REQ-OUT-010. It returns the events of a valid batch, or the reason.
func validate_batch(body []byte) ([]json.RawMessage, string) {
	if !json.Valid(body) {
		return nil, "the body is not valid JSON"
	}
	var fields map[string]json.RawMessage
	if decode_error := json.Unmarshal(body, &fields); decode_error != nil || fields == nil {
		return nil, "the body is not a JSON object"
	}
	schema_version, has_schema_version := fields["schema_version"]
	if !has_schema_version {
		return nil, "schema_version is missing"
	}
	events_field, has_events := fields["events"]
	if !has_events {
		return nil, "events is missing"
	}
	var unknown_keys []string
	for key := range fields {
		if key != "schema_version" && key != "events" {
			unknown_keys = append(unknown_keys, key)
		}
	}
	if len(unknown_keys) > 0 {
		sort.Strings(unknown_keys)
		return nil, "an unknown key: " + unknown_keys[0]
	}
	var version string
	if decode_error := json.Unmarshal(schema_version, &version); decode_error != nil || version != "0.1" {
		return nil, "schema_version must be 0.1"
	}
	var events []json.RawMessage
	if decode_error := json.Unmarshal(events_field, &events); decode_error != nil || events == nil {
		return nil, "events must be a list"
	}
	for _, event := range events {
		if !bytes.HasPrefix(bytes.TrimSpace(event), []byte("{")) {
			return nil, "every event must be an object"
		}
	}
	return events, ""
}
