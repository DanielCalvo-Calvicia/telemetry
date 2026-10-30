package limits_test

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/limits"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func new_envelope(attributes value_objects.Attributes) value_objects.Envelope {
	return value_objects.Envelope{
		SchemaVersion: "0.1",
		EventId:       "01900000-0000-7000-8000-000000000001",
		EventType:     "log.entry.recorded",
		OccurredAt:    "2026-10-28T10:00:00.123456Z",
		RecordedAt:    "2026-10-28T10:00:00.123456Z",
		Severity:      "INFO",
		Resource:      value_objects.Resource{ServiceName: "sensor-hub", InstanceId: "hub-01"},
		Data:          map[string]any{"category": "c", "message": "the message of the event"},
		Attributes:    attributes,
	}
}

func must_limits(t *testing.T, keys int, key_chars int, value_chars int, event_bytes int, batch_bytes int) value_objects.Limits {
	t.Helper()
	result, limits_error := value_objects.NewLimits(keys, key_chars, value_chars, event_bytes, batch_bytes)
	if limits_error != nil {
		t.Fatal(limits_error)
	}
	return result
}

func Test_a_value_of_256_characters_is_kept_and_one_of_257_is_cut(t *testing.T) {
	defaults := value_objects.DefaultLimits()
	kept, kept_report, _ := limits.ApplyLimits(new_envelope(value_objects.Attributes{"k": strings.Repeat("a", 256)}), defaults)
	if kept.Attributes["k"] != strings.Repeat("a", 256) || kept_report != (limits.Report{}) {
		t.Errorf("256 characters must be kept, report %+v", kept_report)
	}
	cut, cut_report, _ := limits.ApplyLimits(new_envelope(value_objects.Attributes{"k": strings.Repeat("a", 257)}), defaults)
	if cut.Attributes["k"] != strings.Repeat("a", 256) || cut_report.AttributeValuesTruncated != 1 {
		t.Errorf("257 characters must be cut to 256 and counted, report %+v", cut_report)
	}
}

func Test_characters_are_code_points_not_bytes_and_not_utf16_units(t *testing.T) {
	defaults := value_objects.DefaultLimits()
	limited, report, _ := limits.ApplyLimits(new_envelope(value_objects.Attributes{"k": strings.Repeat("\U0001F600", 257)}), defaults)
	if limited.Attributes["k"] != strings.Repeat("\U0001F600", 256) || report.AttributeValuesTruncated != 1 {
		t.Errorf("257 emoji must be cut to 256 emoji, report %+v", report)
	}
	long_key := strings.Repeat("é", 65)
	limited_key, key_report, _ := limits.ApplyLimits(new_envelope(value_objects.Attributes{long_key: 1}), defaults)
	if _, found := limited_key.Attributes[strings.Repeat("é", 64)]; !found || key_report.AttributeKeysTruncated != 1 {
		t.Errorf("a key of 65 characters must be cut to 64, report %+v", key_report)
	}
}

func Test_the_33rd_key_in_sorted_order_is_dropped_and_counted(t *testing.T) {
	attributes := value_objects.Attributes{}
	for _, key := range []string{"b", "a", "c"} {
		attributes[key] = 1
	}
	limited, report, apply_error := limits.ApplyLimits(new_envelope(attributes), must_limits(t, 2, 64, 256, 65536, 1048576))
	if apply_error != nil || len(limited.Attributes) != 2 || limited.Attributes["c"] != nil || report.AttributeKeysDropped != 1 {
		t.Errorf("got %v, report %+v, error %v", limited.Attributes, report, apply_error)
	}
}

func Test_the_order_is_utf16_code_units_not_code_points(t *testing.T) {
	// U+FF5E (BMP) sorts after U+1F600 (surrogate pair D83D DE00) in UTF-16 units, but before it in code points.
	attributes := value_objects.Attributes{"～": 1, "\U0001F600": 2}
	limited, report, _ := limits.ApplyLimits(new_envelope(attributes), must_limits(t, 1, 64, 256, 65536, 1048576))
	if _, found := limited.Attributes["\U0001F600"]; !found || report.AttributeKeysDropped != 1 {
		t.Errorf("the key with the surrogate pair must be first: %v", limited.Attributes)
	}
}

func Test_a_key_that_is_equal_to_a_kept_key_after_the_cut_is_dropped(t *testing.T) {
	attributes := value_objects.Attributes{"abc1": "first", "abc2": "second"}
	limited, report, _ := limits.ApplyLimits(new_envelope(attributes), must_limits(t, 32, 3, 256, 65536, 1048576))
	if limited.Attributes["abc"] != "first" || len(limited.Attributes) != 1 {
		t.Errorf("attributes = %v", limited.Attributes)
	}
	if report.AttributeKeysTruncated != 1 || report.AttributeKeysDropped != 1 {
		t.Errorf("report = %+v", report)
	}
}

func Test_every_text_of_a_list_is_cut_on_its_own_and_other_values_are_never_cut(t *testing.T) {
	long_text := strings.Repeat("x", 10)
	attributes := value_objects.Attributes{
		"list_any":    []any{long_text, "ok", 12345678901, true, nil},
		"list_string": []string{long_text, long_text},
		"number":      1234567890123,
		"flag":        true,
		"nothing":     nil,
	}
	limited, report, _ := limits.ApplyLimits(new_envelope(attributes), must_limits(t, 32, 64, 4, 65536, 1048576))
	if !reflect.DeepEqual(limited.Attributes["list_any"], []any{"xxxx", "ok", 12345678901, true, nil}) {
		t.Errorf("list_any = %v", limited.Attributes["list_any"])
	}
	if !reflect.DeepEqual(limited.Attributes["list_string"], []any{"xxxx", "xxxx"}) {
		t.Errorf("list_string = %v", limited.Attributes["list_string"])
	}
	if limited.Attributes["number"] != 1234567890123 || limited.Attributes["flag"] != true || limited.Attributes["nothing"] != nil {
		t.Errorf("scalars changed: %v", limited.Attributes)
	}
	if report.AttributeValuesTruncated != 3 {
		t.Errorf("report = %+v", report)
	}
}

func Test_an_envelope_without_attributes_is_returned_without_attributes(t *testing.T) {
	for _, attributes := range []value_objects.Attributes{nil, {}} {
		limited, report, apply_error := limits.ApplyLimits(new_envelope(attributes), value_objects.DefaultLimits())
		if apply_error != nil || limited.Attributes != nil || report != (limits.Report{}) {
			t.Errorf("attributes %v: got %v, %+v, %v", attributes, limited.Attributes, report, apply_error)
		}
	}
}

func Test_when_no_key_remains_the_attributes_are_omitted(t *testing.T) {
	// Every key is dropped only if the limit of keys is zero, which NewLimits refuses; the smallest limit keeps one.
	limited, report, _ := limits.ApplyLimits(new_envelope(value_objects.Attributes{"a": 1, "b": 2}), must_limits(t, 1, 1, 1, 65536, 65536))
	if len(limited.Attributes) != 1 || report.AttributeKeysDropped != 1 {
		t.Errorf("got %v, %+v", limited.Attributes, report)
	}
	// An empty key (not valid for an event, but possible in a map) is kept and counts as a key.
	empty_key, _, _ := limits.ApplyLimits(new_envelope(value_objects.Attributes{"": 1}), value_objects.DefaultLimits())
	if len(empty_key.Attributes) != 1 {
		t.Errorf("got %v", empty_key.Attributes)
	}
}

func Test_the_input_and_its_attributes_are_never_modified(t *testing.T) {
	original := value_objects.Attributes{
		"a" + strings.Repeat("k", 69): strings.Repeat("v", 300),
		"list":                        []any{strings.Repeat("w", 300), 1},
		"a1":                          1, "b": 2, "c": 3,
	}
	envelope := new_envelope(original)
	snapshot := new_envelope(value_objects.Attributes{
		"a" + strings.Repeat("k", 69): strings.Repeat("v", 300),
		"list":                        []any{strings.Repeat("w", 300), 1},
		"a1":                          1, "b": 2, "c": 3,
	})
	_, report, _ := limits.ApplyLimits(envelope, must_limits(t, 2, 10, 10, 65536, 65536))
	if report.AttributeKeysDropped == 0 || report.AttributeKeysTruncated == 0 {
		t.Fatalf("the case must cut something: %+v", report)
	}
	if !reflect.DeepEqual(envelope, snapshot) {
		t.Error("the envelope that was passed in was modified")
	}
}

func Test_only_the_attributes_are_cut(t *testing.T) {
	envelope := new_envelope(value_objects.Attributes{"k": "short"})
	envelope.Data = map[string]any{"message": strings.Repeat("d", 1000)}
	limited, _, apply_error := limits.ApplyLimits(envelope, must_limits(t, 1, 1, 1, 65536, 65536))
	if apply_error != nil || limited.Data["message"] != strings.Repeat("d", 1000) {
		t.Errorf("data must not be cut: %v", apply_error)
	}
}

func Test_an_event_of_exactly_the_limit_is_accepted_and_one_byte_more_is_rejected(t *testing.T) {
	envelope := new_envelope(nil)
	canonical, _ := canonical_json.CanonicalJSON(envelope)
	size := len(canonical)

	_, accepted_report, accepted_error := limits.ApplyLimits(envelope, must_limits(t, 32, 64, 256, size, size))
	if accepted_error != nil || accepted_report.EventsRejected != 0 {
		t.Fatalf("exactly the limit must be accepted: %v", accepted_error)
	}

	limited, rejected_report, rejected_error := limits.ApplyLimits(envelope, must_limits(t, 32, 64, 256, size-1, size))
	var classified *domain.ClassifiedError
	if !errors.As(rejected_error, &classified) || classified.Code != value_objects.ErrorCodeLimitExceeded {
		t.Fatalf("error = %v, want limit_exceeded", rejected_error)
	}
	if !errors.Is(rejected_error, domain.ErrEventTooLarge) || !errors.Is(rejected_error, limits.ErrEventTooLarge) {
		t.Error("the error must wrap ErrEventTooLarge")
	}
	if !reflect.DeepEqual(limited, value_objects.Envelope{}) || rejected_report.EventsRejected != 1 {
		t.Errorf("a rejected event returns the empty envelope and EventsRejected = 1, got %+v", rejected_report)
	}
	if text := rejected_error.Error(); strings.Contains(text, "the message of the event") || strings.Contains(text, "sensor-hub") {
		t.Errorf("the error must not hold the content of the event: %s", text)
	}
}

func Test_a_rejected_event_keeps_the_other_counters(t *testing.T) {
	envelope := new_envelope(value_objects.Attributes{"k": strings.Repeat("v", 300)})
	_, report, apply_error := limits.ApplyLimits(envelope, must_limits(t, 32, 64, 256, 10, 10))
	if apply_error == nil || report.EventsRejected != 1 || report.AttributeValuesTruncated != 1 {
		t.Errorf("report = %+v, error = %v", report, apply_error)
	}
}

func Test_an_unsupported_value_returns_the_canonical_error_and_an_empty_envelope(t *testing.T) {
	envelope := new_envelope(nil)
	envelope.Data = map[string]any{"channel": make(chan int)}
	limited, report, apply_error := limits.ApplyLimits(envelope, value_objects.DefaultLimits())
	if !errors.Is(apply_error, domain.ErrUnsupportedValue) {
		t.Fatalf("error = %v", apply_error)
	}
	if !reflect.DeepEqual(limited, value_objects.Envelope{}) || report.EventsRejected != 0 {
		t.Errorf("got %+v, %+v", limited, report)
	}
}

func Test_the_report_holds_only_integers(t *testing.T) {
	report_type := reflect.TypeOf(limits.Report{})
	if report_type.NumField() != 4 {
		t.Errorf("the report has %d fields, want 4", report_type.NumField())
	}
	for position := 0; position < report_type.NumField(); position++ {
		if report_type.Field(position).Type.Kind() != reflect.Int {
			t.Errorf("the field %s is not an int", report_type.Field(position).Name)
		}
	}
}

func Test_fits_in_batch_accepts_exactly_the_limit(t *testing.T) {
	batch_limits := must_limits(t, 1, 1, 1, 50, 100)
	if !limits.FitsInBatch(60, 40, batch_limits) {
		t.Error("exactly the limit must fit")
	}
	if limits.FitsInBatch(60, 41, batch_limits) {
		t.Error("one byte more must not fit")
	}
	if !limits.FitsInBatch(0, 0, batch_limits) {
		t.Error("nothing fits in an empty batch")
	}
}

func Test_apply_limits_is_safe_for_concurrent_use(t *testing.T) {
	attributes := value_objects.Attributes{"a": strings.Repeat("v", 300), "b": []any{strings.Repeat("w", 300)}, "c": 1}
	envelope := new_envelope(attributes)
	expected, _, _ := limits.ApplyLimits(envelope, value_objects.DefaultLimits())

	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for repetition := 0; repetition < 50; repetition++ {
				limited, report, apply_error := limits.ApplyLimits(envelope, value_objects.DefaultLimits())
				if apply_error != nil || report.AttributeValuesTruncated != 2 || !reflect.DeepEqual(limited, expected) {
					t.Error("a concurrent call gave another result")
					return
				}
			}
		}()
	}
	group.Wait()
}
