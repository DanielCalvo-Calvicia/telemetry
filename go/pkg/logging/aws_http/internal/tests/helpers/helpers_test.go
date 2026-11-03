package helpers_test

import (
	"encoding/json"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http/helpers"
)

func log_event() value_objects.Envelope {
	return value_objects.Envelope{
		EventType:  "log.entry.recorded",
		OccurredAt: "2026-10-28T10:00:00.123456Z",
		Severity:   value_objects.SeverityInfo,
		Resource:   value_objects.Resource{ServiceName: "svc", InstanceId: "inst"},
		Data:       map[string]any{"category": "c", "function": "f", "message": "m"},
	}
}

func Test_build_endpoint_joins_host_port_and_path(t *testing.T) {
	if endpoint := helpers.BuildEndpoint("http://localhost", "8080", "/api/v1/cloudwatch/logs/add"); endpoint != "http://localhost:8080/api/v1/cloudwatch/logs/add" {
		t.Fatal(endpoint)
	}
}

func Test_map_level_writes_critical_as_fatal_and_keeps_the_rest(t *testing.T) {
	for _, severity := range value_objects.SeverityValues() {
		expected := string(severity)
		if severity == value_objects.SeverityCritical {
			expected = "FATAL"
		}
		if level := helpers.MapLevel(severity); level != expected {
			t.Errorf("%s gave %s", severity, level)
		}
	}
}

func Test_is_log_event_needs_the_prefix_log_dot(t *testing.T) {
	cases := map[string]bool{
		"log.entry.recorded": true, "log.custom.thing": true, "log.": true,
		"logger.x": false, "metric.log.x": false, "": false, "Log.entry": false,
	}
	for event_type, expected := range cases {
		if helpers.IsLogEvent(event_type) != expected {
			t.Errorf("%q must be %v", event_type, expected)
		}
	}
}

func Test_the_request_has_the_four_keys_in_order_and_the_timestamp_is_cut(t *testing.T) {
	event := log_event()
	event.OccurredAt = "2026-10-28T12:00:00.9999+02:00"
	body, build_error := helpers.BuildAddLogRequest(event, "g<&>", "s")
	if build_error != nil {
		t.Fatal(build_error)
	}
	var decoded struct {
		LogGroupName string `json:"log_group_name"`
		Timestamp    int64  `json:"timestamp"`
	}
	if decode_error := json.Unmarshal(body, &decoded); decode_error != nil {
		t.Fatal(decode_error)
	}
	if decoded.LogGroupName != "g<&>" || decoded.Timestamp != 1793181600999 {
		t.Fatalf("%s", body)
	}
	if string(body[:len(`{"log_group_name":"g<&>","log_stream_name":"s","message":`)]) != `{"log_group_name":"g<&>","log_stream_name":"s","message":` {
		t.Fatalf("%s", body)
	}
}

func Test_the_message_has_the_keys_of_the_old_entry_in_order_and_null_fields(t *testing.T) {
	message, build_error := helpers.BuildMessage(log_event())
	if build_error != nil {
		t.Fatal(build_error)
	}
	expected := `{"ProducerID":"svc","ApplicationID":"inst","ProcessID":"","Application":"svc","Category":"c","Function":"f",` +
		`"Indentation":0,"Depth":0,"Level":"INFO","Message":"m","Fields":null,"Timestamp":"2026-10-28T10:00:00.123456Z"}`
	if string(message) != expected {
		t.Fatalf("%s", message)
	}
}

func Test_an_empty_attribute_map_is_null_and_a_value_that_is_not_text_is_empty(t *testing.T) {
	event := log_event()
	event.Attributes = value_objects.Attributes{}
	event.Data = map[string]any{"category": 7, "message": nil}
	message, build_error := helpers.BuildMessage(event)
	if build_error != nil {
		t.Fatal(build_error)
	}
	expected := `{"ProducerID":"svc","ApplicationID":"inst","ProcessID":"","Application":"svc","Category":"","Function":"",` +
		`"Indentation":0,"Depth":0,"Level":"INFO","Message":"","Fields":null,"Timestamp":"2026-10-28T10:00:00.123456Z"}`
	if string(message) != expected {
		t.Fatalf("%s", message)
	}
}

func Test_a_text_that_is_not_utf8_is_an_error_in_every_place_it_can_be(t *testing.T) {
	places := map[string]func(event *value_objects.Envelope){
		"service name": func(event *value_objects.Envelope) { event.Resource.ServiceName = "\xff" },
		"instance id":  func(event *value_objects.Envelope) { event.Resource.InstanceId = "\xff" },
		"correlation":  func(event *value_objects.Envelope) { event.Context = &value_objects.Context{CorrelationId: "\xff"} },
		"category":     func(event *value_objects.Envelope) { event.Data = map[string]any{"category": "\xff"} },
		"function":     func(event *value_objects.Envelope) { event.Data = map[string]any{"function": "\xff"} },
		"message":      func(event *value_objects.Envelope) { event.Data = map[string]any{"message": "\xff"} },
		"attributes":   func(event *value_objects.Envelope) { event.Attributes = value_objects.Attributes{"k": "\xff"} },
		"timestamp":    func(event *value_objects.Envelope) { event.OccurredAt = "\xff" },
	}
	for name, change := range places {
		event := log_event()
		change(&event)
		if _, build_error := helpers.BuildAddLogRequest(event, "g", "s"); build_error == nil {
			t.Errorf("%s: an error is expected", name)
		}
	}
	for name, texts := range map[string][2]string{"group": {"\xff", "s"}, "stream": {"g", "\xff"}} {
		if _, build_error := helpers.BuildAddLogRequest(log_event(), texts[0], texts[1]); build_error == nil {
			t.Errorf("%s: an error is expected", name)
		}
	}
	// The message text itself is not UTF-8: BuildMessage fails on the Timestamp it copies as it is.
	event := log_event()
	event.OccurredAt = "\xff"
	if _, build_error := helpers.BuildMessage(event); build_error == nil {
		t.Error("an occurred_at that is not UTF-8 is an error")
	}
}

func Test_every_control_character_is_escaped_and_nothing_else(t *testing.T) {
	event := log_event()
	event.Data = map[string]any{"message": "\b\t\n\f\r\x01\x1f\\\"<>&é✓\x7f"}
	message, build_error := helpers.BuildMessage(event)
	if build_error != nil {
		t.Fatal(build_error)
	}
	expected := `"Message":"\b\t\n\f\r\u0001\u001f\\\"<>&é✓` + "\x7f" + `"`
	if !contains(string(message), expected) {
		t.Fatalf("%s", message)
	}
}

func contains(text string, part string) bool {
	for position := 0; position+len(part) <= len(text); position++ {
		if text[position:position+len(part)] == part {
			return true
		}
	}
	return false
}
