package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// limits_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const limits_fixtures_folder = "../../../../../integration-tests/fixtures/limits"

type limits_fixture_options struct {
	MaxAttributeKeys       *int `json:"max_attribute_keys"`
	MaxAttributeKeyChars   *int `json:"max_attribute_key_chars"`
	MaxAttributeValueChars *int `json:"max_attribute_value_chars"`
	MaxEventBytes          *int `json:"max_event_bytes"`
	MaxBatchBytes          *int `json:"max_batch_bytes"`
}

type limits_fixture struct {
	Description string                 `json:"description"`
	Kind        string                 `json:"kind"`
	Options     limits_fixture_options `json:"options"`
	Input       json.RawMessage        `json:"input"`
	Expected    *string                `json:"expected"`
	ErrorCode   string                 `json:"error_code"`
	Report      struct {
		AttributeKeysDropped     int `json:"attribute_keys_dropped"`
		AttributeKeysTruncated   int `json:"attribute_keys_truncated"`
		AttributeValuesTruncated int `json:"attribute_values_truncated"`
		EventsRejected           int `json:"events_rejected"`
	} `json:"report"`
}

func limits_number_or_default(number *int, default_number int) int {
	if number == nil {
		return default_number
	}
	return *number
}

func Test_every_shared_limits_case_gives_the_same_result_through_the_service(t *testing.T) {
	file_paths, glob_error := filepath.Glob(filepath.Join(limits_fixtures_folder, "*.json"))
	if glob_error != nil {
		t.Fatal(glob_error)
	}
	if len(file_paths) == 0 {
		t.Fatalf("no case in %s", limits_fixtures_folder)
	}

	for _, file_path := range file_paths {
		t.Run(filepath.Base(file_path), func(t *testing.T) {
			content, read_error := os.ReadFile(file_path)
			if read_error != nil {
				t.Fatal(read_error)
			}
			var fixture limits_fixture
			if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
				t.Fatal(decode_error)
			}
			switch fixture.Kind {
			case "batch":
				return // FitsInBatch is a function of the domain; its cases are run by the tests of the operation.
			case "limits":
			default:
				t.Fatalf("%s: the kind %q is not known (limits or batch)", fixture.Description, fixture.Kind)
			}

			defaults := value_objects.DefaultLimits()
			case_limits, limits_error := value_objects.NewLimits(
				limits_number_or_default(fixture.Options.MaxAttributeKeys, defaults.MaxAttributeKeys()),
				limits_number_or_default(fixture.Options.MaxAttributeKeyChars, defaults.MaxAttributeKeyChars()),
				limits_number_or_default(fixture.Options.MaxAttributeValueChars, defaults.MaxAttributeValueChars()),
				limits_number_or_default(fixture.Options.MaxEventBytes, defaults.MaxEventBytes()),
				limits_number_or_default(fixture.Options.MaxBatchBytes, defaults.MaxBatchBytes()),
			)
			var classified *domain.ClassifiedError
			if fixture.ErrorCode == "config_field_invalid" {
				if !errors.As(limits_error, &classified) || classified.Code != fixture.ErrorCode {
					t.Fatalf("%s: NewLimits error = %v, want the code %s", fixture.Description, limits_error, fixture.ErrorCode)
				}
				return
			}
			if limits_error != nil {
				t.Fatalf("%s: unexpected error of NewLimits: %v", fixture.Description, limits_error)
			}

			var envelope value_objects.Envelope
			if decode_error := json.Unmarshal(fixture.Input, &envelope); decode_error != nil {
				t.Fatal(decode_error)
			}
			limited_envelope, report, apply_error := services.NewLimitsService(case_limits).Apply(context.Background(), envelope)

			if report.AttributeKeysDropped != fixture.Report.AttributeKeysDropped ||
				report.AttributeKeysTruncated != fixture.Report.AttributeKeysTruncated ||
				report.AttributeValuesTruncated != fixture.Report.AttributeValuesTruncated ||
				report.EventsRejected != fixture.Report.EventsRejected {
				t.Errorf("%s: report = %+v, want %+v", fixture.Description, report, fixture.Report)
			}
			if fixture.ErrorCode != "" {
				if !errors.As(apply_error, &classified) || classified.Code != fixture.ErrorCode {
					t.Fatalf("%s: error = %v, want the code %s", fixture.Description, apply_error, fixture.ErrorCode)
				}
				return
			}
			if apply_error != nil {
				t.Fatalf("%s: unexpected error: %v", fixture.Description, apply_error)
			}
			if fixture.Expected == nil {
				t.Fatalf("%s: the case has neither expected nor error_code", fixture.Description)
			}
			canonical, canonical_error := canonical_json.CanonicalJSON(limited_envelope)
			if canonical_error != nil {
				t.Fatal(canonical_error)
			}
			if string(canonical) != *fixture.Expected {
				t.Errorf("%s:\n got  %s\n want %s", fixture.Description, canonical, *fixture.Expected)
			}
		})
	}
}
