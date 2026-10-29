package event_validation_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// fixtures_directory is the folder of the tables that the Python tests read too (the same files).
const fixtures_directory = "../../../../../../integration-tests/fixtures/event_validation"

// valid_case is a case of valid_cases.json.
type valid_case struct {
	Name      string `json:"name"`
	EventType string `json:"event_type"`
	Data      any    `json:"data"`
}

// invalid_case is a case of invalid_cases.json.
type invalid_case struct {
	Name      string  `json:"name"`
	EventType string  `json:"event_type"`
	Data      any     `json:"data"`
	ErrorCode string  `json:"error_code"`
	FieldPath *string `json:"field_path"`
	Message   string  `json:"message"`
	Secret    string  `json:"secret"`
}

// load_cases reads the cases of a shared fixture file; an empty or missing file fails the test.
func load_cases[Case any](test *testing.T, relative_path string) []Case {
	test.Helper()
	content, read_error := os.ReadFile(filepath.Join(fixtures_directory, filepath.FromSlash(relative_path)))
	if read_error != nil {
		test.Fatalf("cannot read the fixture %s: %v", relative_path, read_error)
	}
	var file struct {
		Cases []Case `json:"cases"`
	}
	if decode_error := json.Unmarshal(content, &file); decode_error != nil {
		test.Fatalf("cannot decode the fixture %s: %v", relative_path, decode_error)
	}
	if len(file.Cases) == 0 {
		test.Fatalf("the fixture %s has no case", relative_path)
	}
	return file.Cases
}

// classified_code returns the code of a classified error, or "" when the error is nil or not classified.
func classified_code(validation_error error) string {
	var classified *domain.ClassifiedError
	if errors.As(validation_error, &classified) {
		return classified.Code
	}
	return ""
}

func TestEveryValidFixtureIsAccepted(test *testing.T) {
	for _, each := range load_cases[valid_case](test, "valid_cases.json") {
		test.Run(each.Name, func(test *testing.T) {
			if validation_error := event_validation.ValidateEvent(each.EventType, each.Data); validation_error != nil {
				test.Errorf("a valid event was rejected: %v", validation_error)
			}
		})
	}
}

func TestEveryInvalidFixtureGivesTheExpectedCodeAndFieldPath(test *testing.T) {
	for _, each := range load_cases[invalid_case](test, "invalid_cases.json") {
		test.Run(each.Name, func(test *testing.T) {
			validation_error := event_validation.ValidateEvent(each.EventType, each.Data)
			if validation_error == nil {
				test.Fatal("an invalid event was accepted")
			}
			if code := classified_code(validation_error); code != each.ErrorCode {
				test.Errorf("code %q, want %q (%v)", code, each.ErrorCode, validation_error)
			}
			message := validation_error.Error()
			if !strings.HasSuffix(message, "\n"+each.Message) {
				test.Errorf("message %q does not end with the detail %q", message, each.Message)
			}
			if each.FieldPath != nil && !strings.Contains(message, each.EventType+": "+*each.FieldPath+": ") {
				test.Errorf("message %q does not name the event type and the field path %q", message, *each.FieldPath)
			}
			if each.Secret != "" && strings.Contains(message, each.Secret) {
				test.Errorf("the message contains the offending value: %q", message)
			}
		})
	}
}

func TestTheErrorKeepsItsSentinelAndItsCode(test *testing.T) {
	schema_error := event_validation.ValidateEvent(value_objects.EventTypeLogEntryRecorded, map[string]any{})
	if !errors.Is(schema_error, domain.ErrEventSchemaInvalid) || errors.Is(schema_error, domain.ErrEventTypeUnknown) ||
		classified_code(schema_error) != value_objects.ErrorCodeEventSchemaInvalid {
		test.Errorf("schema error: %v", schema_error)
	}
	unknown_error := event_validation.ValidateEvent("no.such.type", map[string]any{})
	if !errors.Is(unknown_error, domain.ErrEventTypeUnknown) || errors.Is(unknown_error, domain.ErrEventSchemaInvalid) ||
		classified_code(unknown_error) != value_objects.ErrorCodeEventTypeUnknown {
		test.Errorf("unknown type error: %v", unknown_error)
	}
}

func TestEveryRegisteredEventTypeHasAValidator(test *testing.T) {
	for _, event_type := range value_objects.EventTypes() {
		validation_error := event_validation.ValidateEvent(event_type, nil)
		if classified_code(validation_error) != value_objects.ErrorCodeEventSchemaInvalid {
			test.Errorf("%s: a registered type must be validated, not reported as unknown: %v", event_type, validation_error)
		}
	}
}
