package services_test

import (
	"context"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/call_context"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

// ----- THE ROUTING TABLE ----------------------------------------------------

func Test_the_destination_kind_follows_the_first_segment_of_the_event_type(t *testing.T) {
	cases := []struct {
		event_type string
		want       services.DestinationKind
	}{
		{"log.entry.recorded", services.DestinationKindLog},
		{"log", services.DestinationKindLog},
		{"metric.counter.incremented", services.DestinationKindMetrics},
		{"trace.span.ended", services.DestinationKindTrace},
		{"device.reading.taken", services.DestinationKindDeviceTelemetry},
		{"ai.generation.finished", services.DestinationKindAITelemetry},
		{"execution.step.finished", services.DestinationKindEvent},
		{"report.generated", services.DestinationKindEvent},
		{"", services.DestinationKindEvent},
		{".", services.DestinationKindEvent},
		{"logs.entry", services.DestinationKindEvent},
		{"LOG.entry", services.DestinationKindEvent},
		{"metrics.x", services.DestinationKindEvent},
		{" log.entry", services.DestinationKindEvent},
		{"unknown..type", services.DestinationKindEvent},
	}
	for _, test_case := range cases {
		if got := services.DestinationKindOf(test_case.event_type); got != test_case.want {
			t.Errorf("DestinationKindOf(%q) = %q, want %q", test_case.event_type, got, test_case.want)
		}
	}
}

// The ports under short names, for the tables below.
type (
	outbound_log     = outbound.LogSinkPort
	outbound_event   = outbound.EventSinkPort
	outbound_metrics = outbound.MetricsSinkPort
	outbound_trace   = outbound.TraceSinkPort
	outbound_device  = outbound.DeviceTelemetrySinkPort
	outbound_ai      = outbound.AITelemetrySinkPort
)

// ----- THE FIXTURES OF THE PIPELINE -----------------------------------------

type panicking_sink struct{}

func (panicking_sink) Send(_ context.Context, _ []value_objects.Envelope) dtos.SendResult {
	panic("the sink broke")
}

// pipeline_for builds a pipeline with the real masker and the default limits.
func pipeline_for(test *testing.T, validate_data services.EventDataValidator, destinations services.Destinations) *services.EventPipelineService {
	test.Helper()
	return pipeline_with(test, validate_data, value_objects.DefaultLimits(), destinations)
}

func pipeline_with(test *testing.T, validate_data services.EventDataValidator, limits value_objects.Limits, destinations services.Destinations) *services.EventPipelineService {
	test.Helper()
	return services.NewEventPipelineService(
		new_test_services(test, validate_data).service,
		services.NewMaskingService(spy_masker(test, masking.MaskerOptions{})),
		services.NewLimitsService(limits),
		destinations,
	)
}

func input_of_type(event_type string, data map[string]any) dtos.NewEventInput {
	return dtos.NewEventInput{EventType: event_type, Severity: value_objects.SeverityInfo, Data: data}
}

func Test_every_kind_reaches_only_the_sinks_of_its_kind(t *testing.T) {
	cases := []struct {
		event_type string
		sink_index int
	}{
		{"log.entry.recorded", 0},
		{"execution.step.finished", 1},
		{"metric.counter.incremented", 2},
		{"trace.span.ended", 3},
		{"device.reading.taken", 4},
		{"ai.generation.finished", 5},
	}
	for _, test_case := range cases {
		t.Run(test_case.event_type, func(t *testing.T) {
			log_sink, event_sink, metrics_sink := &fakes.FakeLogSink{}, &fakes.FakeEventSink{}, &fakes.FakeMetricsSink{}
			trace_sink, device_sink, ai_sink := &fakes.FakeTraceSink{}, &fakes.FakeDeviceTelemetrySink{}, &fakes.FakeAITelemetrySink{}
			batch_counts := []func() int{log_sink.Calls, event_sink.Calls, metrics_sink.Calls, trace_sink.Calls, device_sink.Calls, ai_sink.Calls}
			pipeline := pipeline_for(t, permissive_validator, services.Destinations{
				Log: []outbound_log{log_sink}, Event: []outbound_event{event_sink}, Metrics: []outbound_metrics{metrics_sink},
				Trace: []outbound_trace{trace_sink}, DeviceTelemetry: []outbound_device{device_sink}, AITelemetry: []outbound_ai{ai_sink},
			})

			result := pipeline.Emit(context.Background(), input_of_type(test_case.event_type, valid_log_data()))

			if result != (dtos.EmitResult{Accepted: 1}) {
				t.Fatalf("result = %+v", result)
			}
			for index, calls := range batch_counts {
				want := 0
				if index == test_case.sink_index {
					want = 1
				}
				if calls() != want {
					t.Errorf("sink %d received %d calls, want %d", index, calls(), want)
				}
			}
		})
	}
}

func Test_an_event_with_no_sink_of_its_kind_is_dropped_with_no_destination(t *testing.T) {
	pipeline := pipeline_for(t, permissive_validator, services.Destinations{Event: []outbound_event{&fakes.FakeEventSink{}}})

	result := pipeline.Emit(context.Background(), input_of_type("log.entry.recorded", valid_log_data()))

	if result != (dtos.EmitResult{Dropped: 1, Reason: services.ReasonNoDestination}) {
		t.Fatalf("result = %+v", result)
	}
}

func Test_a_panicking_sink_counts_one_failure_and_the_others_still_receive(t *testing.T) {
	healthy_sink := &fakes.FakeLogSink{}
	pipeline := pipeline_for(t, event_validation.ValidateEvent, services.Destinations{
		Log: []outbound_log{panicking_sink{}, healthy_sink},
	})

	result := pipeline.Emit(context.Background(), input_of_type("log.entry.recorded", valid_log_data()))

	if result != (dtos.EmitResult{Accepted: 1, Failed: 1, Reason: services.ReasonSinkPanicked}) {
		t.Fatalf("result = %+v", result)
	}
	if healthy_sink.Calls() != 1 {
		t.Fatalf("the healthy sink received %d calls", healthy_sink.Calls())
	}
}

func Test_the_result_is_the_sum_of_the_sinks_and_the_reason_is_the_first_one(t *testing.T) {
	first_sink, second_sink, third_sink := &fakes.FakeLogSink{}, &fakes.FakeLogSink{}, &fakes.FakeLogSink{}
	first_sink.SetResult(dtos.SendResult{Accepted: 1})
	second_sink.SetResult(dtos.SendResult{Dropped: 2, Failed: 1, Reason: "queue_full"})
	third_sink.SetResult(dtos.SendResult{Failed: 3, Reason: "send_failed"})
	pipeline := pipeline_for(t, permissive_validator, services.Destinations{Log: []outbound_log{first_sink, second_sink, third_sink}})

	result := pipeline.Emit(context.Background(), input_of_type("log.entry.recorded", valid_log_data()))

	if result != (dtos.EmitResult{Accepted: 1, Dropped: 2, Failed: 4, Reason: "queue_full"}) {
		t.Fatalf("result = %+v", result)
	}
}

func Test_a_failure_of_the_chain_is_a_drop_with_the_code_of_the_error(t *testing.T) {
	tiny_limits, limits_error := value_objects.NewLimits(32, 64, 256, 100, 1000)
	if limits_error != nil {
		t.Fatal(limits_error)
	}
	cases := []struct {
		name     string
		pipeline *services.EventPipelineService
		input    dtos.NewEventInput
		want     string
	}{
		{"unknown event type", pipeline_for(t, event_validation.ValidateEvent, services.Destinations{}), input_of_type("nope.nothing", valid_log_data()), value_objects.ErrorCodeEventTypeUnknown},
		{"data that does not match the schema", pipeline_for(t, event_validation.ValidateEvent, services.Destinations{}), input_of_type("log.entry.recorded", map[string]any{"category": "http"}), value_objects.ErrorCodeEventSchemaInvalid},
		{"over the event limit", pipeline_with(t, permissive_validator, tiny_limits, services.Destinations{}), input_of_type("log.entry.recorded", map[string]any{"message": strings.Repeat("a", 500)}), value_objects.ErrorCodeLimitExceeded},
		{"a validator that panics", pipeline_for(t, func(string, any) error { panic("boom") }, services.Destinations{}), input_of_type("log.entry.recorded", valid_log_data()), value_objects.ErrorCodeEventBuildFailed},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			result := test_case.pipeline.Emit(context.Background(), test_case.input)
			if result != (dtos.EmitResult{Dropped: 1, Reason: test_case.want}) {
				t.Fatalf("result = %+v, want a drop with %s", result, test_case.want)
			}
		})
	}
}

func Test_a_failure_of_masking_or_of_limits_drops_the_event_and_no_sink_receives_it(t *testing.T) {
	cases := []struct {
		name    string
		masking *services.MaskingService
		limits  *services.LimitsService
		want    string
	}{
		{"masking", nil, services.NewLimitsService(value_objects.DefaultLimits()), value_objects.ErrorCodeMaskingFailed},
		{"limits", services.NewMaskingService(spy_masker(t, masking.MaskerOptions{})), nil, value_objects.ErrorCodeLimitsFailed},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			sink := &fakes.FakeLogSink{}
			pipeline := services.NewEventPipelineService(
				new_test_services(t, permissive_validator).service, test_case.masking, test_case.limits,
				services.Destinations{Log: []outbound_log{sink}},
			)

			result := pipeline.Emit(context.Background(), input_of_type("log.entry.recorded", valid_log_data()))

			if result != (dtos.EmitResult{Dropped: 1, Reason: test_case.want}) {
				t.Fatalf("result = %+v", result)
			}
			if sink.Calls() != 0 {
				t.Fatal("a sink received an event whose chain failed")
			}
		})
	}
}

func Test_a_pipeline_that_cannot_run_never_panics(t *testing.T) {
	broken_pipeline := services.NewEventPipelineService(nil, nil, nil, services.Destinations{})
	if result := broken_pipeline.Emit(context.Background(), input_of_type("log.entry.recorded", valid_log_data())); result != (dtos.EmitResult{Dropped: 1, Reason: value_objects.ErrorCodeEventBuildFailed}) {
		t.Fatalf("result = %+v", result)
	}

	nil_sink_pipeline := pipeline_for(t, permissive_validator, services.Destinations{Log: []outbound_log{nil}})
	if result := nil_sink_pipeline.Emit(context.Background(), input_of_type("log.entry.recorded", valid_log_data())); result != (dtos.EmitResult{Dropped: 1, Reason: value_objects.ErrorCodeEventBuildFailed}) {
		t.Fatalf("a nil sink: result = %+v", result)
	}
}

// ----- THE CALL CONTEXT -----------------------------------------------------

func Test_the_call_context_of_ctx_becomes_the_context_of_the_event_unless_the_input_has_one(t *testing.T) {
	carried, context_error := call_context.NewCallContext(call_context.WithTraceID("0af7651916cd43dd8448eb211c80319c"), call_context.WithExecutionID("run-7"))
	if context_error != nil {
		t.Fatal(context_error)
	}
	ctx := call_context.WithCallContext(context.Background(), carried)
	own_context, own_error := value_objects.NewCorrelationContext(value_objects.WithCorrelationID("own-1"))
	if own_error != nil {
		t.Fatal(own_error)
	}
	only_application, application_error := call_context.NewCallContext(call_context.WithApplication("shop"))
	if application_error != nil {
		t.Fatal(application_error)
	}

	run := func(emit_context context.Context, input dtos.NewEventInput) value_objects.Envelope {
		sink := &fakes.FakeLogSink{}
		pipeline := pipeline_for(t, permissive_validator, services.Destinations{Log: []outbound_log{sink}})
		pipeline.Emit(emit_context, input)
		return sink.Batches()[0][0]
	}

	from_call_context := run(ctx, input_of_type("log.entry.recorded", valid_log_data()))
	if from_call_context.Context == nil || from_call_context.Context.TraceId != "0af7651916cd43dd8448eb211c80319c" || from_call_context.Context.ExecutionId != "run-7" {
		t.Fatalf("the context of the call was not attached: %+v", from_call_context.Context)
	}

	input_with_context := input_of_type("log.entry.recorded", valid_log_data())
	input_with_context.Context = &own_context
	from_input := run(ctx, input_with_context)
	if from_input.Context == nil || from_input.Context.CorrelationId != "own-1" || from_input.Context.TraceId != "" {
		t.Fatalf("the context of the input must win: %+v", from_input.Context)
	}

	if without := run(context.Background(), input_of_type("log.entry.recorded", valid_log_data())); without.Context != nil {
		t.Fatalf("no call context must give no context: %+v", without.Context)
	}
	if without := run(call_context.WithCallContext(context.Background(), only_application), input_of_type("log.entry.recorded", valid_log_data())); without.Context != nil {
		t.Fatalf("a call context with no id must give no context: %+v", without.Context)
	}
}

// ----- THE SPY: NO SINK EVER RECEIVES A SECRET -------------------------------

type secret_kind struct {
	name        string
	text        string
	core        string
	has_newline bool
}

func secret_kinds() []secret_kind {
	return []secret_kind{
		{"aws access key", spy_access_key_id, spy_access_key_id, false},
		{"jwt", spy_jwt, spy_jwt, false},
		{"pem block", spy_pem, spy_pem_body, true},
		{"connection string password", "postgres://admin:" + spy_password + "@db:5432/app", spy_password, false},
		{"bearer token", "Authorization: Bearer " + spy_bearer_value, spy_bearer_value, false},
		{"password pair", "password=" + spy_password, spy_password, false},
	}
}

// secret_input_in places the secret in the one place: data, attributes or context.
func secret_input_in(test *testing.T, event_type string, place string, secret secret_kind) dtos.NewEventInput {
	test.Helper()
	input := input_of_type(event_type, map[string]any{"category": "http", "message": "plain"})
	switch place {
	case "data":
		input.Data["message"] = "before " + secret.text + " after"
	case "attributes":
		attributes, attributes_error := value_objects.NewEventAttributes(map[string]any{"note": "see " + secret.text})
		if attributes_error != nil {
			test.Fatal(attributes_error)
		}
		input.Attributes = &attributes
	case "context":
		correlation_context, context_error := value_objects.NewCorrelationContext(value_objects.WithCorrelationID(secret.text))
		if context_error != nil {
			test.Fatal(context_error)
		}
		input.Context = &correlation_context
	}
	return input
}

// every_sink_spy is one sink of each kind that fails the test when what it receives holds a secret.
type spy_run struct {
	event_type string
	sinks      services.Destinations
	received   func() []value_objects.Envelope
}

func spy_runs() []spy_run {
	var runs []spy_run
	add := func(event_type string, destinations func(*fakes.FakeLogSink, *fakes.FakeEventSink, *fakes.FakeMetricsSink, *fakes.FakeTraceSink, *fakes.FakeDeviceTelemetrySink, *fakes.FakeAITelemetrySink) services.Destinations) {
		log_sink, event_sink, metrics_sink := &fakes.FakeLogSink{}, &fakes.FakeEventSink{}, &fakes.FakeMetricsSink{}
		trace_sink, device_sink, ai_sink := &fakes.FakeTraceSink{}, &fakes.FakeDeviceTelemetrySink{}, &fakes.FakeAITelemetrySink{}
		runs = append(runs, spy_run{
			event_type: event_type,
			sinks:      destinations(log_sink, event_sink, metrics_sink, trace_sink, device_sink, ai_sink),
			received: func() []value_objects.Envelope {
				var received []value_objects.Envelope
				for _, batches := range [][][]value_objects.Envelope{log_sink.Batches(), event_sink.Batches(), metrics_sink.Batches(), trace_sink.Batches(), device_sink.Batches(), ai_sink.Batches()} {
					for _, batch := range batches {
						received = append(received, batch...)
					}
				}
				return received
			},
		})
	}
	all := func(log_sink *fakes.FakeLogSink, event_sink *fakes.FakeEventSink, metrics_sink *fakes.FakeMetricsSink, trace_sink *fakes.FakeTraceSink, device_sink *fakes.FakeDeviceTelemetrySink, ai_sink *fakes.FakeAITelemetrySink) services.Destinations {
		return services.Destinations{
			Log: []outbound_log{log_sink}, Event: []outbound_event{event_sink}, Metrics: []outbound_metrics{metrics_sink},
			Trace: []outbound_trace{trace_sink}, DeviceTelemetry: []outbound_device{device_sink}, AITelemetry: []outbound_ai{ai_sink},
		}
	}
	for _, event_type := range []string{"log.entry.recorded", "execution.step.finished", "metric.counter.incremented", "trace.span.ended", "device.reading.taken", "ai.generation.finished"} {
		add(event_type, all)
	}
	return runs
}

func Test_no_sink_of_any_kind_receives_a_secret_in_data_attributes_or_context(t *testing.T) {
	for _, run := range spy_runs() {
		for _, place := range []string{"data", "attributes", "context"} {
			for _, secret := range secret_kinds() {
				if place == "context" && secret.has_newline {
					continue // an id of the context with a line break is refused by the value object, so it cannot get there
				}
				t.Run(run.event_type+"/"+place+"/"+secret.name, func(t *testing.T) {
					pipeline := pipeline_for(t, permissive_validator, run.sinks)

					result := pipeline.Emit(context.Background(), secret_input_in(t, run.event_type, place, secret))

					if result.Accepted != 1 {
						t.Fatalf("result = %+v", result)
					}
					received := run.received()
					if len(received) == 0 {
						t.Fatal("the sink received nothing, so the test proves nothing")
					}
					sent_text := canonical_text(t, received[len(received)-1])
					if strings.Contains(sent_text, secret.core) {
						t.Fatalf("a sink received the secret %q: %s", secret.core, sent_text)
					}
					if !strings.Contains(sent_text, domain.MaskedText) {
						t.Fatalf("the mask is not there: %s", sent_text)
					}
				})
			}
		}
	}
}
