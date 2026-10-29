package value_objects_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// fixtures_directory is the folder of the tables that the Python tests read too (the same files).
const fixtures_directory = "../../../../../integration-tests/fixtures/value_objects"

// base_tables are the tables of the base value objects; they are read by the tests of that package.
var base_tables = map[string]bool{"identifier.json": true, "hex_digits.json": true}

type shared_case struct {
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input"`
	Valid       bool            `json:"valid"`
}

type shared_table struct {
	Description string        `json:"description"`
	Cases       []shared_case `json:"cases"`
}

// accepts_input builds the value object of one input and returns the error of the constructor and, for a valid input,
// whether the extra check of the table (the formatted text of an instant) holds.
type accepts_input func(test *testing.T, input json.RawMessage) (construction_error error, extra_check_failure string)

// decode_object decodes a JSON object into a map; the numbers arrive as float64 and the lists as []any.
func decode_object(test *testing.T, input json.RawMessage) map[string]any {
	test.Helper()
	var decoded map[string]any
	if decode_error := json.Unmarshal(input, &decoded); decode_error != nil {
		test.Fatalf("the input %s is not a JSON object: %v", input, decode_error)
	}
	return decoded
}

// decode_text decodes a JSON string.
func decode_text(test *testing.T, input json.RawMessage) string {
	test.Helper()
	var decoded string
	if decode_error := json.Unmarshal(input, &decoded); decode_error != nil {
		test.Fatalf("the input %s is not a JSON string: %v", input, decode_error)
	}
	return decoded
}

// optional_text returns the text of a field of the input and whether the field is present.
func optional_text(test *testing.T, object map[string]any, name string) (string, bool) {
	test.Helper()
	raw_value, is_present := object[name]
	if !is_present {
		return "", false
	}
	text, is_text := raw_value.(string)
	if !is_text {
		test.Fatalf("the field %s of the input is not a string: %v", name, raw_value)
	}
	return text, true
}

func accepts_severity(test *testing.T, input json.RawMessage) (error, string) {
	_, construction_error := value_objects.NewSeverity(decode_text(test, input))
	return construction_error, ""
}

func accepts_actor_kind(test *testing.T, input json.RawMessage) (error, string) {
	_, construction_error := value_objects.NewActorKind(decode_text(test, input))
	return construction_error, ""
}

func accepts_event_id(test *testing.T, input json.RawMessage) (error, string) {
	text := decode_text(test, input)
	event_id, construction_error := value_objects.NewEventID(text)
	if construction_error == nil && event_id.String() != text {
		return nil, "the id was changed to " + event_id.String()
	}
	return construction_error, ""
}

func accepts_utc_instant(test *testing.T, input json.RawMessage) (error, string) {
	instant, parse_error := time.Parse(time.RFC3339Nano, decode_text(test, input))
	if parse_error != nil {
		test.Fatalf("the input %s is not an ISO text with an offset: %v", input, parse_error)
	}
	utc_instant, construction_error := value_objects.NewUTCInstant(instant)
	if construction_error != nil {
		return construction_error, ""
	}
	expected := instant.UTC().Format("2006-01-02T15:04:05.000000") + "Z"
	if utc_instant.String() != expected {
		return nil, "the text is " + utc_instant.String() + ", want " + expected
	}
	return nil, ""
}

func accepts_resource(test *testing.T, input json.RawMessage) (error, string) {
	object := decode_object(test, input)
	service_name, _ := optional_text(test, object, "service_name")
	instance_id, _ := optional_text(test, object, "instance_id")
	var options []value_objects.ResourceOption
	if text, is_present := optional_text(test, object, "environment"); is_present {
		options = append(options, value_objects.WithEnvironment(text))
	}
	if text, is_present := optional_text(test, object, "service_version"); is_present {
		options = append(options, value_objects.WithServiceVersion(text))
	}
	if text, is_present := optional_text(test, object, "host"); is_present {
		options = append(options, value_objects.WithHost(text))
	}
	if text, is_present := optional_text(test, object, "time_zone"); is_present {
		options = append(options, value_objects.WithTimeZone(text))
	}
	_, construction_error := value_objects.NewResourceIdentity(service_name, instance_id, options...)
	return construction_error, ""
}

func accepts_actor(test *testing.T, input json.RawMessage) (error, string) {
	object := decode_object(test, input)
	kind, _ := optional_text(test, object, "kind")
	id, _ := optional_text(test, object, "id")
	var options []value_objects.ActorOption
	if text, is_present := optional_text(test, object, "type"); is_present {
		options = append(options, value_objects.WithActorType(text))
	}
	if text, is_present := optional_text(test, object, "version"); is_present {
		options = append(options, value_objects.WithActorVersion(text))
	}
	_, construction_error := value_objects.NewActorIdentity(value_objects.ActorKind(kind), id, options...)
	return construction_error, ""
}

func accepts_context(test *testing.T, input json.RawMessage) (error, string) {
	object := decode_object(test, input)
	var options []value_objects.ContextOption
	if text, is_present := optional_text(test, object, "trace_id"); is_present {
		options = append(options, value_objects.WithTraceID(text))
	}
	if text, is_present := optional_text(test, object, "span_id"); is_present {
		options = append(options, value_objects.WithSpanID(text))
	}
	if text, is_present := optional_text(test, object, "execution_id"); is_present {
		options = append(options, value_objects.WithExecutionID(text))
	}
	if text, is_present := optional_text(test, object, "correlation_id"); is_present {
		options = append(options, value_objects.WithCorrelationID(text))
	}
	_, construction_error := value_objects.NewCorrelationContext(options...)
	return construction_error, ""
}

func accepts_attributes(test *testing.T, input json.RawMessage) (error, string) {
	_, construction_error := value_objects.NewEventAttributes(decode_object(test, input))
	return construction_error, ""
}

// checkers maps each table of this package to the function that builds its value object.
var checkers = map[string]accepts_input{
	"severity.json":    accepts_severity,
	"actor_kind.json":  accepts_actor_kind,
	"event_id.json":    accepts_event_id,
	"utc_instant.json": accepts_utc_instant,
	"resource.json":    accepts_resource,
	"actor.json":       accepts_actor,
	"context.json":     accepts_context,
	"attributes.json":  accepts_attributes,
}

func TestEveryTableOfTheSharedFixturesIsKnown(test *testing.T) {
	entries, read_error := os.ReadDir(fixtures_directory)
	if read_error != nil {
		test.Fatalf("cannot read the fixtures folder: %v", read_error)
	}
	if len(entries) == 0 {
		test.Fatal("the fixtures folder is empty")
	}
	for _, entry := range entries {
		if _, is_checked := checkers[entry.Name()]; !is_checked && !base_tables[entry.Name()] {
			test.Errorf("the table %s has no check: add it to checkers (or to base_tables)", entry.Name())
		}
	}
	for table_name := range checkers {
		if _, stat_error := os.Stat(filepath.Join(fixtures_directory, table_name)); stat_error != nil {
			test.Errorf("the table %s of checkers does not exist: %v", table_name, stat_error)
		}
	}
}

func TestEveryValueObjectAcceptsExactlyTheValidCasesOfTheSharedTables(test *testing.T) {
	for table_name, check := range checkers {
		content, read_error := os.ReadFile(filepath.Join(fixtures_directory, table_name))
		if read_error != nil {
			test.Fatalf("cannot read %s: %v", table_name, read_error)
		}
		var table shared_table
		if decode_error := json.Unmarshal(content, &table); decode_error != nil {
			test.Fatalf("cannot decode %s: %v", table_name, decode_error)
		}
		if len(table.Cases) == 0 {
			test.Fatalf("the table %s has no case", table_name)
		}
		for _, current := range table.Cases {
			test.Run(strings.TrimSuffix(table_name, ".json")+"/"+current.Description, func(test *testing.T) {
				construction_error, extra_check_failure := check(test, current.Input)
				if current.Valid && construction_error != nil {
					test.Errorf("a valid input was rejected: %v", construction_error)
				}
				if !current.Valid && construction_error == nil {
					test.Errorf("an invalid input %s was accepted", current.Input)
				}
				if extra_check_failure != "" {
					test.Error(extra_check_failure)
				}
			})
		}
	}
}
