package console_test

import (
	"math"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console/helpers"
)

func Test_escape_text(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		expected string
	}{
		{"plain text is kept", "hello world", "hello world"},
		{"line feed", "a\nb", `a\nb`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"nul", "a\x00b", `a\u0000b`},
		{"escape character", "\x1b[31m", `\u001b[31m`},
		{"unit separator, the last control character", "\x1f", `\u001f`},
		{"another control character", "\x0b\x0c", `\u000b\u000c`},
		{"the space is kept", " ", " "},
		{"delete", "\x7f", `\u007f`},
		{"non ASCII is kept", "señal température", "señal température"},
		{"an astral character is kept", "ok 😀", "ok 😀"},
		{"empty", "", ""},
	}
	for _, escape_case := range cases {
		t.Run(escape_case.name, func(t *testing.T) {
			if got := helpers.EscapeText(escape_case.text); got != escape_case.expected {
				t.Fatalf("got %q, expected %q", got, escape_case.expected)
			}
		})
	}
}

// text_event builds an envelope of the given type, severity, data and attributes.
func text_event(event_type string, severity value_objects.Severity, data map[string]any, attributes value_objects.Attributes) value_objects.Envelope {
	return value_objects.Envelope{
		SchemaVersion: "0.1",
		EventId:       "01900000-0000-7000-8000-000000000001",
		EventType:     event_type,
		OccurredAt:    "2026-10-28T10:00:00.123456Z",
		RecordedAt:    "2026-10-28T10:00:00.123456Z",
		Severity:      severity,
		Data:          data,
		Attributes:    attributes,
	}
}

func Test_encode_text_line(t *testing.T) {
	log_type := value_objects.EventTypeLogEntryRecorded
	cases := []struct {
		name     string
		event    value_objects.Envelope
		expected string
	}{
		{
			"category and message",
			text_event(log_type, value_objects.SeverityInfo, map[string]any{"category": "http", "message": "started"}, nil),
			"2026-10-28T10:00:00.123456Z INFO     [http] started\n",
		},
		{
			"with a function",
			text_event(log_type, value_objects.SeverityCritical, map[string]any{"category": "http", "function": "serve", "message": "down"}, nil),
			"2026-10-28T10:00:00.123456Z CRITICAL [http] serve: down\n",
		},
		{
			"an empty function is left out",
			text_event(log_type, value_objects.SeverityWarn, map[string]any{"category": "http", "function": "", "message": "m"}, nil),
			"2026-10-28T10:00:00.123456Z WARN     [http] m\n",
		},
		{
			"a missing category",
			text_event(log_type, value_objects.SeverityTrace, map[string]any{"message": "m"}, nil),
			"2026-10-28T10:00:00.123456Z TRACE    [] m\n",
		},
		{
			"a non string message and function",
			text_event(log_type, value_objects.SeverityDebug, map[string]any{"category": "c", "function": 7, "message": 42}, nil),
			"2026-10-28T10:00:00.123456Z DEBUG    [c] \n",
		},
		{
			"no data at all",
			text_event(log_type, value_objects.SeverityError, nil, nil),
			"2026-10-28T10:00:00.123456Z ERROR    [] \n",
		},
		{
			"attributes sorted, with a nested list, a number and a bool",
			text_event(log_type, value_objects.SeverityInfo, map[string]any{"category": "c", "message": "m"},
				value_objects.Attributes{"tags": []any{"a", 1.0, true}, "ok": false, "n": 2.5, "name": "bob"}),
			`2026-10-28T10:00:00.123456Z INFO     [c] m n=2.5 name="bob" ok=false tags=["a",1,true]` + "\n",
		},
		{
			"a key that needs escaping",
			text_event(log_type, value_objects.SeverityInfo, map[string]any{"category": "c", "message": "m"},
				value_objects.Attributes{"k\ney": "v\nw"}),
			`2026-10-28T10:00:00.123456Z INFO     [c] m k\ney="v\nw"` + "\n",
		},
		{
			"another event type shows its type and the canonical JSON of its data",
			text_event("device.measurement.recorded", value_objects.SeverityInfo, map[string]any{"value": 21.5, "name": "temp"}, value_objects.Attributes{"unit": "C"}),
			`2026-10-28T10:00:00.123456Z INFO     device.measurement.recorded {"name":"temp","value":21.5} unit="C"` + "\n",
		},
		{
			"another event type with no data",
			text_event("device.measurement.recorded", value_objects.SeverityInfo, nil, nil),
			"2026-10-28T10:00:00.123456Z INFO     device.measurement.recorded {}\n",
		},
	}
	for _, text_case := range cases {
		t.Run(text_case.name, func(t *testing.T) {
			line, encode_error := helpers.EncodeTextLine(text_case.event)
			if encode_error != nil {
				t.Fatal(encode_error)
			}
			if string(line) != text_case.expected {
				t.Fatalf("got\n%q\nexpected\n%q", line, text_case.expected)
			}
		})
	}
}

func Test_encode_text_line_returns_the_error_of_a_value_without_canonical_json(t *testing.T) {
	bad_attribute := text_event(value_objects.EventTypeLogEntryRecorded, value_objects.SeverityInfo,
		map[string]any{"category": "c", "message": "m"}, value_objects.Attributes{"ratio": math.NaN()})
	if _, encode_error := helpers.EncodeTextLine(bad_attribute); encode_error == nil {
		t.Error("a NaN attribute must be an error")
	}
	bad_data := text_event("device.measurement.recorded", value_objects.SeverityInfo, map[string]any{"value": math.Inf(1)}, nil)
	if _, encode_error := helpers.EncodeTextLine(bad_data); encode_error == nil {
		t.Error("an infinite value in the data must be an error")
	}
}

func Test_encode_json_line_is_the_canonical_json_and_a_line_feed(t *testing.T) {
	event := text_event(value_objects.EventTypeLogEntryRecorded, value_objects.SeverityInfo, map[string]any{"category": "c", "message": "m"}, nil)
	line, encode_error := helpers.EncodeJSONLine(event)
	if encode_error != nil {
		t.Fatal(encode_error)
	}
	expected := `{"data":{"category":"c","message":"m"},"event_id":"01900000-0000-7000-8000-000000000001","event_type":"log.entry.recorded",` +
		`"occurred_at":"2026-10-28T10:00:00.123456Z","recorded_at":"2026-10-28T10:00:00.123456Z","resource":{"instance_id":"","service_name":""},` +
		`"schema_version":"0.1","severity":"INFO"}` + "\n"
	if string(line) != expected {
		t.Fatalf("got\n%s\nexpected\n%s", line, expected)
	}

	bad_event := text_event(value_objects.EventTypeLogEntryRecorded, value_objects.SeverityInfo, map[string]any{"value": math.NaN()}, nil)
	if _, encode_error := helpers.EncodeJSONLine(bad_event); encode_error == nil {
		t.Error("a NaN in the data must be an error")
	}
}
