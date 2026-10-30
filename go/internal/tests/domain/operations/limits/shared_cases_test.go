package limits_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/limits"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// fixtures_directory is the folder of the cases that the Python tests read too (the same files).
const fixtures_directory = "../../../../../../integration-tests/fixtures/limits"

// limits_options are the options of a case; an absent number is the default.
type limits_options struct {
	MaxAttributeKeys       *int `json:"max_attribute_keys"`
	MaxAttributeKeyChars   *int `json:"max_attribute_key_chars"`
	MaxAttributeValueChars *int `json:"max_attribute_value_chars"`
	MaxEventBytes          *int `json:"max_event_bytes"`
	MaxBatchBytes          *int `json:"max_batch_bytes"`
}

type report_fields struct {
	AttributeKeysDropped     int `json:"attribute_keys_dropped"`
	AttributeKeysTruncated   int `json:"attribute_keys_truncated"`
	AttributeValuesTruncated int `json:"attribute_values_truncated"`
	EventsRejected           int `json:"events_rejected"`
}

type shared_case struct {
	Description string          `json:"description"`
	Kind        string          `json:"kind"`
	Options     limits_options  `json:"options"`
	Input       json.RawMessage `json:"input"`
	Expected    json.RawMessage `json:"expected"`
	Report      report_fields   `json:"report"`
	ErrorCode   string          `json:"error_code"`
	Cases       []batch_case    `json:"cases"`
}

type batch_case struct {
	Description string         `json:"description"`
	Options     limits_options `json:"options"`
	Input       struct {
		BatchBytes int `json:"batch_bytes"`
		EventBytes int `json:"event_bytes"`
	} `json:"input"`
	Expected bool `json:"expected"`
}

func number_or_default(number *int, default_number int) int {
	if number == nil {
		return default_number
	}
	return *number
}

// build returns the limits of the options: NewLimits with the numbers of the case, the defaults where absent.
func (options limits_options) build() (value_objects.Limits, error) {
	defaults := value_objects.DefaultLimits()
	return value_objects.NewLimits(
		number_or_default(options.MaxAttributeKeys, defaults.MaxAttributeKeys()),
		number_or_default(options.MaxAttributeKeyChars, defaults.MaxAttributeKeyChars()),
		number_or_default(options.MaxAttributeValueChars, defaults.MaxAttributeValueChars()),
		number_or_default(options.MaxEventBytes, defaults.MaxEventBytes()),
		number_or_default(options.MaxBatchBytes, defaults.MaxBatchBytes()),
	)
}

func Test_every_shared_case_gives_the_same_limited_event_and_counts(t *testing.T) {
	file_paths, glob_error := filepath.Glob(filepath.Join(fixtures_directory, "*.json"))
	if glob_error != nil {
		t.Fatal(glob_error)
	}
	if len(file_paths) == 0 {
		t.Fatalf("no case in %s", fixtures_directory)
	}

	for _, file_path := range file_paths {
		t.Run(filepath.Base(file_path), func(t *testing.T) {
			content, read_error := os.ReadFile(file_path)
			if read_error != nil {
				t.Fatal(read_error)
			}
			var shared shared_case
			if decode_error := json.Unmarshal(content, &shared); decode_error != nil {
				t.Fatal(decode_error)
			}
			switch shared.Kind {
			case "limits":
				run_limits_case(t, shared)
			case "batch":
				run_batch_cases(t, shared)
			default:
				t.Fatalf("%s: the kind %q is not known (limits or batch)", shared.Description, shared.Kind)
			}
		})
	}
}

func run_limits_case(t *testing.T, shared shared_case) {
	t.Helper()
	case_limits, limits_error := shared.Options.build()
	if shared.ErrorCode == "config_field_invalid" {
		var classified *domain.ClassifiedError
		if !errors.As(limits_error, &classified) || classified.Code != shared.ErrorCode {
			t.Fatalf("%s: NewLimits error = %v, want the code %s", shared.Description, limits_error, shared.ErrorCode)
		}
		return
	}
	if limits_error != nil {
		t.Fatalf("%s: unexpected error of NewLimits: %v", shared.Description, limits_error)
	}

	var envelope value_objects.Envelope
	if decode_error := json.Unmarshal(shared.Input, &envelope); decode_error != nil {
		t.Fatal(decode_error)
	}
	limited_envelope, report, apply_error := limits.ApplyLimits(envelope, case_limits)

	expected_report := limits.Report{
		AttributeKeysDropped:     shared.Report.AttributeKeysDropped,
		AttributeKeysTruncated:   shared.Report.AttributeKeysTruncated,
		AttributeValuesTruncated: shared.Report.AttributeValuesTruncated,
		EventsRejected:           shared.Report.EventsRejected,
	}
	if report.AttributeKeysDropped != expected_report.AttributeKeysDropped {
		t.Errorf("%s: AttributeKeysDropped = %d, want %d", shared.Description, report.AttributeKeysDropped, expected_report.AttributeKeysDropped)
	}
	if report.AttributeKeysTruncated != expected_report.AttributeKeysTruncated {
		t.Errorf("%s: AttributeKeysTruncated = %d, want %d", shared.Description, report.AttributeKeysTruncated, expected_report.AttributeKeysTruncated)
	}
	if report.AttributeValuesTruncated != expected_report.AttributeValuesTruncated {
		t.Errorf("%s: AttributeValuesTruncated = %d, want %d", shared.Description, report.AttributeValuesTruncated, expected_report.AttributeValuesTruncated)
	}
	if report.EventsRejected != expected_report.EventsRejected {
		t.Errorf("%s: EventsRejected = %d, want %d", shared.Description, report.EventsRejected, expected_report.EventsRejected)
	}

	if shared.ErrorCode != "" {
		var classified *domain.ClassifiedError
		if !errors.As(apply_error, &classified) || classified.Code != shared.ErrorCode {
			t.Fatalf("%s: error = %v, want the code %s", shared.Description, apply_error, shared.ErrorCode)
		}
		return
	}
	if apply_error != nil {
		t.Fatalf("%s: unexpected error: %v", shared.Description, apply_error)
	}
	var expected_text string
	if decode_error := json.Unmarshal(shared.Expected, &expected_text); decode_error != nil || len(shared.Expected) == 0 {
		t.Fatalf("%s: the case has neither a text in expected nor an error_code", shared.Description)
	}
	canonical, canonical_error := canonical_json.CanonicalJSON(limited_envelope)
	if canonical_error != nil {
		t.Fatal(canonical_error)
	}
	if string(canonical) != expected_text {
		t.Errorf("%s:\n got  %s\n want %s", shared.Description, canonical, expected_text)
	}
}

func run_batch_cases(t *testing.T, shared shared_case) {
	t.Helper()
	if len(shared.Cases) == 0 {
		t.Fatalf("%s: the batch file has no case", shared.Description)
	}
	for _, batch := range shared.Cases {
		batch_limits, limits_error := batch.Options.build()
		if limits_error != nil {
			t.Fatalf("%s: %v", batch.Description, limits_error)
		}
		if actual := limits.FitsInBatch(batch.Input.BatchBytes, batch.Input.EventBytes, batch_limits); actual != batch.Expected {
			t.Errorf("%s: FitsInBatch = %v, want %v", batch.Description, actual, batch.Expected)
		}
	}
}
