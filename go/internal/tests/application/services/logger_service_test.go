package services_test

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/inbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// The logger satisfies its port and the pipeline satisfies the emitter; a call returns no value (compile time: these
// are method expressions of a func without results).
var (
	_ inbound.LoggerPort                                                 = (*services.LoggerService)(nil)
	_ services.EventEmitter                                              = (*services.EventPipelineService)(nil)
	_ func(*services.LoggerService, context.Context, dtos.LogEntryInput) = (*services.LoggerService).Info
)

type fake_emitter struct {
	mutex  sync.Mutex
	inputs []dtos.NewEventInput
	panics bool
}

func (emitter *fake_emitter) Emit(_ context.Context, input dtos.NewEventInput) dtos.EmitResult {
	if emitter.panics {
		panic("the emitter failed")
	}
	emitter.mutex.Lock()
	defer emitter.mutex.Unlock()
	emitter.inputs = append(emitter.inputs, input)
	return dtos.EmitResult{Accepted: 1}
}

func Test_each_method_emits_a_log_event_with_the_severity_of_the_method(t *testing.T) {
	emitter := &fake_emitter{}
	logger := services.NewLoggerService(emitter)
	ctx := context.Background()
	entry := dtos.LogEntryInput{Category: "http", Message: "done"}

	logger.Trace(ctx, entry)
	logger.Debug(ctx, entry)
	logger.Info(ctx, entry)
	logger.Warn(ctx, entry)
	logger.Error(ctx, entry)
	logger.Critical(ctx, entry)

	want_severities := []value_objects.Severity{
		value_objects.SeverityTrace, value_objects.SeverityDebug, value_objects.SeverityInfo,
		value_objects.SeverityWarn, value_objects.SeverityError, value_objects.SeverityCritical,
	}
	var got_severities []value_objects.Severity
	for _, input := range emitter.inputs {
		if input.EventType != value_objects.EventTypeLogEntryRecorded {
			t.Errorf("event type %q", input.EventType)
		}
		got_severities = append(got_severities, input.Severity)
	}
	if !slices.Equal(got_severities, want_severities) {
		t.Fatalf("severities %v, want %v", got_severities, want_severities)
	}
}

func Test_data_has_exactly_the_category_the_message_and_the_function_only_when_given(t *testing.T) {
	emitter := &fake_emitter{}
	logger := services.NewLoggerService(emitter)

	logger.Info(context.Background(), dtos.LogEntryInput{Category: "http", Message: "done"})
	logger.Info(context.Background(), dtos.LogEntryInput{Category: "", Function: "serve", Message: ""})

	want_data := []map[string]any{
		{"category": "http", "message": "done"},
		{"category": "", "function": "serve", "message": ""},
	}
	for position, input := range emitter.inputs {
		if !reflect.DeepEqual(input.Data, want_data[position]) {
			t.Errorf("data %v, want %v", input.Data, want_data[position])
		}
	}
}

func Test_no_body_is_captured_a_call_with_only_category_and_message_has_no_attributes(t *testing.T) {
	emitter := &fake_emitter{}
	logger := services.NewLoggerService(emitter)

	logger.Error(context.Background(), dtos.LogEntryInput{Category: "http", Message: "failed"})

	if len(emitter.inputs) != 1 {
		t.Fatalf("%d events", len(emitter.inputs))
	}
	input := emitter.inputs[0]
	if len(input.Data) != 2 {
		t.Errorf("data keys %v, want exactly category and message", input.Data)
	}
	if input.Attributes != nil {
		t.Errorf("attributes %v, want none", input.Attributes.Entries())
	}
	if input.Actor != nil || input.Context != nil || input.OccurredAt != nil {
		t.Errorf("the logger set more than data and severity: %+v", input)
	}
}

func Test_the_fields_become_the_attributes(t *testing.T) {
	emitter := &fake_emitter{}
	logger := services.NewLoggerService(emitter)

	logger.Info(context.Background(), dtos.LogEntryInput{Category: "c", Message: "m", Fields: map[string]any{"order": 7, "": "x", "gone": nil}})

	attributes := emitter.inputs[0].Attributes
	if attributes == nil || !reflect.DeepEqual(attributes.Entries(), map[string]any{"order": int64(7)}) {
		t.Fatalf("attributes %v", attributes)
	}
}

func Test_a_panicking_emitter_does_not_panic_the_caller(t *testing.T) {
	logger := services.NewLoggerService(&fake_emitter{panics: true})
	ctx := context.Background()
	entry := dtos.LogEntryInput{Category: "c", Message: "m"}

	logger.Trace(ctx, entry)
	logger.Debug(ctx, entry)
	logger.Info(ctx, entry)
	logger.Warn(ctx, entry)
	logger.Error(ctx, entry)
	logger.Critical(ctx, entry)
}

func Test_a_nil_service_and_a_nil_emitter_do_nothing(t *testing.T) {
	var nil_service *services.LoggerService
	nil_service.Info(context.Background(), dtos.LogEntryInput{Category: "c", Message: "m"})
	nil_service.Critical(context.Background(), dtos.LogEntryInput{})

	services.NewLoggerService(nil).Warn(context.Background(), dtos.LogEntryInput{Category: "c", Message: "m"})
}

func Test_a_call_with_a_nil_context_does_not_panic(t *testing.T) {
	emitter := &fake_emitter{}
	//nolint:staticcheck // a nil context is the case under test
	services.NewLoggerService(emitter).Info(nil, dtos.LogEntryInput{Category: "c", Message: "m"})
}

func Test_the_logger_is_safe_for_concurrent_use(t *testing.T) {
	emitter := &fake_emitter{}
	logger := services.NewLoggerService(emitter)
	var group sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for call := 0; call < 50; call++ {
				logger.Info(context.Background(), dtos.LogEntryInput{Category: "c", Message: "m", Fields: map[string]any{"a": call}})
			}
		}()
	}
	group.Wait()
	if len(emitter.inputs) != 800 {
		t.Fatalf("%d events, want 800", len(emitter.inputs))
	}
}
