package call_context

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// CORRELATION
// =============================================================================

// ----- MAPPERS --------------------------------------------------------------

// ToCorrelationContext returns the correlation context of the envelope for a call context: the trace id, the span id,
// the execution id and the correlation id, only the ones that are set. The bool is false when none of the four is set;
// the envelope then has no context. The application and the instance id are NOT part of the envelope context: the
// resource comes from the configuration, and they are kept for the migration of the old consumers and for later
// per-request use. The indentation is not part of it either (the console adapter decides how to draw it). An error of
// value_objects.NewCorrelationContext is returned as it is.
func ToCorrelationContext(call_context CallContext) (value_objects.CorrelationContext, bool, error) {
	var options []value_objects.ContextOption
	if call_context.trace_id != "" {
		options = append(options, value_objects.WithTraceID(call_context.trace_id))
	}
	if call_context.span_id != "" {
		options = append(options, value_objects.WithSpanID(call_context.span_id))
	}
	if call_context.execution_id != "" {
		options = append(options, value_objects.WithExecutionID(call_context.execution_id))
	}
	if call_context.correlation_id != "" {
		options = append(options, value_objects.WithCorrelationID(call_context.correlation_id))
	}
	if len(options) == 0 {
		return value_objects.CorrelationContext{}, false, nil
	}

	correlation_context, build_error := value_objects.NewCorrelationContext(options...)
	if build_error != nil {
		return value_objects.CorrelationContext{}, false, build_error
	}
	return correlation_context, true, nil
}
