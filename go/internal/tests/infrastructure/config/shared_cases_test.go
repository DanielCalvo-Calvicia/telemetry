package config_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

// config_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const config_fixtures_folder = "../../../../../integration-tests/fixtures/config"

type config_problem struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

type config_fixture struct {
	Description string                     `json:"description"`
	Kind        string                     `json:"kind"`
	Profile     string                     `json:"profile"`
	Overrides   map[string]json.RawMessage `json:"overrides"`
	Expected    *string                    `json:"expected"`
	Problems    []config_problem           `json:"problems"`
}

func profile_config(t *testing.T, profile string) config.Config {
	t.Helper()
	switch profile {
	case "microservice":
		return config.Microservice()
	case "cronjob":
		return config.Cronjob()
	case "worker":
		return config.Worker()
	case "robotics":
		return config.Robotics()
	case "offline_device":
		return config.OfflineDevice()
	}
	t.Fatalf("the profile %q is not known", profile)
	return config.Config{}
}

// apply_overrides applies the overrides over the config as encoding/json does: absent fields keep the profile value,
// slices replace, maps merge.
func apply_overrides(t *testing.T, base config.Config, overrides map[string]json.RawMessage) config.Config {
	t.Helper()
	overrides_text, marshal_error := json.Marshal(overrides)
	if marshal_error != nil {
		t.Fatal(marshal_error)
	}
	if decode_error := json.Unmarshal(overrides_text, &base); decode_error != nil {
		t.Fatal(decode_error)
	}
	return base
}

func Test_every_shared_config_case_gives_the_same_result(t *testing.T) {
	file_paths, glob_error := filepath.Glob(filepath.Join(config_fixtures_folder, "*.json"))
	if glob_error != nil {
		t.Fatal(glob_error)
	}
	if len(file_paths) == 0 {
		t.Fatalf("no case in %s", config_fixtures_folder)
	}

	for _, file_path := range file_paths {
		t.Run(filepath.Base(file_path), func(t *testing.T) {
			content, read_error := os.ReadFile(file_path)
			if read_error != nil {
				t.Fatal(read_error)
			}
			var fixture config_fixture
			if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
				t.Fatal(decode_error)
			}
			if fixture.Kind != "config" {
				t.Fatalf("%s: the kind %q is not known (config)", fixture.Description, fixture.Kind)
			}
			case_config := apply_overrides(t, profile_config(t, fixture.Profile), fixture.Overrides)
			validation_error := config.Validate(case_config)

			if fixture.Expected != nil {
				if validation_error != nil {
					t.Fatalf("%s: expected a valid configuration, got %v", fixture.Description, validation_error)
				}
				canonical_text, canonical_error := canonical_json.CanonicalJSON(case_config)
				if canonical_error != nil {
					t.Fatal(canonical_error)
				}
				if string(canonical_text) != *fixture.Expected {
					t.Fatalf("%s:\n got %s\nwant %s", fixture.Description, canonical_text, *fixture.Expected)
				}
				if _, provider_error := config.NewConfigurationProvider(context.Background(), case_config, fakes.NewFakeIDGenerator()); provider_error != nil {
					t.Fatalf("the provider of a valid configuration failed: %v", provider_error)
				}
				return
			}

			validation, is_validation := validation_error.(*config.ValidationError)
			if !is_validation {
				t.Fatalf("%s: expected a *ValidationError, got %v", fixture.Description, validation_error)
			}
			got_problems := make([]config_problem, 0, len(validation.Problems))
			for _, problem := range validation.Problems {
				got_problems = append(got_problems, config_problem{Field: problem.Field, Code: problem.Code})
			}
			if !reflect.DeepEqual(got_problems, fixture.Problems) {
				t.Fatalf("%s:\n got %v\nwant %v", fixture.Description, got_problems, fixture.Problems)
			}
			_, provider_error := config.NewConfigurationProvider(context.Background(), case_config, fakes.NewFakeIDGenerator())
			if !reflect.DeepEqual(provider_error, validation_error) {
				t.Fatalf("the provider returned %v, expected the same error as Validate", provider_error)
			}
		})
	}
}
