package call_depth_test

import (
	"context"
	"errors"
	"testing"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	call_depth "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/call_depth"
)

type fake_id_generator struct {
	id_to_return    string
	error_to_return error
}

func (id_generator fake_id_generator) NewID(_ context.Context) (string, error) {
	return id_generator.id_to_return, id_generator.error_to_return
}

func Test_a_context_without_call_context_has_none(t *testing.T) {
	call_context, found := call_depth.FromContext(context.Background())

	if found || call_context != (call_depth.CallContext{}) {
		t.Errorf("FromContext() = %+v, %v; want the zero value and false", call_context, found)
	}
}

func Test_with_call_context_stores_and_reads_back_every_value(t *testing.T) {
	stored_call_context := call_depth.CallContext{Application: "app", ApplicationID: "instance", ProcessID: "request", Indentation: 2}
	ctx := call_depth.WithCallContext(context.Background(), stored_call_context)

	call_context, found := call_depth.FromContext(ctx)

	if !found || call_context != stored_call_context {
		t.Errorf("FromContext() = %+v, %v; want %+v", call_context, found, stored_call_context)
	}
}

func Test_new_request_context_starts_a_request(t *testing.T) {
	id_generator := fake_id_generator{id_to_return: "new-process-id"}

	ctx, err := call_depth.NewRequestContext(context.Background(), id_generator, "app", "instance")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call_context, found := call_depth.FromContext(ctx)
	expected_call_context := call_depth.CallContext{Application: "app", ApplicationID: "instance", ProcessID: "new-process-id", Indentation: call_depth.StartingIndentation}
	if !found || call_context != expected_call_context {
		t.Errorf("FromContext() = %+v, %v; want %+v", call_context, found, expected_call_context)
	}
	if call_depth.StartingIndentation != 1 {
		t.Errorf("StartingIndentation = %d, want 1", call_depth.StartingIndentation)
	}
}

func Test_new_request_context_when_the_id_cannot_be_generated(t *testing.T) {
	random_source_failure := errors.New("random source unavailable")
	id_generator := fake_id_generator{error_to_return: random_source_failure}
	original_context := context.Background()

	ctx, err := call_depth.NewRequestContext(original_context, id_generator, "app", "instance")

	if !errors.Is(err, call_depth.ErrCallDepthProcessIDFailed) || !errors.Is(err, application.ErrCallDepthProcessIDFailed) {
		t.Errorf("error = %v, want ErrCallDepthProcessIDFailed", err)
	}
	if !errors.Is(err, random_source_failure) {
		t.Errorf("error = %v, want the root cause kept", err)
	}
	if _, found := call_depth.FromContext(ctx); found {
		t.Error("a failed request context must not carry a call context")
	}
}

func Test_next_indentation_goes_one_layer_deeper_without_touching_the_original(t *testing.T) {
	original_context := call_depth.WithCallContext(context.Background(), call_depth.CallContext{ProcessID: "request", Indentation: 1})

	deeper_context, ok := call_depth.NextIndentation(original_context)
	if !ok {
		t.Fatal("NextIndentation must succeed when there is a call context")
	}
	deepest_context, _ := call_depth.NextIndentation(deeper_context)

	original_call_context, _ := call_depth.FromContext(original_context)
	deeper_call_context, _ := call_depth.FromContext(deeper_context)
	deepest_call_context, _ := call_depth.FromContext(deepest_context)
	if original_call_context.Indentation != 1 || deeper_call_context.Indentation != 2 || deepest_call_context.Indentation != 3 {
		t.Errorf("indentations = %d, %d, %d; want 1, 2, 3", original_call_context.Indentation, deeper_call_context.Indentation, deepest_call_context.Indentation)
	}
	if deepest_call_context.ProcessID != "request" {
		t.Error("going deeper must keep the rest of the call context")
	}
}

func Test_next_indentation_without_call_context_returns_the_context_unchanged(t *testing.T) {
	original_context := context.Background()

	returned_context, ok := call_depth.NextIndentation(original_context)

	if ok || returned_context != original_context {
		t.Errorf("NextIndentation() = %v, %v; want the same context and false", returned_context, ok)
	}
}
