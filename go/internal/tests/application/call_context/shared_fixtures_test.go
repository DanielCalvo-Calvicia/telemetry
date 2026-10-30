package call_context_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/call_context"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/mappers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// call_context_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const call_context_fixtures_folder = "../../../../../integration-tests/fixtures/call_context"

func read_fixture(t *testing.T, file_name string, fixture any) {
	t.Helper()
	if _, stat_error := os.Stat(call_context_fixtures_folder); stat_error != nil {
		t.Fatalf("the folder %s is missing: %v", call_context_fixtures_folder, stat_error)
	}
	content, read_error := os.ReadFile(filepath.Join(call_context_fixtures_folder, file_name))
	if read_error != nil {
		t.Fatal(read_error)
	}
	if decode_error := json.Unmarshal(content, fixture); decode_error != nil {
		t.Fatal(decode_error)
	}
}

func Test_every_shared_id_case_is_accepted_or_rejected_as_the_table_says(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Description string `json:"description"`
			Input       struct {
				Field string `json:"field"`
				Value string `json:"value"`
			} `json:"input"`
			Valid bool `json:"valid"`
		} `json:"cases"`
	}
	read_fixture(t, "ids.json", &fixture)
	if len(fixture.Cases) == 0 {
		t.Fatal("the table ids.json has no case")
	}

	for _, fixture_case := range fixture.Cases {
		t.Run(fixture_case.Description, func(t *testing.T) {
			options := map[string]call_context.Option{
				"trace_id":       call_context.WithTraceID(fixture_case.Input.Value),
				"span_id":        call_context.WithSpanID(fixture_case.Input.Value),
				"execution_id":   call_context.WithExecutionID(fixture_case.Input.Value),
				"correlation_id": call_context.WithCorrelationID(fixture_case.Input.Value),
				"application":    call_context.WithApplication(fixture_case.Input.Value),
				"instance_id":    call_context.WithInstanceID(fixture_case.Input.Value),
			}
			option, field_known := options[fixture_case.Input.Field]
			if !field_known {
				t.Fatalf("the field %q is not known", fixture_case.Input.Field)
			}

			built_context, build_error := call_context.NewCallContext(option)

			if fixture_case.Valid {
				if build_error != nil {
					t.Fatalf("unexpected error: %v", build_error)
				}
				getters := map[string]string{
					"trace_id":       built_context.TraceID(),
					"span_id":        built_context.SpanID(),
					"execution_id":   built_context.ExecutionID(),
					"correlation_id": built_context.CorrelationID(),
					"application":    built_context.Application(),
					"instance_id":    built_context.InstanceID(),
				}
				if getters[fixture_case.Input.Field] != fixture_case.Input.Value {
					t.Errorf("the getter gave %q, want %q", getters[fixture_case.Input.Field], fixture_case.Input.Value)
				}
				return
			}
			var classified *domain.ClassifiedError
			if !errors.As(build_error, &classified) {
				t.Fatalf("error = %v, want a *domain.ClassifiedError", build_error)
			}
			if code := value_objects.CodeOf(build_error); code != value_objects.ErrorCodeEventSchemaInvalid {
				t.Errorf("code = %q, want event_schema_invalid", code)
			}
			if len(strings.TrimSpace(fixture_case.Input.Value)) >= 6 && strings.Contains(build_error.Error(), fixture_case.Input.Value) {
				t.Errorf("the message carries the value: %q", build_error.Error())
			}
		})
	}
}

func Test_every_shared_mapping_case_gives_the_same_correlation_context(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Description string `json:"description"`
			Input       struct {
				TraceID       string `json:"trace_id"`
				SpanID        string `json:"span_id"`
				ExecutionID   string `json:"execution_id"`
				CorrelationID string `json:"correlation_id"`
				Application   string `json:"application"`
				InstanceID    string `json:"instance_id"`
			} `json:"input"`
			Expected *string `json:"expected"`
		} `json:"cases"`
	}
	read_fixture(t, "mapping.json", &fixture)
	if len(fixture.Cases) == 0 {
		t.Fatal("the table mapping.json has no case")
	}

	for _, fixture_case := range fixture.Cases {
		t.Run(fixture_case.Description, func(t *testing.T) {
			built_context, build_error := call_context.NewCallContext(
				call_context.WithTraceID(fixture_case.Input.TraceID),
				call_context.WithSpanID(fixture_case.Input.SpanID),
				call_context.WithExecutionID(fixture_case.Input.ExecutionID),
				call_context.WithCorrelationID(fixture_case.Input.CorrelationID),
				call_context.WithApplication(fixture_case.Input.Application),
				call_context.WithInstanceID(fixture_case.Input.InstanceID),
			)
			if build_error != nil {
				t.Fatalf("unexpected error: %v", build_error)
			}

			correlation_context, has_context, map_error := call_context.ToCorrelationContext(built_context)

			if map_error != nil {
				t.Fatalf("unexpected error: %v", map_error)
			}
			if fixture_case.Expected == nil {
				if has_context {
					t.Error("want no context")
				}
				return
			}
			if !has_context {
				t.Fatal("want a context")
			}
			wire_context := mappers.MapContext(&correlation_context)
			canonical, canonical_error := canonical_json.CanonicalJSON(wire_context)
			if canonical_error != nil {
				t.Fatal(canonical_error)
			}
			if string(canonical) != *fixture_case.Expected {
				t.Errorf("got %s, want %s", canonical, *fixture_case.Expected)
			}
		})
	}
}
