package contract_test

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func TestSeverityValuesAreTheSixOfTheContract(test *testing.T) {
	expected := []value_objects.Severity{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "CRITICAL"}
	got := value_objects.SeverityValues()
	if len(got) != len(expected) {
		test.Fatalf("got %d severities, want %d", len(got), len(expected))
	}
	for position := range expected {
		if got[position] != expected[position] {
			test.Errorf("severity %d is %q, want %q", position, got[position], expected[position])
		}
	}
	if value_objects.SeverityCritical != "CRITICAL" {
		test.Errorf("SeverityCritical is %q", value_objects.SeverityCritical)
	}
}

func TestTheOtherEnumsHaveTheirValues(test *testing.T) {
	if got := value_objects.ActorKindValues(); len(got) != 3 || got[0] != value_objects.ActorKindDevice {
		test.Errorf("actor kinds: %v", got)
	}
	if got := value_objects.FailureCategoryValues(); len(got) != 2 || got[1] != value_objects.FailureCategoryConfiguration {
		test.Errorf("failure categories: %v", got)
	}
}

func TestTheRegisteredEventTypeIsListed(test *testing.T) {
	got := value_objects.EventTypes()
	if len(got) != 1 || got[0] != value_objects.EventTypeLogEntryRecorded {
		test.Errorf("event types: %v", got)
	}
	if value_objects.EventTypeLogEntryRecorded != "log.entry.recorded" {
		test.Errorf("the constant is %q", value_objects.EventTypeLogEntryRecorded)
	}
}

func TestTheErrorCatalogHasTheFiveCodes(test *testing.T) {
	code_pattern := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	catalog := value_objects.ErrorCodes()
	if len(catalog) != 5 {
		test.Fatalf("got %d error codes, want 5", len(catalog))
	}
	seen := map[string]bool{}
	for _, info := range catalog {
		if !code_pattern.MatchString(info.Code) || seen[info.Code] || info.DefaultMessage == "" {
			test.Errorf("bad entry: %+v", info)
		}
		seen[info.Code] = true
	}
	if !seen[value_objects.ErrorCodeEventSchemaInvalid] || catalog[2].Category != value_objects.FailureCategoryValidation {
		test.Errorf("event_schema_invalid is missing or has the wrong category: %+v", catalog)
	}
}

func TestTheEnvelopeIsSerializedWithTheNamesOfTheContract(test *testing.T) {
	envelope := value_objects.Envelope{
		SchemaVersion: "0.1",
		EventId:       "01900000-0000-7000-8000-000000000000",
		EventType:     value_objects.EventTypeLogEntryRecorded,
		OccurredAt:    "2026-10-28T10:00:00.000000Z",
		RecordedAt:    "2026-10-28T10:00:00.000001Z",
		Severity:      value_objects.SeverityInfo,
		Resource:      value_objects.Resource{ServiceName: "my-service", InstanceId: "instance-1"},
		Data:          map[string]any{"category": "http", "message": "request received"},
	}
	encoded, marshal_error := json.Marshal(envelope)
	if marshal_error != nil {
		test.Fatal(marshal_error)
	}
	var decoded map[string]any
	if unmarshal_error := json.Unmarshal(encoded, &decoded); unmarshal_error != nil {
		test.Fatal(unmarshal_error)
	}
	for _, name := range []string{"schema_version", "event_id", "event_type", "occurred_at", "recorded_at", "severity", "resource", "data"} {
		if _, found := decoded[name]; !found {
			test.Errorf("the field %q is missing in %s", name, encoded)
		}
	}
	for _, name := range []string{"actor", "context", "attributes"} {
		if _, found := decoded[name]; found {
			test.Errorf("the optional field %q must be omitted in %s", name, encoded)
		}
	}
	data := value_objects.LogEntryRecorded{Category: "http", Message: "request received"}
	if encoded_data, _ := json.Marshal(data); string(encoded_data) != `{"category":"http","message":"request received"}` {
		test.Errorf("data: %s", encoded_data)
	}
}
