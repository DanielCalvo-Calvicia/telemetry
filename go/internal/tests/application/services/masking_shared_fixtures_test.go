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
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// masking_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const masking_fixtures_folder = "../../../../../integration-tests/fixtures/masking/envelopes"

type masking_options struct {
	ExtraPatterns   []string          `json:"extra_patterns"`
	FieldNames      *[]string         `json:"field_names"`
	ExtraFieldNames []string          `json:"extra_field_names"`
	FieldClasses    map[string]string `json:"field_classes"`
	ClassRules      map[string]string `json:"class_rules"`
}

type masking_fixture struct {
	Description string          `json:"description"`
	Kind        string          `json:"kind"`
	Options     masking_options `json:"options"`
	Input       json.RawMessage `json:"input"`
	Expected    *string         `json:"expected"`
	ErrorCode   string          `json:"error_code"`
}

// to_masker_options maps the options of a fixture to the options of the masker. A present field_names, even an empty
// list, replaces the default list; an absent one is nil.
func (options masking_options) to_masker_options() masking.MaskerOptions {
	masker_options := masking.MaskerOptions{
		ExtraPatterns:   options.ExtraPatterns,
		ExtraFieldNames: options.ExtraFieldNames,
		FieldClasses:    options.FieldClasses,
		ClassRules:      options.ClassRules,
	}
	if options.FieldNames != nil {
		masker_options.FieldNames = append([]string{}, *options.FieldNames...)
	}
	return masker_options
}

func Test_every_shared_envelope_case_gives_the_same_masked_bytes(t *testing.T) {
	file_paths, glob_error := filepath.Glob(filepath.Join(masking_fixtures_folder, "*.json"))
	if glob_error != nil {
		t.Fatal(glob_error)
	}
	if len(file_paths) == 0 {
		t.Fatalf("no case in %s", masking_fixtures_folder)
	}

	for _, file_path := range file_paths {
		t.Run(filepath.Base(file_path), func(t *testing.T) {
			content, read_error := os.ReadFile(file_path)
			if read_error != nil {
				t.Fatal(read_error)
			}
			var fixture masking_fixture
			if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
				t.Fatal(decode_error)
			}
			if fixture.Kind != "mask" {
				t.Fatalf("%s: the kind %q is not known (only mask)", fixture.Description, fixture.Kind)
			}

			masker, masker_error := masking.NewSensitiveDataMasker(fixture.Options.to_masker_options())
			if fixture.ErrorCode != "" {
				var classified *domain.ClassifiedError
				if !errors.As(masker_error, &classified) || classified.Code != fixture.ErrorCode {
					t.Fatalf("%s: error = %v, want the code %s", fixture.Description, masker_error, fixture.ErrorCode)
				}
				return
			}
			if masker_error != nil {
				t.Fatalf("%s: unexpected error: %v", fixture.Description, masker_error)
			}
			if fixture.Expected == nil {
				t.Fatalf("%s: the case has neither expected nor error_code", fixture.Description)
			}

			var envelope value_objects.Envelope
			if decode_error := json.Unmarshal(fixture.Input, &envelope); decode_error != nil {
				t.Fatal(decode_error)
			}
			masked_envelope, mask_error := services.NewMaskingService(masker).Mask(context.Background(), envelope)
			if mask_error != nil {
				t.Fatalf("%s: unexpected error: %v", fixture.Description, mask_error)
			}
			if actual := canonical_text(t, masked_envelope); actual != *fixture.Expected {
				t.Errorf("%s:\n got  %s\n want %s", fixture.Description, actual, *fixture.Expected)
			}
		})
	}
}
