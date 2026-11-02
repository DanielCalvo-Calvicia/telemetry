package file_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file/helpers"
)

// file_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const file_fixtures_folder = "../../../../../../../integration-tests/fixtures/file"

// load_cases reads a fixture file of the folder into target and fails when the folder or the file is missing.
func load_cases(t *testing.T, file_name string, target any) {
	t.Helper()
	entries, read_error := os.ReadDir(file_fixtures_folder)
	if read_error != nil {
		t.Fatalf("the folder of the shared cases is missing: %v", read_error)
	}
	if len(entries) == 0 {
		t.Fatal("the folder of the shared cases has no files")
	}
	content, file_error := os.ReadFile(filepath.Join(file_fixtures_folder, file_name))
	if file_error != nil {
		t.Fatal(file_error)
	}
	if decode_error := json.Unmarshal(content, target); decode_error != nil {
		t.Fatalf("%s: %v", file_name, decode_error)
	}
}

func Test_scenarios_shared_cases(t *testing.T) {
	var scenarios struct {
		Cases []struct {
			Name        string                     `json:"name"`
			Description string                     `json:"description"`
			Settings    json.RawMessage            `json:"settings"`
			Batches     [][]value_objects.Envelope `json:"batches"`
			Expected    map[string]string          `json:"expected_files"`
			Counters    struct {
				FilesRotated         int64 `json:"files_rotated"`
				FilesDeletedForQuota int64 `json:"files_deleted_for_quota"`
			} `json:"expected_counters"`
		} `json:"cases"`
	}
	load_cases(t, "scenarios.json", &scenarios)
	if len(scenarios.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range scenarios.Cases {
		t.Run(shared_case.Name, func(t *testing.T) {
			settings := file.DefaultSettings()
			if decode_error := json.Unmarshal(shared_case.Settings, &settings); decode_error != nil {
				t.Fatal(decode_error)
			}
			directory := t.TempDir()
			settings.Directory = directory
			destination, build_error := file.NewFileDestination(settings)
			if build_error != nil {
				t.Fatal(build_error)
			}
			for _, batch := range shared_case.Batches {
				destination.Send(context.Background(), batch)
			}

			entries, read_error := os.ReadDir(directory)
			if read_error != nil {
				t.Fatal(read_error)
			}
			got_files := map[string]string{}
			for _, entry := range entries {
				content, file_error := os.ReadFile(filepath.Join(directory, entry.Name()))
				if file_error != nil {
					t.Fatal(file_error)
				}
				got_files[entry.Name()] = string(content)
			}
			if len(got_files) != len(shared_case.Expected) {
				t.Fatalf("the directory holds %d files, expected %d", len(got_files), len(shared_case.Expected))
			}
			for name, expected_content := range shared_case.Expected {
				got_content, has_file := got_files[name]
				if !has_file {
					t.Errorf("the file %s is missing", name)
					continue
				}
				if got_content != expected_content {
					t.Errorf("the file %s holds\n%q\nexpected\n%q", name, got_content, expected_content)
				}
			}
			counters := destination.Counters()
			if counters.FilesRotated != shared_case.Counters.FilesRotated || counters.FilesDeletedForQuota != shared_case.Counters.FilesDeletedForQuota {
				t.Errorf("the counters are %+v, expected rotated %d and deleted %d", counters,
					shared_case.Counters.FilesRotated, shared_case.Counters.FilesDeletedForQuota)
			}
		})
	}
}

func Test_rotation_plan_shared_cases(t *testing.T) {
	var plan struct {
		Cases []struct {
			Description string   `json:"description"`
			Base        string   `json:"base"`
			MaxFiles    int      `json:"max_files"`
			Existing    []string `json:"existing"`
			Expected    []struct {
				Op   string `json:"op"`
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"expected_steps"`
		} `json:"cases"`
	}
	load_cases(t, "rotation_plan.json", &plan)
	if len(plan.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range plan.Cases {
		t.Run(shared_case.Description, func(t *testing.T) {
			got := helpers.PlanRotation(shared_case.Base, shared_case.MaxFiles, shared_case.Existing)
			expected := []helpers.Step{}
			for _, step := range shared_case.Expected {
				expected = append(expected, helpers.Step{Op: step.Op, From: step.From, To: step.To})
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("the steps are %v, expected %v", got, expected)
			}
		})
	}
}

func Test_quota_plan_shared_cases(t *testing.T) {
	var plan struct {
		Cases []struct {
			Description string           `json:"description"`
			Base        string           `json:"base"`
			MaxFiles    int              `json:"max_files"`
			Sizes       map[string]int64 `json:"sizes"`
			QuotaBytes  int64            `json:"quota_bytes"`
			Expected    []string         `json:"expected_delete"`
		} `json:"cases"`
	}
	load_cases(t, "quota_plan.json", &plan)
	if len(plan.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range plan.Cases {
		t.Run(shared_case.Description, func(t *testing.T) {
			got := helpers.PlanQuota(shared_case.Base, shared_case.MaxFiles, shared_case.Sizes, shared_case.QuotaBytes)
			expected := shared_case.Expected
			if expected == nil {
				expected = []string{}
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("the files to delete are %v, expected %v", got, expected)
			}
		})
	}
}

func Test_settings_shared_cases(t *testing.T) {
	var file_cases struct {
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
	load_cases(t, "settings.json", &file_cases)
	if len(file_cases.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file_cases.Cases {
		t.Run(shared_case.Description, func(t *testing.T) {
			settings := file.DefaultSettings()
			if decode_error := json.Unmarshal(shared_case.Settings, &settings); decode_error != nil {
				t.Fatal(decode_error)
			}
			validation_error := file.ValidateSettings(settings)

			if shared_case.Expected != nil {
				if validation_error != nil {
					t.Fatalf("the settings must be valid: %v", validation_error)
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

			var typed_error *config.ValidationError
			if !errors.As(validation_error, &typed_error) {
				t.Fatalf("expected a *config.ValidationError, got %v", validation_error)
			}
			if len(typed_error.Problems) != len(shared_case.Problems) {
				t.Fatalf("%d problems %v, expected %d", len(typed_error.Problems), typed_error.Problems, len(shared_case.Problems))
			}
			for position, expected := range shared_case.Problems {
				got := typed_error.Problems[position]
				if got.Field != expected.Field || got.Code != expected.Code {
					t.Errorf("problem %d is %s/%s, expected %s/%s", position, got.Field, got.Code, expected.Field, expected.Code)
				}
				if got.Reason == "" {
					t.Errorf("problem %d has no reason", position)
				}
			}
		})
	}
}
