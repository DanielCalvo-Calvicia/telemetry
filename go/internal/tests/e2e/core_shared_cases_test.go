package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

// core_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const core_fixtures_folder = "../../../../integration-tests/fixtures/core"

type core_actor struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

type core_emit struct {
	EventType  string         `json:"event_type"`
	Severity   string         `json:"severity"`
	Data       map[string]any `json:"data"`
	OccurredAt string         `json:"occurred_at"`
	Attributes map[string]any `json:"attributes"`
	Actor      *core_actor    `json:"actor"`
}

type core_problem struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

type core_expected struct {
	Results    []core_send_result    `json:"results"`
	Deliveries map[string][][]string `json:"deliveries"`
	Shutdown   struct {
		Sent    int `json:"sent"`
		Spooled int `json:"spooled"`
		Dropped int `json:"dropped"`
	} `json:"shutdown"`
}

type core_fixture struct {
	Description string `json:"description"`
	Kind        string `json:"kind"`
	case_setup
	Emits    []core_emit `json:"emits"`
	NewError *struct {
		Problems []core_problem `json:"problems"`
	} `json:"new_error"`
	Expected *core_expected `json:"expected"`
}

func case_input(t *testing.T, emit core_emit) telemetry.NewEventInput {
	t.Helper()
	input := telemetry.NewEventInput{EventType: emit.EventType, Severity: telemetry.Severity(emit.Severity), Data: emit.Data}
	if emit.OccurredAt != "" {
		occurred_at := parse_instant(t, emit.OccurredAt)
		input.OccurredAt = &occurred_at
	}
	if emit.Attributes != nil {
		attributes, attributes_error := telemetry.NewEventAttributes(emit.Attributes)
		if attributes_error != nil {
			t.Fatal(attributes_error)
		}
		input.Attributes = &attributes
	}
	if emit.Actor != nil {
		var actor_options []telemetry.ActorOption
		if emit.Actor.Type != "" {
			actor_options = append(actor_options, telemetry.WithActorType(emit.Actor.Type))
		}
		if emit.Actor.Version != "" {
			actor_options = append(actor_options, telemetry.WithActorVersion(emit.Actor.Version))
		}
		actor, actor_error := telemetry.NewActorIdentity(emit.Actor.Kind, emit.Actor.ID, actor_options...)
		if actor_error != nil {
			t.Fatal(actor_error)
		}
		input.Actor = &actor
	}
	return input
}

func Test_every_shared_core_case_gives_the_same_result(t *testing.T) {
	file_paths, glob_error := filepath.Glob(filepath.Join(core_fixtures_folder, "*.json"))
	if glob_error != nil {
		t.Fatal(glob_error)
	}
	if len(file_paths) == 0 {
		t.Fatalf("no case in %s", core_fixtures_folder)
	}

	for _, file_path := range file_paths {
		if filepath.Base(file_path) == "public_names.json" {
			continue
		}
		t.Run(filepath.Base(file_path), func(t *testing.T) {
			content, read_error := os.ReadFile(file_path)
			if read_error != nil {
				t.Fatal(read_error)
			}
			var fixture core_fixture
			if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
				t.Fatal(decode_error)
			}
			if fixture.Kind != "core" {
				t.Fatalf("%s: the kind %q is not known (core)", fixture.Description, fixture.Kind)
			}

			core, readers, new_error := build_case_core(t, fixture.case_setup)

			if fixture.NewError != nil {
				validation_error, is_validation := new_error.(*telemetry.ValidationError)
				if !is_validation {
					t.Fatalf("%s: expected a *ValidationError, got %v", fixture.Description, new_error)
				}
				got_problems := make([]core_problem, 0, len(validation_error.Problems))
				for _, problem := range validation_error.Problems {
					got_problems = append(got_problems, core_problem{Field: problem.Field, Code: problem.Code})
				}
				if !reflect.DeepEqual(got_problems, fixture.NewError.Problems) {
					t.Fatalf("%s:\n got %v\nwant %v", fixture.Description, got_problems, fixture.NewError.Problems)
				}
				return
			}
			if new_error != nil {
				t.Fatalf("%s: New failed: %v", fixture.Description, new_error)
			}
			if fixture.Expected == nil {
				t.Fatalf("%s: the case has neither new_error nor expected", fixture.Description)
			}

			ctx := case_context(t, fixture.case_setup)

			got_results := make([]core_send_result, 0, len(fixture.Emits))
			for _, emit := range fixture.Emits {
				result := core.Emit(ctx, case_input(t, emit))
				got_results = append(got_results, core_send_result{Accepted: result.Accepted, Dropped: result.Dropped, Failed: result.Failed, Reason: result.Reason})
			}
			if !reflect.DeepEqual(got_results, fixture.Expected.Results) {
				t.Fatalf("%s: results\n got %v\nwant %v", fixture.Description, got_results, fixture.Expected.Results)
			}

			for port, want_sinks := range fixture.Expected.Deliveries {
				got_sinks := readers[port]
				if len(got_sinks) != len(want_sinks) {
					t.Fatalf("%s: port %s has %d sinks, expected %d", fixture.Description, port, len(got_sinks), len(want_sinks))
				}
				for sink_index, got_sink := range got_sinks {
					got_events := canonical_events(t, got_sink)
					want_events := want_sinks[sink_index]
					if want_events == nil {
						want_events = []string{}
					}
					if !reflect.DeepEqual(got_events, want_events) {
						t.Fatalf("%s: port %s sink %d\n got %q\nwant %q", fixture.Description, port, sink_index, got_events, want_events)
					}
				}
			}

			shutdown_result := core.Shutdown(context.Background())
			if shutdown_result.Sent != fixture.Expected.Shutdown.Sent || shutdown_result.Spooled != fixture.Expected.Shutdown.Spooled || shutdown_result.Dropped != fixture.Expected.Shutdown.Dropped {
				t.Fatalf("%s: shutdown got %+v, want %+v", fixture.Description, shutdown_result, fixture.Expected.Shutdown)
			}
		})
	}
}
