package entities_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/entities"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

var reference_time = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func new_valid_log_entry(t *testing.T, fields map[string]any) entities.LogEntry {
	t.Helper()
	log_entry, err := entities.NewLogEntry("producer", "application-id", "process-id", "app", "service", "Put", 2, 3, "INFO", "stored", fields, reference_time)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return log_entry
}

func Test_new_log_entry_builds_every_value_object(t *testing.T) {
	log_entry := new_valid_log_entry(t, map[string]any{"bucket": "b"})

	if log_entry.Identifiers != value_objects.NewIdentifiers("producer", "application-id", "process-id") {
		t.Errorf("Identifiers = %+v", log_entry.Identifiers)
	}
	if log_entry.Level != value_objects.LevelInfo || log_entry.Message.String() != "stored" {
		t.Errorf("Level/Message = %q %q", log_entry.Level, log_entry.Message)
	}
	if log_entry.Location != value_objects.NewLocation("app", "service", "Put") {
		t.Errorf("Location = %+v", log_entry.Location)
	}
	if log_entry.Indentation != value_objects.NewIndentation(2, 3) {
		t.Errorf("Indentation = %+v", log_entry.Indentation)
	}
	if log_entry.Fields["bucket"] != "b" || !log_entry.Timestamp.Equal(reference_time) {
		t.Errorf("Fields/Timestamp = %v %v", log_entry.Fields, log_entry.Timestamp)
	}
}

func Test_new_log_entry_with_an_invalid_level(t *testing.T) {
	log_entry, err := entities.NewLogEntry("p", "a", "r", "app", "cat", "fn", 1, 1, "VERBOSE", "msg", nil, reference_time)

	if !errors.Is(err, value_objects.ErrInvalidLevel) {
		t.Errorf("error = %v, want the level sentinel", err)
	}
	if !errors.Is(err, entities.ErrLogEntryCreationFailed) {
		t.Errorf("error = %v, want ErrLogEntryCreationFailed too", err)
	}
	if log_entry.Message != "" || log_entry.Level != "" {
		t.Errorf("a failed construction must return the zero entry: %+v", log_entry)
	}
}

func Test_new_log_entry_does_not_share_the_callers_fields(t *testing.T) {
	source_fields := map[string]any{"a": 1}
	log_entry := new_valid_log_entry(t, source_fields)
	source_fields["a"] = 2

	if log_entry.Fields["a"] != 1 {
		t.Error("the entry must copy the fields")
	}
}

func Test_add_fields_does_not_modify_the_original(t *testing.T) {
	original_entry := new_valid_log_entry(t, map[string]any{"a": 1})
	changed_entry := original_entry.AddFields("b", 2)

	if len(original_entry.Fields) != 1 || original_entry.Fields["b"] != nil {
		t.Errorf("the original entry changed: %v", original_entry.Fields)
	}
	if changed_entry.Fields["a"] != 1 || changed_entry.Fields["b"] != 2 {
		t.Errorf("AddFields() = %v", changed_entry.Fields)
	}
}

func Test_position_and_location_helpers_return_copies(t *testing.T) {
	original_entry := new_valid_log_entry(t, nil)

	if next_layer := original_entry.NextIndentation(); next_layer.Indentation != value_objects.NewIndentation(3, 1) {
		t.Errorf("NextIndentation() = %+v", next_layer.Indentation)
	}
	if next_step := original_entry.NextDepth(); next_step.Indentation != value_objects.NewIndentation(2, 4) {
		t.Errorf("NextDepth() = %+v", next_step.Indentation)
	}
	located_entry := original_entry.WithCategory("adapter").WithFunction("Get")
	if located_entry.Location.Category != "adapter" || located_entry.Location.Function != "Get" {
		t.Errorf("Location = %+v", located_entry.Location)
	}

	if original_entry.Indentation != value_objects.NewIndentation(2, 3) || original_entry.Location.Category != "service" {
		t.Error("the original entry must not change")
	}
}

func Test_log_entry_json_shape(t *testing.T) {
	log_entry := new_valid_log_entry(t, nil)

	encoded_entry, err := json.Marshal(log_entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded_entry map[string]any
	if err := json.Unmarshal(encoded_entry, &decoded_entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, expected_key := range []string{"identifiers", "level", "message", "timestamp", "location", "indentation"} {
		if _, found := decoded_entry[expected_key]; !found {
			t.Errorf("json is missing %q: %s", expected_key, encoded_entry)
		}
	}
	if _, found := decoded_entry["fields"]; found {
		t.Errorf("empty fields must be omitted: %s", encoded_entry)
	}
}
