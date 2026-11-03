package aws_http_test

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
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http"
)

// fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const fixtures_folder = "../../../../../../integration-tests/fixtures/aws_http"

// load_cases reads a fixture file of the folder into target and fails when the folder or the file is missing.
func load_cases(t *testing.T, file_name string, target any) {
	t.Helper()
	entries, read_error := os.ReadDir(fixtures_folder)
	if read_error != nil {
		t.Fatalf("the folder of the shared cases is missing: %v", read_error)
	}
	if len(entries) == 0 {
		t.Fatal("the folder of the shared cases has no files")
	}
	content, file_error := os.ReadFile(filepath.Join(fixtures_folder, file_name))
	if file_error != nil {
		t.Fatal(file_error)
	}
	if decode_error := json.Unmarshal(content, target); decode_error != nil {
		t.Fatalf("%s: %v", file_name, decode_error)
	}
}

func Test_add_log_request_shared_cases(t *testing.T) {
	var file struct {
		LogGroupName  string `json:"log_group_name"`
		LogStreamName string `json:"log_stream_name"`
		Cases         []struct {
			Name            string                 `json:"name"`
			Event           value_objects.Envelope `json:"event"`
			ExpectedRequest string                 `json:"expected_request"`
			ExpectedMessage string                 `json:"expected_message"`
			ExpectedSkipped bool                   `json:"expected_skipped"`
		} `json:"cases"`
	}
	load_cases(t, "add_log_request.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		t.Run(shared_case.Name, func(t *testing.T) {
			receiver := new_receiver(t, 200)
			settings := settings_for_server(t, receiver.server.URL)
			settings.LogGroupName = file.LogGroupName
			settings.LogStreamName = file.LogStreamName
			destination := new_destination(t, settings)

			delivery := destination.Deliver(context.Background(), []value_objects.Envelope{shared_case.Event})

			requests := receiver.received()
			if shared_case.ExpectedSkipped {
				if len(requests) != 0 {
					t.Fatalf("a skipped event makes no request, got %d", len(requests))
				}
				if delivery.Skipped != 1 || delivery.Sent != 0 || delivery.Result.Accepted != 1 || delivery.Result.Dropped != 0 ||
					delivery.Result.Failed != 0 || delivery.Result.Reason != "" {
					t.Fatalf("%+v", delivery)
				}
				if delivery.Outcome.StatusCode != 0 || delivery.Outcome.Reason != "" {
					t.Fatalf("no request was made, the outcome must be empty: %+v", delivery.Outcome)
				}
				return
			}
			if len(requests) != 1 {
				t.Fatalf("exactly one request expected, got %d", len(requests))
			}
			if requests[0].Body != shared_case.ExpectedRequest {
				t.Fatalf("the body is\n%s\nexpected\n%s", requests[0].Body, shared_case.ExpectedRequest)
			}
			if requests[0].Method != "POST" || requests[0].Path != endpoint_path {
				t.Errorf("the request is %s %s", requests[0].Method, requests[0].Path)
			}
			if requests[0].Headers.Get("Content-Type") != "application/json" {
				t.Errorf("the Content-Type is %q", requests[0].Headers.Get("Content-Type"))
			}
			if requests[0].Headers.Get("Content-Encoding") != "" {
				t.Errorf("the body must not be compressed: %q", requests[0].Headers.Get("Content-Encoding"))
			}
			if delivery.Skipped != 0 || delivery.Sent != 1 || delivery.Result.Accepted != 1 || delivery.Result.Reason != "" {
				t.Fatalf("%+v", delivery)
			}
			if delivery.Outcome.StatusCode != 200 {
				t.Fatalf("the outcome is the one of the request: %+v", delivery.Outcome)
			}

			// The message inside the body is the expected message, and the body is valid JSON with the four keys.
			var decoded struct {
				Message   string `json:"message"`
				Timestamp int64  `json:"timestamp"`
			}
			if decode_error := json.Unmarshal([]byte(requests[0].Body), &decoded); decode_error != nil {
				t.Fatal(decode_error)
			}
			if decoded.Message != shared_case.ExpectedMessage {
				t.Fatalf("the message is\n%s\nexpected\n%s", decoded.Message, shared_case.ExpectedMessage)
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
			settings := aws_http.DefaultSettings()
			if decode_error := json.Unmarshal(shared_case.Settings, &settings); decode_error != nil {
				t.Fatal(decode_error)
			}
			validation_error := aws_http.ValidateSettings(settings)
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
				if _, build_error := aws_http.NewDestination(settings); build_error != nil {
					t.Fatalf("valid settings must build a destination: %v", build_error)
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
				if problem.Field != shared_case.Problems[position].Field || problem.Code != shared_case.Problems[position].Code {
					t.Errorf("problem %d is %s/%s, expected %s/%s", position, problem.Field, problem.Code,
						shared_case.Problems[position].Field, shared_case.Problems[position].Code)
				}
			}
			// The constructor refuses the same settings with the same error and builds no destination.
			destination, build_error := aws_http.NewDestination(settings)
			if destination != nil || build_error == nil {
				t.Fatalf("an invalid setting must not build a destination")
			}
		})
	}
}
