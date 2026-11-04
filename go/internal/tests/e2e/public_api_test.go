package e2e_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/waiting"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

func public_config() telemetry.Config {
	cfg := fast_delivery(telemetry.Microservice())
	cfg.Resource.ServiceName = "sensor-hub"
	return cfg
}

func public_input() telemetry.NewEventInput {
	return telemetry.NewEventInput{
		EventType: "log.entry.recorded",
		Severity:  telemetry.SeverityInfo,
		Data:      map[string]any{"category": "http", "message": "started"},
	}
}

func Test_new_builds_a_core_with_the_default_clock_and_ids_and_delivers_an_event(t *testing.T) {
	sink := &fakes.FakeLogSink{}
	core, new_error := telemetry.New(public_config(), telemetry.WithLogSink(sink))
	if new_error != nil {
		t.Fatal(new_error)
	}

	result := core.Emit(context.Background(), public_input())

	if result != (telemetry.EmitResult{Accepted: 1}) {
		t.Fatalf("result = %+v", result)
	}
	waiting.Eventually(t, waiting.DefaultTimeout, "the sink receives the event", func() bool { return sink.Calls() == 1 })
	// The sink was called, but the worker counts the batch a moment later; a Shutdown in between would report the event
	// still in flight as sent. The public diagnostics say when the delivery is counted.
	waiting.Eventually(t, waiting.DefaultTimeout, "the delivery is counted", func() bool {
		destinations := core.Diagnostics().Destinations
		return len(destinations) == 1 && destinations[0].Delivered == 1
	})
	if shutdown_result := core.Shutdown(context.Background()); shutdown_result != (telemetry.ShutdownResult{}) {
		t.Fatalf("shutdown = %+v", shutdown_result)
	}
}

func Test_new_with_an_invalid_configuration_returns_a_validation_error(t *testing.T) {
	core, new_error := telemetry.New(telemetry.Microservice())

	var validation_error *telemetry.ValidationError
	if core != nil || !errors.As(new_error, &validation_error) || validation_error.Problems[0].Field != "resource.service_name" {
		t.Fatalf("got %v", new_error)
	}
}

func Test_a_nil_telemetry_drops_the_event(t *testing.T) {
	var core *telemetry.Telemetry
	if result := core.Emit(context.Background(), public_input()); result.Dropped != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func Test_the_call_context_helpers_work_through_the_public_package(t *testing.T) {
	call_context, context_error := telemetry.NewCallContext(telemetry.WithApplication("shop"), telemetry.WithInstanceID("i-1"),
		telemetry.WithTraceID("0af7651916cd43dd8448eb211c80319c"), telemetry.WithSpanID("b7ad6b7169203331"),
		telemetry.WithExecutionID("run-1"), telemetry.WithCorrelationID("order-1"))
	if context_error != nil {
		t.Fatal(context_error)
	}
	ctx := telemetry.WithCallContext(context.Background(), call_context)
	deeper_ctx, deeper := telemetry.NextIndentation(ctx)
	carried, found := telemetry.FromContext(deeper_ctx)
	if !deeper || !found || carried.Indentation() != call_context.Indentation()+1 {
		t.Fatalf("found = %v, deeper = %v, indentation = %d", found, deeper, carried.Indentation())
	}
}

func Test_the_value_helpers_accept_valid_values_and_refuse_invalid_ones(t *testing.T) {
	actor, actor_error := telemetry.NewActorIdentity("subsystem", "billing", telemetry.WithActorType("api"), telemetry.WithActorVersion("1.0"))
	if actor_error != nil || actor.ID() != "billing" {
		t.Fatalf("actor = %+v, error = %v", actor, actor_error)
	}
	if _, kind_error := telemetry.NewActorIdentity("robot-overlord", "x"); kind_error == nil {
		t.Fatal("an unknown actor kind must be refused")
	}
	if _, attributes_error := telemetry.NewEventAttributes(map[string]any{"nested": map[string]any{}}); attributes_error == nil {
		t.Fatal("a nested attribute must be refused")
	}
}
