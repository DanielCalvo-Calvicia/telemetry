package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

// core_fixtures_folder is the folder of the cases that the Python tests read too (the same files).
const core_fixtures_folder = "../../../../integration-tests/fixtures/core"

type core_send_result struct {
	Accepted int    `json:"accepted"`
	Dropped  int    `json:"dropped"`
	Failed   int    `json:"failed"`
	Reason   string `json:"reason"`
}

type core_sink struct {
	Result *core_send_result `json:"result"`
}

type core_call_context struct {
	TraceID       string `json:"trace_id"`
	SpanID        string `json:"span_id"`
	ExecutionID   string `json:"execution_id"`
	CorrelationID string `json:"correlation_id"`
}

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
	Config      struct {
		Profile   string                     `json:"profile"`
		Overrides map[string]json.RawMessage `json:"overrides"`
	} `json:"config"`
	Clock struct {
		WallTime string `json:"wall_time"`
	} `json:"clock"`
	EventIDs    []string               `json:"event_ids"`
	Sinks       map[string][]core_sink `json:"sinks"`
	Spool       bool                   `json:"spool"`
	CallContext *core_call_context     `json:"call_context"`
	Emits       []core_emit            `json:"emits"`
	NewError    *struct {
		Problems []core_problem `json:"problems"`
	} `json:"new_error"`
	Expected *core_expected `json:"expected"`
}

// delivery_reader returns the events a fake sink received, flattened from its batches, as canonical JSON.
type delivery_reader interface {
	Batches() [][]telemetry.Envelope
}

func case_config(t *testing.T, profile string, overrides map[string]json.RawMessage) telemetry.Config {
	t.Helper()
	profiles := map[string]func() telemetry.Config{
		"microservice":   telemetry.Microservice,
		"cronjob":        telemetry.Cronjob,
		"worker":         telemetry.Worker,
		"robotics":       telemetry.Robotics,
		"offline_device": telemetry.OfflineDevice,
	}
	make_profile, known := profiles[profile]
	if !known {
		t.Fatalf("the profile %q is not known", profile)
	}
	base := make_profile()
	overrides_text, marshal_error := json.Marshal(overrides)
	if marshal_error != nil {
		t.Fatal(marshal_error)
	}
	if decode_error := json.Unmarshal(overrides_text, &base); decode_error != nil {
		t.Fatal(decode_error)
	}
	return base
}

func parse_instant(t *testing.T, text string) telemetry.UTCInstant {
	t.Helper()
	parsed_time, parse_error := time.Parse(time.RFC3339Nano, text)
	if parse_error != nil {
		t.Fatalf("the time %q: %v", text, parse_error)
	}
	instant, instant_error := telemetry.NewUTCInstant(parsed_time.UTC())
	if instant_error != nil {
		t.Fatal(instant_error)
	}
	return instant
}

// case_sinks builds the fake sinks of the case and returns the options that pass them, and the readers by port.
func case_sinks(t *testing.T, sinks map[string][]core_sink) ([]telemetry.Option, map[string][]delivery_reader) {
	t.Helper()
	var options []telemetry.Option
	readers := map[string][]delivery_reader{}
	for port, entries := range sinks {
		for _, entry := range entries {
			result := telemetry.SendResult{}
			if entry.Result != nil {
				result = telemetry.SendResult{Accepted: entry.Result.Accepted, Dropped: entry.Result.Dropped, Failed: entry.Result.Failed, Reason: entry.Result.Reason}
			}
			switch port {
			case "log":
				sink := &fakes.FakeLogSink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithLogSink(sink))
				readers[port] = append(readers[port], sink)
			case "event":
				sink := &fakes.FakeEventSink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithEventSink(sink))
				readers[port] = append(readers[port], sink)
			case "metrics":
				sink := &fakes.FakeMetricsSink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithMetricsSink(sink))
				readers[port] = append(readers[port], sink)
			case "trace":
				sink := &fakes.FakeTraceSink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithTraceSink(sink))
				readers[port] = append(readers[port], sink)
			case "device_telemetry":
				sink := &fakes.FakeDeviceTelemetrySink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithDeviceTelemetrySink(sink))
				readers[port] = append(readers[port], sink)
			case "ai_telemetry":
				sink := &fakes.FakeAITelemetrySink{}
				if entry.Result != nil {
					sink.SetResult(result)
				}
				options = append(options, telemetry.WithAITelemetrySink(sink))
				readers[port] = append(readers[port], sink)
			default:
				t.Fatalf("the sink port %q is not known", port)
			}
		}
	}
	return options, readers
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

			event_ids := make([]telemetry.EventID, 0, len(fixture.EventIDs))
			for _, event_id_text := range fixture.EventIDs {
				event_id, id_error := telemetry.NewEventID(event_id_text)
				if id_error != nil {
					t.Fatal(id_error)
				}
				event_ids = append(event_ids, event_id)
			}
			options, readers := case_sinks(t, fixture.Sinks)
			options = append(options,
				telemetry.WithClock(fakes.NewFakeClock(parse_instant(t, fixture.Clock.WallTime))),
				telemetry.WithIDGenerator(fakes.NewFakeIDGenerator(event_ids...)),
			)
			if fixture.Spool {
				options = append(options, telemetry.WithLocalSpool(&fakes.FakeLocalSpool{}))
			}

			core, new_error := telemetry.New(case_config(t, fixture.Config.Profile, fixture.Config.Overrides), options...)

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

			ctx := context.Background()
			if fixture.CallContext != nil {
				call_context, context_error := telemetry.NewCallContext(
					telemetry.WithTraceID(fixture.CallContext.TraceID),
					telemetry.WithSpanID(fixture.CallContext.SpanID),
					telemetry.WithExecutionID(fixture.CallContext.ExecutionID),
					telemetry.WithCorrelationID(fixture.CallContext.CorrelationID),
				)
				if context_error != nil {
					t.Fatal(context_error)
				}
				ctx = telemetry.WithCallContext(ctx, call_context)
			}

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
					got_events := []string{}
					for _, batch := range got_sink.Batches() {
						for _, envelope := range batch {
							canonical_text, canonical_error := canonical_json.CanonicalJSON(envelope)
							if canonical_error != nil {
								t.Fatal(canonical_error)
							}
							got_events = append(got_events, string(canonical_text))
						}
					}
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
