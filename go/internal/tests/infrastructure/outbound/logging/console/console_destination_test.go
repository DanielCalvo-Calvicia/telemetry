package console_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console/external"
)

// The destination implements both ports.
var (
	_ outbound.LogSinkPort   = (*console.ConsoleDestination)(nil)
	_ outbound.EventSinkPort = (*console.ConsoleDestination)(nil)
)

// log_event builds a log event of the given severity and message.
func log_event(severity value_objects.Severity, message string) value_objects.Envelope {
	return text_event(value_objects.EventTypeLogEntryRecorded, severity, map[string]any{"category": "c", "message": message}, nil)
}

// failing_writer fails on the calls whose number (from 1) is in fail_on; the other lines go to the buffer.
type failing_writer struct {
	calls   int
	fail_on map[int]bool
	buffer  bytes.Buffer
}

var writer_broken_error = errors.New("the writer is broken")

func (writer *failing_writer) Write(content []byte) (int, error) {
	writer.calls++
	if writer.fail_on[writer.calls] {
		return 0, writer_broken_error
	}
	return writer.buffer.Write(content)
}

// panicking_writer panics on every write.
type panicking_writer struct{}

func (panicking_writer) Write([]byte) (int, error) { panic("the writer panicked") }

// short_writer takes one byte less than it is given and reports no error.
type short_writer struct{}

func (short_writer) Write(content []byte) (int, error) { return len(content) - 1, nil }

func new_destination(t *testing.T, settings console.Settings, writer io.Writer) *console.ConsoleDestination {
	t.Helper()
	destination, build_error := console.NewConsoleDestination(settings, writer)
	if build_error != nil {
		t.Fatal(build_error)
	}
	return destination
}

func Test_a_failing_writer_fails_only_the_lines_it_refused(t *testing.T) {
	writer := &failing_writer{fail_on: map[int]bool{2: true, 4: true}}
	destination := new_destination(t, console.DefaultSettings(), writer)
	events := []value_objects.Envelope{
		log_event(value_objects.SeverityInfo, "one"), log_event(value_objects.SeverityInfo, "two"),
		log_event(value_objects.SeverityInfo, "three"), log_event(value_objects.SeverityInfo, "four"),
		log_event(value_objects.SeverityInfo, "five"),
	}
	result := destination.Send(context.Background(), events)
	if result.Accepted != 3 || result.Failed != 2 || result.Dropped != 0 || result.Reason != "write_failed" {
		t.Fatalf("the result is %+v", result)
	}
	lines := strings.Split(strings.TrimSuffix(writer.buffer.String(), "\n"), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"message":"one"`) || !strings.Contains(lines[1], `"message":"three"`) ||
		!strings.Contains(lines[2], `"message":"five"`) {
		t.Fatalf("the written lines are %q", lines)
	}
}

func Test_a_short_write_is_a_failure(t *testing.T) {
	destination := new_destination(t, console.DefaultSettings(), short_writer{})
	result := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "m")})
	if result.Failed != 1 || result.Reason != "write_failed" {
		t.Fatalf("the result is %+v", result)
	}
	write_error := destination.WriteEvent(log_event(value_objects.SeverityInfo, "m"))
	if !errors.Is(write_error, adapter_error.ErrWriteFailed) || !errors.Is(write_error, external.ErrShortWrite) {
		t.Fatalf("the error is %v", write_error)
	}
}

func Test_a_writer_that_panics_does_not_panic_the_caller(t *testing.T) {
	destination := new_destination(t, console.DefaultSettings(), panicking_writer{})
	events := []value_objects.Envelope{log_event(value_objects.SeverityInfo, "one"), log_event(value_objects.SeverityInfo, "two")}
	result := destination.Send(context.Background(), events)
	if result.Failed != 2 || result.Accepted != 0 || result.Reason != "destination_failed" {
		t.Fatalf("the result is %+v", result)
	}
	// The mutex was released: another call works.
	if again := destination.Send(context.Background(), events[:1]); again.Failed != 1 {
		t.Fatalf("the second result is %+v", again)
	}
	write_error := destination.WriteEvent(events[0])
	if !errors.Is(write_error, adapter_error.ErrDestinationUnusable) {
		t.Fatalf("WriteEvent returned %v", write_error)
	}
}

func Test_a_panic_after_some_events_counts_only_the_rest_as_failed(t *testing.T) {
	// The first event is below the minimum (accepted without a write), the second makes the writer panic.
	destination := new_destination(t, console.DefaultSettings(), panicking_writer{})
	events := []value_objects.Envelope{log_event(value_objects.SeverityDebug, "quiet"), log_event(value_objects.SeverityInfo, "loud")}
	result := destination.Send(context.Background(), events)
	if result.Accepted != 1 || result.Failed != 1 || result.Reason != "destination_failed" {
		t.Fatalf("the result is %+v", result)
	}
}

func Test_a_zero_destination_fails_every_event_without_panicking(t *testing.T) {
	var nil_destination *console.ConsoleDestination
	result := nil_destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "m"), log_event(value_objects.SeverityInfo, "m")})
	if result.Failed != 2 || result.Reason != "destination_failed" {
		t.Fatalf("the result is %+v", result)
	}
	if !errors.Is(nil_destination.WriteEvent(log_event(value_objects.SeverityInfo, "m")), adapter_error.ErrDestinationUnusable) {
		t.Error("WriteEvent of a nil destination must be ErrDestinationUnusable")
	}
	empty_destination := &console.ConsoleDestination{}
	// A destination that was not built by the constructor has no minimum severity: it filters every event and does not panic.
	if result := empty_destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityInfo, "m")}); result.Failed != 0 {
		t.Fatalf("the result of a zero destination is %+v", result)
	}
}

func Test_an_event_that_cannot_be_encoded_fails_and_the_others_are_written(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		t.Run(format, func(t *testing.T) {
			var buffer bytes.Buffer
			settings := console.Settings{Format: format, MinimumSeverity: "INFO"}
			destination := new_destination(t, settings, &buffer)
			bad_event := log_event(value_objects.SeverityInfo, "bad")
			bad_event.Attributes = value_objects.Attributes{"ratio": math.NaN()}
			bad_event.Data["ratio"] = math.NaN() // the json format encodes the data, the text format the attributes
			events := []value_objects.Envelope{log_event(value_objects.SeverityInfo, "good one"), bad_event, log_event(value_objects.SeverityInfo, "good two")}
			result := destination.Send(context.Background(), events)
			if result.Accepted != 2 || result.Failed != 1 || result.Reason != "serialization_failed" {
				t.Fatalf("the result is %+v", result)
			}
			if strings.Count(buffer.String(), "\n") != 2 || !strings.Contains(buffer.String(), "good one") || !strings.Contains(buffer.String(), "good two") {
				t.Fatalf("the output is %q", buffer.String())
			}
			write_error := destination.WriteEvent(bad_event)
			if !errors.Is(write_error, adapter_error.ErrSerializationFailed) {
				t.Fatalf("WriteEvent returned %v", write_error)
			}
		})
	}
}

func Test_write_event_returns_a_classified_error_with_the_cause(t *testing.T) {
	writer := &failing_writer{fail_on: map[int]bool{1: true}}
	destination := new_destination(t, console.DefaultSettings(), writer)
	write_error := destination.WriteEvent(log_event(value_objects.SeverityInfo, "m"))
	if !errors.Is(write_error, adapter_error.ErrWriteFailed) || !errors.Is(write_error, writer_broken_error) {
		t.Fatalf("the error is %v", write_error)
	}
	// An event below the minimum is filtered: nothing is written and there is no error.
	if filtered_error := destination.WriteEvent(log_event(value_objects.SeverityDebug, "quiet")); filtered_error != nil {
		t.Fatalf("a filtered event returned %v", filtered_error)
	}
	if writer.calls != 1 {
		t.Fatalf("the writer was called %d times", writer.calls)
	}
	// A good write.
	if good_error := destination.WriteEvent(log_event(value_objects.SeverityInfo, "m")); good_error != nil {
		t.Fatal(good_error)
	}
	if !strings.HasSuffix(writer.buffer.String(), "\n") {
		t.Errorf("the line is %q", writer.buffer.String())
	}
}

func Test_a_filtered_event_counts_as_accepted_and_is_not_written(t *testing.T) {
	var buffer bytes.Buffer
	destination := new_destination(t, console.Settings{Format: "json", MinimumSeverity: "ERROR"}, &buffer)
	result := destination.Send(context.Background(), []value_objects.Envelope{
		log_event(value_objects.SeverityWarn, "no"), log_event(value_objects.SeverityError, "yes"), log_event("NOPE", "unknown"),
	})
	if result.Accepted != 3 || result.Failed != 0 || result.Dropped != 0 || result.Reason != "" {
		t.Fatalf("the result is %+v", result)
	}
	if strings.Count(buffer.String(), "\n") != 1 || !strings.Contains(buffer.String(), `"message":"yes"`) {
		t.Fatalf("the output is %q", buffer.String())
	}
}

func Test_invalid_settings_are_refused_with_the_validation_error(t *testing.T) {
	destination, build_error := console.NewConsoleDestination(console.Settings{Format: "xml", MinimumSeverity: "info"}, &bytes.Buffer{})
	if destination != nil || build_error == nil {
		t.Fatal("invalid settings must not build a destination")
	}
	var typed_error interface{ Code() string }
	if !errors.As(build_error, &typed_error) || typed_error.Code() != "config_field_invalid" {
		t.Fatalf("the error is %v", build_error)
	}
	if strings.Contains(build_error.Error(), "xml") {
		t.Errorf("the error holds the value: %v", build_error)
	}
}

func Test_the_standard_output_destination_builds_and_filters_without_writing(t *testing.T) {
	destination, build_error := console.NewStandardOutputDestination(console.Settings{Format: "text", MinimumSeverity: "CRITICAL"})
	if build_error != nil {
		t.Fatal(build_error)
	}
	// Every event is below CRITICAL, so nothing is written to the real standard output.
	result := destination.Send(context.Background(), []value_objects.Envelope{log_event(value_objects.SeverityError, "m")})
	if result.Accepted != 1 {
		t.Fatalf("the result is %+v", result)
	}
	if _, build_error := console.NewStandardOutputDestination(console.Settings{}); build_error == nil {
		t.Error("empty settings must be invalid")
	}
	if external.StandardOutput() == nil {
		t.Error("the standard output is nil")
	}
}

// locked_buffer is a buffer that two goroutines may write to; it records each Write call as one entry, so a line that
// was cut in two writes would show.
type locked_buffer struct {
	mutex  sync.Mutex
	writes [][]byte
}

func (buffer *locked_buffer) Write(content []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	buffer.writes = append(buffer.writes, append([]byte(nil), content...))
	return len(content), nil
}

func Test_concurrent_senders_never_interleave_their_lines(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		t.Run(format, func(t *testing.T) {
			buffer := &locked_buffer{}
			destination := new_destination(t, console.Settings{Format: format, MinimumSeverity: "INFO"}, buffer)
			const goroutines, events_each = 16, 25
			var group sync.WaitGroup
			for goroutine := 0; goroutine < goroutines; goroutine++ {
				group.Add(1)
				go func() {
					defer group.Done()
					events := make([]value_objects.Envelope, events_each)
					for position := range events {
						events[position] = log_event(value_objects.SeverityInfo, strings.Repeat("x", 200))
					}
					if result := destination.Send(context.Background(), events); result.Accepted != events_each {
						t.Errorf("the result is %+v", result)
					}
				}()
			}
			group.Wait()
			if len(buffer.writes) != goroutines*events_each {
				t.Fatalf("%d writes, expected %d", len(buffer.writes), goroutines*events_each)
			}
			for _, line := range buffer.writes {
				if !bytes.HasSuffix(line, []byte("\n")) || bytes.Count(line, []byte("\n")) != 1 {
					t.Fatalf("a line is not complete: %q", line)
				}
				if format == "json" {
					var parsed map[string]any
					if decode_error := json.Unmarshal(line, &parsed); decode_error != nil {
						t.Fatalf("a line is not JSON: %q", line)
					}
				}
			}
		})
	}
}
