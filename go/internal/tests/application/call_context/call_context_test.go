package call_context_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/call_context"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

const (
	valid_trace_id = "0af7651916cd43dd8448eb211c80319c"
	valid_span_id  = "b7ad6b7169203331"
)

func must_call_context(t *testing.T, options ...call_context.Option) call_context.CallContext {
	t.Helper()
	built_context, build_error := call_context.NewCallContext(options...)
	if build_error != nil {
		t.Fatalf("unexpected error: %v", build_error)
	}
	return built_context
}

// ----- ported from the old call_depth tests ----------------------------------

func Test_a_context_without_call_context_has_none(t *testing.T) {
	found_context, found := call_context.FromContext(context.Background())

	if found || found_context != (call_context.CallContext{}) {
		t.Errorf("FromContext() = %+v, %v; want the zero value and false", found_context, found)
	}
}

func Test_with_call_context_stores_and_reads_back_every_value(t *testing.T) {
	stored_context := must_call_context(t,
		call_context.WithApplication("app"), call_context.WithInstanceID("instance"),
		call_context.WithExecutionID("run-1"), call_context.WithCorrelationID("request"),
		call_context.WithTraceID(valid_trace_id), call_context.WithSpanID(valid_span_id))
	ctx := call_context.WithCallContext(context.Background(), stored_context)

	found_context, found := call_context.FromContext(ctx)

	if !found || found_context != stored_context {
		t.Fatalf("FromContext() = %+v, %v; want %+v", found_context, found, stored_context)
	}
	if found_context.Application() != "app" || found_context.InstanceID() != "instance" ||
		found_context.ExecutionID() != "run-1" || found_context.CorrelationID() != "request" ||
		found_context.TraceID() != valid_trace_id || found_context.SpanID() != valid_span_id ||
		found_context.Indentation() != call_context.StartingIndentation {
		t.Errorf("a getter did not give the stored value: %+v", found_context)
	}
}

func Test_new_request_context_starts_a_request(t *testing.T) {
	id_generator := fakes.NewFakeIDGenerator()

	ctx, request_error := call_context.NewRequestContext(context.Background(), id_generator, "app", "instance")
	if request_error != nil {
		t.Fatalf("unexpected error: %v", request_error)
	}

	found_context, found := call_context.FromContext(ctx)
	if !found {
		t.Fatal("the request context must carry a call context")
	}
	if found_context.Application() != "app" || found_context.InstanceID() != "instance" ||
		found_context.CorrelationID() != "01900000-0000-7000-8000-000000000001" ||
		found_context.Indentation() != call_context.StartingIndentation {
		t.Errorf("FromContext() = %+v, unexpected values", found_context)
	}
	if found_context.ExecutionID() != "" || found_context.TraceID() != "" || found_context.SpanID() != "" {
		t.Errorf("a request context must not invent an execution, a trace or a span: %+v", found_context)
	}
	if call_context.StartingIndentation != 1 {
		t.Errorf("StartingIndentation = %d, want 1", call_context.StartingIndentation)
	}
}

func Test_new_request_context_when_the_id_cannot_be_generated(t *testing.T) {
	random_source_failure := errors.New("random source unavailable")
	id_generator := fakes.NewFakeIDGenerator()
	id_generator.FailWith(random_source_failure)
	original_context := context.Background()

	ctx, request_error := call_context.NewRequestContext(original_context, id_generator, "app", "instance")

	if !errors.Is(request_error, call_context.ErrCallContextIDFailed) || !errors.Is(request_error, application.ErrCallContextIDFailed) {
		t.Errorf("error = %v, want ErrCallContextIDFailed", request_error)
	}
	if !errors.Is(request_error, random_source_failure) {
		t.Errorf("error = %v, want the root cause kept", request_error)
	}
	if code := value_objects.CodeOf(request_error); code != value_objects.ErrorCodeEventBuildFailed {
		t.Errorf("code = %q, want event_build_failed", code)
	}
	var classified *domain.ClassifiedError
	if !errors.As(request_error, &classified) {
		t.Error("the error must be a *domain.ClassifiedError")
	}
	if ctx != original_context {
		t.Error("a failed request context must return the context unchanged")
	}
	if _, found := call_context.FromContext(ctx); found {
		t.Error("a failed request context must not carry a call context")
	}
}

func Test_new_request_context_with_an_invalid_application_returns_the_validation_error(t *testing.T) {
	original_context := context.Background()

	ctx, request_error := call_context.NewRequestContext(original_context, fakes.NewFakeIDGenerator(), "  ", "instance")

	if !errors.Is(request_error, call_context.ErrCallContextFieldInvalid) {
		t.Errorf("error = %v, want ErrCallContextFieldInvalid", request_error)
	}
	if ctx != original_context {
		t.Error("a failed request context must return the context unchanged")
	}
}

func Test_next_indentation_goes_one_layer_deeper_without_touching_the_original(t *testing.T) {
	original_context := call_context.WithCallContext(context.Background(), must_call_context(t, call_context.WithCorrelationID("request")))

	deeper_context, ok := call_context.NextIndentation(original_context)
	if !ok {
		t.Fatal("NextIndentation must succeed when there is a call context")
	}
	deepest_context, _ := call_context.NextIndentation(deeper_context)

	original_call_context, _ := call_context.FromContext(original_context)
	deeper_call_context, _ := call_context.FromContext(deeper_context)
	deepest_call_context, _ := call_context.FromContext(deepest_context)
	if original_call_context.Indentation() != 1 || deeper_call_context.Indentation() != 2 || deepest_call_context.Indentation() != 3 {
		t.Errorf("indentations = %d, %d, %d; want 1, 2, 3",
			original_call_context.Indentation(), deeper_call_context.Indentation(), deepest_call_context.Indentation())
	}
	if deepest_call_context.CorrelationID() != "request" {
		t.Error("going deeper must keep the rest of the call context")
	}
}

func Test_next_indentation_without_call_context_returns_the_context_unchanged(t *testing.T) {
	original_context := context.Background()

	returned_context, ok := call_context.NextIndentation(original_context)

	if ok || returned_context != original_context {
		t.Errorf("NextIndentation() = %v, %v; want the same context and false", returned_context, ok)
	}
}

// ----- new behavior ----------------------------------------------------------

func Test_a_call_context_with_nothing_set_gives_empty_values_and_no_error(t *testing.T) {
	empty_context := must_call_context(t)

	if empty_context.Application() != "" || empty_context.InstanceID() != "" || empty_context.ExecutionID() != "" ||
		empty_context.CorrelationID() != "" || empty_context.TraceID() != "" || empty_context.SpanID() != "" {
		t.Errorf("unexpected values: %+v", empty_context)
	}
	if empty_context.Indentation() != call_context.StartingIndentation {
		t.Errorf("Indentation() = %d, want %d", empty_context.Indentation(), call_context.StartingIndentation)
	}
	if empty_texts := must_call_context(t, call_context.WithTraceID(""), call_context.WithApplication("")); empty_texts != empty_context {
		t.Errorf("empty texts must count as not set: %+v", empty_texts)
	}
}

func Test_from_context_with_a_nil_context_does_not_panic(t *testing.T) {
	//lint:ignore SA1012 the test checks that a nil context is tolerated
	found_context, found := call_context.FromContext(nil) //nolint:staticcheck

	if found || found_context != (call_context.CallContext{}) {
		t.Errorf("FromContext(nil) = %+v, %v; want the zero value and false", found_context, found)
	}
}

func Test_every_invalid_field_is_reported_together_in_order_and_never_with_its_value(t *testing.T) {
	secret_value := "sk-live-SECRET\n"
	_, build_error := call_context.NewCallContext(
		call_context.WithSpanID("ZZ-"+secret_value),
		call_context.WithTraceID("00000000000000000000000000000000"),
		call_context.WithCorrelationID(secret_value),
		call_context.WithExecutionID("   "),
		call_context.WithInstanceID(secret_value),
		call_context.WithApplication("\t"),
	)

	if build_error == nil {
		t.Fatal("want an error")
	}
	var classified *domain.ClassifiedError
	if !errors.As(build_error, &classified) {
		t.Fatalf("error = %T, want *domain.ClassifiedError", build_error)
	}
	if code := value_objects.CodeOf(build_error); code != value_objects.ErrorCodeEventSchemaInvalid {
		t.Errorf("code = %q, want event_schema_invalid", code)
	}
	if !errors.Is(build_error, call_context.ErrCallContextFieldInvalid) || !errors.Is(build_error, call_context.ErrCallContextIDAllZeros) {
		t.Errorf("error = %v, want both sentinels", build_error)
	}
	message := build_error.Error()
	last_position := -1
	for _, field := range []string{"application:", "instance_id:", "execution_id:", "correlation_id:", "trace_id:", "span_id:"} {
		position := strings.Index(message, field)
		if position < 0 || position < last_position {
			t.Errorf("message %q must name %s, after the previous field", message, field)
		}
		last_position = position
	}
	if strings.Contains(message, "SECRET") || strings.Contains(message, "sk-live") {
		t.Errorf("the message carries a value: %q", message)
	}
}

func Test_the_context_is_never_modified_by_adding_a_call_context(t *testing.T) {
	original_context := context.Background()

	with_context := call_context.WithCallContext(original_context, must_call_context(t, call_context.WithApplication("app")))

	if _, found := call_context.FromContext(original_context); found {
		t.Error("the original context must not carry the call context")
	}
	if _, found := call_context.FromContext(with_context); !found {
		t.Error("the new context must carry the call context")
	}
}

func Test_a_deeper_context_does_not_change_the_value_read_before(t *testing.T) {
	first_context := call_context.WithCallContext(context.Background(), must_call_context(t))
	read_before, _ := call_context.FromContext(first_context)

	_, _ = call_context.NextIndentation(first_context)

	if read_before.Indentation() != 1 {
		t.Errorf("a value read before changed: %d", read_before.Indentation())
	}
}

func Test_to_correlation_context_maps_the_ids_that_are_set(t *testing.T) {
	built_context := must_call_context(t, call_context.WithApplication("app"), call_context.WithInstanceID("instance"), call_context.WithExecutionID("run-1"))

	correlation_context, has_context, map_error := call_context.ToCorrelationContext(built_context)

	if map_error != nil || !has_context {
		t.Fatalf("ToCorrelationContext() = %v, %v", has_context, map_error)
	}
	if execution_id, is_present := correlation_context.ExecutionID(); !is_present || execution_id.String() != "run-1" {
		t.Errorf("execution id = %q, %v", execution_id, is_present)
	}
	if _, is_present := correlation_context.TraceID(); is_present {
		t.Error("an id that is not set must be absent")
	}
}

func Test_to_correlation_context_of_the_zero_call_context_has_no_context(t *testing.T) {
	_, has_context, map_error := call_context.ToCorrelationContext(call_context.CallContext{})

	if has_context || map_error != nil {
		t.Errorf("ToCorrelationContext(zero) = %v, %v; want false and no error", has_context, map_error)
	}
}

func Test_a_fake_id_generator_gives_each_request_its_own_correlation_id(t *testing.T) {
	id_generator := fakes.NewFakeIDGenerator()

	first_context, _ := call_context.NewRequestContext(context.Background(), id_generator, "app", "instance")
	second_context, _ := call_context.NewRequestContext(context.Background(), id_generator, "app", "instance")

	first_call_context, _ := call_context.FromContext(first_context)
	second_call_context, _ := call_context.FromContext(second_context)
	if first_call_context.CorrelationID() == second_call_context.CorrelationID() {
		t.Error("two requests must not share a correlation id")
	}
}
