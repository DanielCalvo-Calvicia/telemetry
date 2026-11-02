package http_client_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
)

// http_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const http_fixtures_folder = "../../../../../../../integration-tests/fixtures/http"

// load_cases reads a fixture file of the folder into target and fails when the folder or the file is missing.
func load_cases(t *testing.T, file_name string, target any) {
	t.Helper()
	entries, read_error := os.ReadDir(http_fixtures_folder)
	if read_error != nil {
		t.Fatalf("the folder of the shared cases is missing: %v", read_error)
	}
	if len(entries) == 0 {
		t.Fatal("the folder of the shared cases has no files")
	}
	content, file_error := os.ReadFile(filepath.Join(http_fixtures_folder, file_name))
	if file_error != nil {
		t.Fatal(file_error)
	}
	if decode_error := json.Unmarshal(content, target); decode_error != nil {
		t.Fatalf("%s: %v", file_name, decode_error)
	}
}

func Test_batch_body_shared_cases(t *testing.T) {
	var file struct {
		Cases []struct {
			Description string                   `json:"description"`
			Events      []value_objects.Envelope `json:"events"`
			Expected    string                   `json:"expected"`
		} `json:"cases"`
	}
	load_cases(t, "batch_body.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		t.Run(shared_case.Description, func(t *testing.T) {
			body, build_error := helpers.BuildBatchBody(shared_case.Events)
			if build_error != nil {
				t.Fatal(build_error)
			}
			if string(body) != shared_case.Expected {
				t.Fatalf("the body is\n%s\nexpected\n%s", body, shared_case.Expected)
			}
			compressed, compress_error := helpers.CompressGzip(body)
			if compress_error != nil {
				t.Fatal(compress_error)
			}
			gzip_reader, reader_error := gzip.NewReader(bytes.NewReader(compressed))
			if reader_error != nil {
				t.Fatal(reader_error)
			}
			plain, read_error := io.ReadAll(gzip_reader)
			if read_error != nil {
				t.Fatal(read_error)
			}
			if string(plain) != shared_case.Expected {
				t.Fatalf("the gunzipped body is not the expected text")
			}
		})
	}
}

func Test_response_classification_shared_cases(t *testing.T) {
	var file struct {
		Cases []struct {
			Description string `json:"description"`
			Status      int    `json:"status"`
			Class       string `json:"class"`
		} `json:"cases"`
	}
	load_cases(t, "response_classification.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		if got := helpers.ClassifyStatus(shared_case.Status); string(got) != shared_case.Class {
			t.Errorf("%s: status %d is %q, expected %q", shared_case.Description, shared_case.Status, got, shared_case.Class)
		}
	}
}

func Test_retry_after_shared_cases(t *testing.T) {
	var file struct {
		Cases []struct {
			Description     string `json:"description"`
			Header          string `json:"header"`
			Now             string `json:"now"`
			ExpectedSeconds *int64 `json:"expected_seconds"`
		} `json:"cases"`
	}
	load_cases(t, "retry_after.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		now, parse_error := time.Parse(time.RFC3339, shared_case.Now)
		if parse_error != nil {
			t.Fatal(parse_error)
		}
		milliseconds, ok := helpers.ParseRetryAfter(shared_case.Header, now)
		if shared_case.ExpectedSeconds == nil {
			if ok {
				t.Errorf("%s: expected no delay, got %d ms", shared_case.Description, milliseconds)
			}
			continue
		}
		if !ok || milliseconds != *shared_case.ExpectedSeconds*1000 {
			t.Errorf("%s: got %d ms (ok %v), expected %d s", shared_case.Description, milliseconds, ok, *shared_case.ExpectedSeconds)
		}
	}
}

func Test_backoff_shared_cases(t *testing.T) {
	var file struct {
		Cases []struct {
			Description string  `json:"description"`
			Attempt     int     `json:"attempt"`
			BaseMS      int     `json:"base_ms"`
			MaxMS       int     `json:"max_ms"`
			Random      float64 `json:"random"`
			ExpectedMS  int64   `json:"expected_ms"`
		} `json:"cases"`
	}
	load_cases(t, "backoff.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		if got := helpers.BackoffWithJitter(shared_case.Attempt, shared_case.BaseMS, shared_case.MaxMS, shared_case.Random); got != shared_case.ExpectedMS {
			t.Errorf("%s: got %d, expected %d", shared_case.Description, got, shared_case.ExpectedMS)
		}
	}
}

func Test_auth_header_shared_cases(t *testing.T) {
	var file struct {
		Cases []struct {
			Description string `json:"description"`
			Settings    struct {
				BearerToken  string `json:"bearer_token"`
				APIKeyHeader string `json:"api_key_header"`
				APIKeyValue  string `json:"api_key_value"`
			} `json:"settings"`
			Expected *struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"expected"`
		} `json:"cases"`
	}
	load_cases(t, "auth_header.json", &file)
	if len(file.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, shared_case := range file.Cases {
		name, value, ok := helpers.AuthHeader(shared_case.Settings.BearerToken, shared_case.Settings.APIKeyHeader, shared_case.Settings.APIKeyValue)
		if shared_case.Expected == nil {
			if ok {
				t.Errorf("%s: expected no header, got %s", shared_case.Description, name)
			}
			continue
		}
		if !ok || name != shared_case.Expected.Name || value != shared_case.Expected.Value {
			t.Errorf("%s: got %q %q (ok %v)", shared_case.Description, name, value, ok)
		}
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
			settings := http_client.DefaultSettings()
			if decode_error := json.Unmarshal(shared_case.Settings, &settings); decode_error != nil {
				t.Fatal(decode_error)
			}
			validation_error := http_client.ValidateSettings(settings)
			if shared_case.Expected != nil {
				canonical, canonical_error := canonical_json.CanonicalJSON(settings)
				if canonical_error != nil {
					t.Fatal(canonical_error)
				}
				if string(canonical) != *shared_case.Expected {
					t.Fatalf("the effective settings are\n%s\nexpected\n%s", canonical, *shared_case.Expected)
				}
				if validation_error != nil {
					t.Fatalf("expected valid settings, got: %v", validation_error)
				}
				return
			}
			var typed *config.ValidationError
			if !errors.As(validation_error, &typed) {
				t.Fatalf("expected a *config.ValidationError, got %v", validation_error)
			}
			if len(typed.Problems) != len(shared_case.Problems) {
				t.Fatalf("expected %d problems, got %v", len(shared_case.Problems), typed.Problems)
			}
			for position, expected := range shared_case.Problems {
				if typed.Problems[position].Field != expected.Field || typed.Problems[position].Code != expected.Code {
					t.Fatalf("problem %d is %s %s, expected %s %s", position, typed.Problems[position].Field, typed.Problems[position].Code, expected.Field, expected.Code)
				}
			}
		})
	}
}
