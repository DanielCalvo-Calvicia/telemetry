package http_destination_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	http_destination "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/http"
)

// destination_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const destination_fixtures_folder = "../../../../../../../integration-tests/fixtures/http_destination"

// load_cases reads a fixture file of the folder into target and fails when the folder or the file is missing.
func load_cases(t *testing.T, file_name string, target any) {
	t.Helper()
	entries, read_error := os.ReadDir(destination_fixtures_folder)
	if read_error != nil {
		t.Fatalf("the folder of the shared cases is missing: %v", read_error)
	}
	if len(entries) == 0 {
		t.Fatal("the folder of the shared cases has no files")
	}
	content, file_error := os.ReadFile(filepath.Join(destination_fixtures_folder, file_name))
	if file_error != nil {
		t.Fatal(file_error)
	}
	if decode_error := json.Unmarshal(content, target); decode_error != nil {
		t.Fatalf("%s: %v", file_name, decode_error)
	}
}

func Test_requests_shared_cases(t *testing.T) {
	var file struct {
		Cases []struct {
			Name            string                   `json:"name"`
			Description     string                   `json:"description"`
			Events          []value_objects.Envelope `json:"events"`
			MinimumSeverity *string                  `json:"minimum_severity"`
			Settings        json.RawMessage          `json:"settings"`
			ServerStatus    int                      `json:"server_status"`
			ExpectedBody    *string                  `json:"expected_body"`
			ExpectedHeaders map[string]string        `json:"expected_headers"`
			ExpectedNoReq   bool                     `json:"expected_no_request"`
			ExpectedResult  struct {
				Accepted int    `json:"accepted"`
				Dropped  int    `json:"dropped"`
				Failed   int    `json:"failed"`
				Reason   string `json:"reason"`
			} `json:"expected_result"`
		} `json:"cases"`
	}
	load_cases(t, "requests.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		t.Run(shared_case.Name, func(t *testing.T) {
			receiver := new_receiver(t, shared_case.ServerStatus)
			settings := settings_for(receiver.server.URL)
			if len(shared_case.Settings) > 0 {
				if decode_error := json.Unmarshal(shared_case.Settings, &settings); decode_error != nil {
					t.Fatal(decode_error)
				}
				settings.URL = receiver.server.URL
			}
			if shared_case.MinimumSeverity != nil {
				settings.MinimumSeverity = *shared_case.MinimumSeverity
			}
			destination := new_destination(t, settings)

			result := destination.Send(context.Background(), shared_case.Events)

			if result.Accepted != shared_case.ExpectedResult.Accepted || result.Dropped != shared_case.ExpectedResult.Dropped ||
				result.Failed != shared_case.ExpectedResult.Failed || result.Reason != shared_case.ExpectedResult.Reason {
				t.Fatalf("the result is %+v, expected %+v", result, shared_case.ExpectedResult)
			}
			requests := receiver.received()
			if shared_case.ExpectedNoReq {
				if len(requests) != 0 {
					t.Fatalf("the receiver must get no request, got %d", len(requests))
				}
				return
			}
			if len(requests) != 1 {
				t.Fatalf("the receiver must get exactly one request, got %d", len(requests))
			}
			if shared_case.ExpectedBody == nil {
				t.Fatal("the case has neither expected_body nor expected_no_request")
			}
			if requests[0].Body != *shared_case.ExpectedBody {
				t.Fatalf("the body is\n%s\nexpected\n%s", requests[0].Body, *shared_case.ExpectedBody)
			}
			for header_name, header_value := range shared_case.ExpectedHeaders {
				if requests[0].Headers.Get(header_name) != header_value {
					t.Errorf("the header %s is %q, expected %q", header_name, requests[0].Headers.Get(header_name), header_value)
				}
			}
			for _, request := range requests {
				if request.Path == "/moved" {
					t.Errorf("a redirect was followed")
				}
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
			settings := http_destination.DefaultSettings()
			if decode_error := json.Unmarshal(shared_case.Settings, &settings); decode_error != nil {
				t.Fatal(decode_error)
			}
			validation_error := http_destination.ValidateSettings(settings)
			if shared_case.Expected != nil {
				if validation_error != nil {
					t.Fatalf("the settings must be valid: %v", validation_error)
				}
				canonical, canonical_error := canonical_json.CanonicalJSON(settings)
				if canonical_error != nil {
					t.Fatal(canonical_error)
				}
				if string(canonical) != *shared_case.Expected {
					t.Fatalf("the canonical JSON is\n%s\nexpected\n%s", canonical, *shared_case.Expected)
				}
				return
			}
			var typed_error *config.ValidationError
			if !errors.As(validation_error, &typed_error) {
				t.Fatalf("expected a *config.ValidationError, got %T %v", validation_error, validation_error)
			}
			if len(typed_error.Problems) != len(shared_case.Problems) {
				t.Fatalf("the problems are %+v, expected %+v", typed_error.Problems, shared_case.Problems)
			}
			for position, problem := range typed_error.Problems {
				if problem.Field != shared_case.Problems[position].Field || string(problem.Code) != shared_case.Problems[position].Code {
					t.Errorf("problem %d is %s/%s, expected %s/%s", position, problem.Field, problem.Code,
						shared_case.Problems[position].Field, shared_case.Problems[position].Code)
				}
			}
			// The new constructor refuses the same settings with the same error and builds no destination.
			destination, build_error := http_destination.NewHTTPDestination(settings, value_objects.DefaultLimits())
			if destination != nil || build_error == nil {
				t.Fatalf("an invalid setting must not build a destination")
			}
		})
	}
}
