package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

const building_fixtures_folder = "../../../../../integration-tests/fixtures/building"

type building_clock_info struct {
	Source *string `json:"source"`
	Synced *bool   `json:"synced"`
}

type building_resource struct {
	ServiceName    string  `json:"service_name"`
	InstanceID     string  `json:"instance_id"`
	Environment    *string `json:"environment"`
	ServiceVersion *string `json:"service_version"`
	Host           *string `json:"host"`
	TimeZone       *string `json:"time_zone"`
}

type building_setup struct {
	WallTime  string               `json:"wall_time"`
	ClockInfo *building_clock_info `json:"clock_info"`
	EventID   string               `json:"event_id"`
	Resource  building_resource    `json:"resource"`
}

type building_actor struct {
	Kind    string  `json:"kind"`
	ID      string  `json:"id"`
	Type    *string `json:"type"`
	Version *string `json:"version"`
}

type building_context struct {
	TraceID       *string `json:"trace_id"`
	SpanID        *string `json:"span_id"`
	ExecutionID   *string `json:"execution_id"`
	CorrelationID *string `json:"correlation_id"`
}

type building_input struct {
	EventType  string            `json:"event_type"`
	Severity   string            `json:"severity"`
	Data       map[string]any    `json:"data"`
	OccurredAt *string           `json:"occurred_at"`
	Actor      *building_actor   `json:"actor"`
	Context    *building_context `json:"context"`
	Attributes map[string]any    `json:"attributes"`
}

type building_fixture struct {
	Description     string         `json:"description"`
	Kind            string         `json:"kind"`
	Setup           building_setup `json:"setup"`
	Input           building_input `json:"input"`
	Expected        *string        `json:"expected"`
	ErrorCode       *string        `json:"error_code"`
	MessageContains []string       `json:"message_contains"`
}

func utc_from_text(test *testing.T, text string) value_objects.UTCInstant {
	test.Helper()
	parsed_time, parse_error := time.Parse(time.RFC3339Nano, text)
	if parse_error != nil {
		test.Fatalf("bad time %q: %v", text, parse_error)
	}
	instant, instant_error := value_objects.NewUTCInstant(parsed_time.UTC())
	if instant_error != nil {
		test.Fatal(instant_error)
	}
	return instant
}

func resource_from(test *testing.T, resource building_resource) value_objects.ResourceIdentity {
	test.Helper()
	var options []value_objects.ResourceOption
	if resource.Environment != nil {
		options = append(options, value_objects.WithEnvironment(*resource.Environment))
	}
	if resource.ServiceVersion != nil {
		options = append(options, value_objects.WithServiceVersion(*resource.ServiceVersion))
	}
	if resource.Host != nil {
		options = append(options, value_objects.WithHost(*resource.Host))
	}
	if resource.TimeZone != nil {
		options = append(options, value_objects.WithTimeZone(*resource.TimeZone))
	}
	built, build_error := value_objects.NewResourceIdentity(resource.ServiceName, resource.InstanceID, options...)
	if build_error != nil {
		test.Fatal(build_error)
	}
	return built
}

func new_input_from(test *testing.T, input building_input) dtos.NewEventInput {
	test.Helper()
	severity, severity_error := value_objects.NewSeverity(input.Severity)
	if severity_error != nil {
		test.Fatal(severity_error)
	}
	new_input := dtos.NewEventInput{EventType: input.EventType, Severity: severity, Data: input.Data}
	if input.OccurredAt != nil {
		occurred_at := utc_from_text(test, *input.OccurredAt)
		new_input.OccurredAt = &occurred_at
	}
	if input.Actor != nil {
		var options []value_objects.ActorOption
		if input.Actor.Type != nil {
			options = append(options, value_objects.WithActorType(*input.Actor.Type))
		}
		if input.Actor.Version != nil {
			options = append(options, value_objects.WithActorVersion(*input.Actor.Version))
		}
		kind, kind_error := value_objects.NewActorKind(input.Actor.Kind)
		if kind_error != nil {
			test.Fatal(kind_error)
		}
		actor, actor_error := value_objects.NewActorIdentity(kind, input.Actor.ID, options...)
		if actor_error != nil {
			test.Fatal(actor_error)
		}
		new_input.Actor = &actor
	}
	if input.Context != nil {
		var options []value_objects.ContextOption
		if input.Context.TraceID != nil {
			options = append(options, value_objects.WithTraceID(*input.Context.TraceID))
		}
		if input.Context.SpanID != nil {
			options = append(options, value_objects.WithSpanID(*input.Context.SpanID))
		}
		if input.Context.ExecutionID != nil {
			options = append(options, value_objects.WithExecutionID(*input.Context.ExecutionID))
		}
		if input.Context.CorrelationID != nil {
			options = append(options, value_objects.WithCorrelationID(*input.Context.CorrelationID))
		}
		correlation_context, context_error := value_objects.NewCorrelationContext(options...)
		if context_error != nil {
			test.Fatal(context_error)
		}
		new_input.Context = &correlation_context
	}
	if input.Attributes != nil {
		attributes, attributes_error := value_objects.NewEventAttributes(input.Attributes)
		if attributes_error != nil {
			test.Fatal(attributes_error)
		}
		new_input.Attributes = &attributes
	}
	return new_input
}

func service_from(test *testing.T, setup building_setup) *services.EventBuilderService {
	test.Helper()
	fake_clock := fakes.NewFakeClock(utc_from_text(test, setup.WallTime))
	if setup.ClockInfo != nil {
		var clock_info dtos.ClockInfo
		if setup.ClockInfo.Source != nil {
			clock_info.Source, clock_info.SourceKnown = *setup.ClockInfo.Source, true
		}
		if setup.ClockInfo.Synced != nil {
			clock_info.Synced, clock_info.SyncedKnown = *setup.ClockInfo.Synced, true
		}
		fake_clock.SetClockInfo(clock_info)
	}
	event_id, id_error := value_objects.NewEventID(setup.EventID)
	if id_error != nil {
		test.Fatal(id_error)
	}
	return services.NewEventBuilderService(
		fake_clock,
		fakes.NewFakeIDGenerator(event_id),
		fakes.NewFakeConfiguration(dtos.Configuration{Resource: resource_from(test, setup.Resource)}),
		event_validation.ValidateEvent,
	)
}

func read_building_fixture(test *testing.T, path string) building_fixture {
	test.Helper()
	content, read_error := os.ReadFile(path)
	if read_error != nil {
		test.Fatal(read_error)
	}
	var fixture building_fixture
	if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
		test.Fatalf("not a valid fixture: %v", decode_error)
	}
	return fixture
}

func TestTheBuildingFixturesGiveTheExpectedEnvelopesAndErrors(test *testing.T) {
	paths, glob_error := filepath.Glob(filepath.Join(building_fixtures_folder, "*.json"))
	if glob_error != nil || len(paths) == 0 {
		test.Fatalf("no building fixtures found: %v", glob_error)
	}
	for _, path := range paths {
		test.Run(filepath.Base(path), func(test *testing.T) {
			fixture := read_building_fixture(test, path)
			if fixture.Kind != "build" {
				test.Fatalf("unknown kind %q", fixture.Kind)
			}
			service := service_from(test, fixture.Setup)
			built, build_error := service.Build(context.Background(), new_input_from(test, fixture.Input))
			if fixture.ErrorCode != nil {
				var classified *domain.ClassifiedError
				if !errors.As(build_error, &classified) || classified.Code != *fixture.ErrorCode {
					test.Fatalf("want code %s, got %v", *fixture.ErrorCode, build_error)
				}
				for _, piece := range fixture.MessageContains {
					if !strings.Contains(build_error.Error(), piece) {
						test.Errorf("message %q lacks %q", build_error.Error(), piece)
					}
				}
				return
			}
			if fixture.Expected == nil {
				test.Fatal("a build fixture needs expected or error_code")
			}
			if build_error != nil {
				test.Fatalf("build failed: %v", build_error)
			}
			got, serialize_error := canonical_json.CanonicalJSON(built)
			if serialize_error != nil {
				test.Fatal(serialize_error)
			}
			if string(got) != *fixture.Expected {
				test.Fatalf("bytes differ\n got: %s\nwant: %s", got, *fixture.Expected)
			}
		})
	}
}
