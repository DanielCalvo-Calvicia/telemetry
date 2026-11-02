package console_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console"
)

// console_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const console_fixtures_folder = "../../../../../../../integration-tests/fixtures/console"

// load_cases reads a fixture file of the folder into target and fails when the folder or the file is missing.
func load_cases(t *testing.T, file_name string, target any) {
	t.Helper()
	entries, read_error := os.ReadDir(console_fixtures_folder)
	if read_error != nil {
		t.Fatalf("the folder of the shared cases is missing: %v", read_error)
	}
	if len(entries) == 0 {
		t.Fatal("the folder of the shared cases has no files")
	}
	content, file_error := os.ReadFile(filepath.Join(console_fixtures_folder, file_name))
	if file_error != nil {
		t.Fatal(file_error)
	}
	if decode_error := json.Unmarshal(content, target); decode_error != nil {
		t.Fatalf("%s: %v", file_name, decode_error)
	}
}

func Test_lines_shared_cases(t *testing.T) {
	var file struct {
		Cases []struct {
			Name            string                   `json:"name"`
			Description     string                   `json:"description"`
			Format          string                   `json:"format"`
			MinimumSeverity *string                  `json:"minimum_severity"`
			Events          []value_objects.Envelope `json:"events"`
			ExpectedOutput  string                   `json:"expected_output"`
			ExpectedResult  struct {
				Accepted int `json:"accepted"`
				Dropped  int `json:"dropped"`
				Failed   int `json:"failed"`
			} `json:"expected_result"`
		} `json:"cases"`
	}
	load_cases(t, "lines.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		t.Run(shared_case.Name, func(t *testing.T) {
			settings := console.DefaultSettings()
			settings.Format = shared_case.Format
			if shared_case.MinimumSeverity != nil {
				settings.MinimumSeverity = *shared_case.MinimumSeverity
			}
			var buffer bytes.Buffer
			destination, build_error := console.NewConsoleDestination(settings, &buffer)
			if build_error != nil {
				t.Fatal(build_error)
			}
			result := destination.Send(context.Background(), shared_case.Events)
			if buffer.String() != shared_case.ExpectedOutput {
				t.Fatalf("the output is\n%q\nexpected\n%q", buffer.String(), shared_case.ExpectedOutput)
			}
			if result.Accepted != shared_case.ExpectedResult.Accepted || result.Dropped != shared_case.ExpectedResult.Dropped ||
				result.Failed != shared_case.ExpectedResult.Failed {
				t.Fatalf("the result is %+v, expected %+v", result, shared_case.ExpectedResult)
			}
		})
	}
}

func Test_settings_shared_cases(t *testing.T) {
	var file struct {
		Cases []struct {
			Description string          `json:"description"`
			Settings    json.RawMessage `json:"settings"`
			Expected    *string         `json:"expected"`
			Problems    []struct {
				Field string `json:"field"`
				Code  string `json:"code"`
			} `json:"problems"`
		} `json:"cases"`
	}
	load_cases(t, "settings.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		t.Run(shared_case.Description, func(t *testing.T) {
			settings := console.DefaultSettings()
			if decode_error := json.Unmarshal(shared_case.Settings, &settings); decode_error != nil {
				t.Fatal(decode_error)
			}
			validation_error := console.ValidateSettings(settings)
			_, build_error := console.NewConsoleDestination(settings, &bytes.Buffer{})

			if shared_case.Expected != nil {
				if validation_error != nil || build_error != nil {
					t.Fatalf("the settings must be valid: %v, %v", validation_error, build_error)
				}
				canonical_text, canonical_error := canonical_json.CanonicalJSON(settings)
				if canonical_error != nil {
					t.Fatal(canonical_error)
				}
				if string(canonical_text) != *shared_case.Expected {
					t.Fatalf("the settings are %s, expected %s", canonical_text, *shared_case.Expected)
				}
				return
			}

			for label, got_error := range map[string]error{"ValidateSettings": validation_error, "NewConsoleDestination": build_error} {
				var typed_error *config.ValidationError
				if !errors.As(got_error, &typed_error) {
					t.Fatalf("%s: expected a *config.ValidationError, got %v", label, got_error)
				}
				if len(typed_error.Problems) != len(shared_case.Problems) {
					t.Fatalf("%s: %d problems %v, expected %d", label, len(typed_error.Problems), typed_error.Problems, len(shared_case.Problems))
				}
				for position, expected := range shared_case.Problems {
					got := typed_error.Problems[position]
					if got.Field != expected.Field || got.Code != expected.Code {
						t.Errorf("%s: problem %d is %s/%s, expected %s/%s", label, position, got.Field, got.Code, expected.Field, expected.Code)
					}
					if got.Reason == "" {
						t.Errorf("%s: problem %d has no reason", label, position)
					}
				}
			}
		})
	}
}
