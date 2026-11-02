package services

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/call_context"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// EVENT PIPELINE SERVICE
// =============================================================================

// The reasons of an EmitResult that are not a code of the catalog of error codes.
const (
	// ReasonNoDestination is the reason of an event that was dropped because no sink of its kind was given.
	ReasonNoDestination = "no_destination"
	// ReasonSinkPanicked is the reason of an event that a sink failed to take because it panicked.
	ReasonSinkPanicked = "sink_panicked"
)

// Destinations are the sinks the pipeline delivers to, by kind, in the order the consumer gave them. Several sinks of
// the same kind are allowed (REQ-OUT-003); every one of them receives every event of its kind.
type Destinations struct {
	Log             []outbound.LogSinkPort
	Event           []outbound.EventSinkPort
	Metrics         []outbound.MetricsSinkPort
	Trace           []outbound.TraceSinkPort
	DeviceTelemetry []outbound.DeviceTelemetrySinkPort
	AITelemetry     []outbound.AITelemetrySinkPort
}

// EventPipelineService runs the whole chain for one input: it builds the event, masks it, limits it and hands it to the
// sinks of its kind (T-03.9). It holds no state that changes, so one instance serves any number of goroutines.
type EventPipelineService struct {
	builder      *EventBuilderService
	masking      *MaskingService
	limits       *LimitsService
	destinations Destinations
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewEventPipelineService returns an EventPipelineService that uses the services and delivers to the destinations.
func NewEventPipelineService(
	builder *EventBuilderService,
	masking *MaskingService,
	limits *LimitsService,
	destinations Destinations,
) *EventPipelineService {
	return &EventPipelineService{builder: builder, masking: masking, limits: limits, destinations: destinations}
}

// ----- METHODS --------------------------------------------------------------

// Emit turns one input into one event and delivers it. It never panics and never returns an error (ADR-005,
// REQ-DLV-002): a panic that is recovered anywhere in the chain is EmitResult{Dropped: 1, Reason: event_build_failed}.
// The steps run in this order:
//
//  1. When input.Context is nil and ctx carries a call context (call_context.FromContext), the correlation context of
//     the call context becomes the context of the event; when it gives none, or an error, the event has no context.
//  2. Build (EventBuilderService.Build). An error is Dropped 1 with the code of the error as Reason.
//  3. Mask (MaskingService.Mask), with the same outcome on an error.
//  4. Apply the limits (LimitsService.Apply), with the same outcome on an error (for example limit_exceeded).
//  5. Route by the first segment of the event type (DestinationKindOf). No sink of that kind is Dropped 1 with the
//     reason ReasonNoDestination.
//  6. Deliver: Send is called with the one envelope on EVERY sink of the kind, in the order given, each call protected
//     by its own recover. A sink that panics counts Failed 1 with the reason ReasonSinkPanicked and never stops the
//     others. The result is the sum of the results of the sinks, and Reason is the first reason that is not empty.
//
// Only the masked and limited envelope reaches a sink: no path sends an envelope that was not masked (ADR-006).
// Until T-05 gives the core a bounded queue and the delivery modes, the sinks are called synchronously by the caller
// of Emit. Counting the rejected events as diagnostics is T-05.5.
func (service *EventPipelineService) Emit(ctx context.Context, input dtos.NewEventInput) (result dtos.EmitResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = dtos.EmitResult{Dropped: 1, Reason: value_objects.ErrorCodeEventBuildFailed}
		}
	}()

	if input.Context == nil {
		input.Context = context_of_call(ctx)
	}

	built_envelope, build_error := service.builder.Build(ctx, input)
	if build_error != nil {
		return dropped_because_of(build_error)
	}
	masked_envelope, mask_error := service.masking.Mask(ctx, built_envelope)
	if mask_error != nil {
		return dropped_because_of(mask_error)
	}
	limited_envelope, _, limits_error := service.limits.Apply(ctx, masked_envelope)
	if limits_error != nil {
		return dropped_because_of(limits_error)
	}

	return service.deliver(ctx, DestinationKindOf(limited_envelope.EventType), limited_envelope)
}

// ----- HELPERS --------------------------------------------------------------

// deliver sends the envelope to every sink of the kind and sums what they answer.
func (service *EventPipelineService) deliver(ctx context.Context, kind DestinationKind, envelope value_objects.Envelope) dtos.EmitResult {
	send_functions := service.send_functions(kind)
	if len(send_functions) == 0 {
		return dtos.EmitResult{Dropped: 1, Reason: ReasonNoDestination}
	}

	events := []value_objects.Envelope{envelope}
	var total dtos.EmitResult
	for _, send := range send_functions {
		sink_result := send_protected(func() dtos.SendResult { return send(ctx, events) })
		total.Accepted += sink_result.Accepted
		total.Dropped += sink_result.Dropped
		total.Failed += sink_result.Failed
		if total.Reason == "" {
			total.Reason = sink_result.Reason
		}
	}
	return total
}

// send_functions returns the Send method of every sink of the kind, in the order given.
func (service *EventPipelineService) send_functions(kind DestinationKind) []func(context.Context, []value_objects.Envelope) dtos.SendResult {
	var send_functions []func(context.Context, []value_objects.Envelope) dtos.SendResult
	switch kind {
	case DestinationKindLog:
		for _, sink := range service.destinations.Log {
			send_functions = append(send_functions, sink.Send)
		}
	case DestinationKindMetrics:
		for _, sink := range service.destinations.Metrics {
			send_functions = append(send_functions, sink.Send)
		}
	case DestinationKindTrace:
		for _, sink := range service.destinations.Trace {
			send_functions = append(send_functions, sink.Send)
		}
	case DestinationKindDeviceTelemetry:
		for _, sink := range service.destinations.DeviceTelemetry {
			send_functions = append(send_functions, sink.Send)
		}
	case DestinationKindAITelemetry:
		for _, sink := range service.destinations.AITelemetry {
			send_functions = append(send_functions, sink.Send)
		}
	default:
		for _, sink := range service.destinations.Event {
			send_functions = append(send_functions, sink.Send)
		}
	}
	return send_functions
}

// send_protected calls the function and turns a panic into Failed 1 with the reason ReasonSinkPanicked. The panic value
// is not kept, because it could hold data of the event.
func send_protected(send func() dtos.SendResult) (result dtos.SendResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = dtos.SendResult{Failed: 1, Reason: ReasonSinkPanicked}
		}
	}()
	return send()
}

// dropped_because_of is the result of an event that was dropped for the error: its code is the reason.
func dropped_because_of(cause error) dtos.EmitResult {
	return dtos.EmitResult{Dropped: 1, Reason: value_objects.CodeOf(cause)}
}

// context_of_call returns the correlation context of the call context that ctx carries, or nil when there is none, it
// sets no field or it is not valid.
func context_of_call(ctx context.Context) *value_objects.CorrelationContext {
	carried_call_context, found := call_context.FromContext(ctx)
	if !found {
		return nil
	}
	correlation_context, has_fields, mapping_error := call_context.ToCorrelationContext(carried_call_context)
	if mapping_error != nil || !has_fields {
		return nil
	}
	return &correlation_context
}
