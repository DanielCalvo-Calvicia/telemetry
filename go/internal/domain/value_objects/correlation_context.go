package value_objects

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
)

// ===== CorrelationContext ===================================================

// CorrelationContext links an event to a trace and to the work that produced it. Every field is optional: absent is
// allowed, present but empty is not, and the context with no field is valid. trace_id and span_id follow the W3C
// trace context (32 and 16 lowercase hexadecimal digits). The wire struct of the contract is Context (generated);
// this is its validated form.
type CorrelationContext struct {
	trace_id       optional_field[base_value_objects.HexDigits]
	span_id        optional_field[base_value_objects.HexDigits]
	execution_id   optional_field[base_value_objects.Identifier]
	correlation_id optional_field[base_value_objects.Identifier]
}

// ContextOption sets a field of a CorrelationContext. A field that is set is present, even when its text is empty;
// the constructor then rejects it.
type ContextOption func(context *CorrelationContext)

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrContextTraceIDInvalid is domain.ErrContextTraceIDInvalid.
	ErrContextTraceIDInvalid = domain.ErrContextTraceIDInvalid
	// ErrContextSpanIDInvalid is domain.ErrContextSpanIDInvalid.
	ErrContextSpanIDInvalid = domain.ErrContextSpanIDInvalid
	// ErrContextExecutionIDInvalid is domain.ErrContextExecutionIDInvalid.
	ErrContextExecutionIDInvalid = domain.ErrContextExecutionIDInvalid
	// ErrContextCorrelationIDInvalid is domain.ErrContextCorrelationIDInvalid.
	ErrContextCorrelationIDInvalid = domain.ErrContextCorrelationIDInvalid
)

// ----- CONSTANTS ------------------------------------------------------------

const (
	trace_id_width = domain.TraceIDWidth
	span_id_width  = domain.SpanIDWidth
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewCorrelationContext returns the CorrelationContext of the options. With no option it returns the empty context.
// When a field is invalid it returns the sentinel of each invalid field (ErrContextTraceIDInvalid, ...) joined with
// the cause of the base value object, so errors.Is finds both.
func NewCorrelationContext(options ...ContextOption) (CorrelationContext, error) {
	var context CorrelationContext
	for _, option := range options {
		option(&context)
	}
	validation_error := JoinErrors(
		field_failure(ErrContextTraceIDInvalid, context.trace_id.validate_if_present(validate_trace_id)),
		field_failure(ErrContextSpanIDInvalid, context.span_id.validate_if_present(validate_span_id)),
		field_failure(ErrContextExecutionIDInvalid, context.execution_id.validate_if_present(base_value_objects.ValidateIdentifier)),
		field_failure(ErrContextCorrelationIDInvalid, context.correlation_id.validate_if_present(base_value_objects.ValidateIdentifier)),
	)
	if validation_error != nil {
		return CorrelationContext{}, validation_error
	}
	return context, nil
}

// WithTraceID sets the W3C trace id: 32 lowercase hexadecimal digits.
func WithTraceID(trace_id string) ContextOption {
	return func(context *CorrelationContext) {
		context.trace_id = present_field(base_value_objects.HexDigits(trace_id))
	}
}

// WithSpanID sets the W3C span id: 16 lowercase hexadecimal digits.
func WithSpanID(span_id string) ContextOption {
	return func(context *CorrelationContext) {
		context.span_id = present_field(base_value_objects.HexDigits(span_id))
	}
}

// WithExecutionID sets the id of the execution that produced the event.
func WithExecutionID(execution_id string) ContextOption {
	return func(context *CorrelationContext) {
		context.execution_id = present_field(base_value_objects.Identifier(execution_id))
	}
}

// WithCorrelationID sets the id that links related events.
func WithCorrelationID(correlation_id string) ContextOption {
	return func(context *CorrelationContext) {
		context.correlation_id = present_field(base_value_objects.Identifier(correlation_id))
	}
}

// ----- METHODS --------------------------------------------------------------

// TraceID returns the trace id and whether it is present.
func (context CorrelationContext) TraceID() (base_value_objects.HexDigits, bool) {
	return context.trace_id.get()
}

// SpanID returns the span id and whether it is present.
func (context CorrelationContext) SpanID() (base_value_objects.HexDigits, bool) {
	return context.span_id.get()
}

// ExecutionID returns the execution id and whether it is present.
func (context CorrelationContext) ExecutionID() (base_value_objects.Identifier, bool) {
	return context.execution_id.get()
}

// CorrelationID returns the correlation id and whether it is present.
func (context CorrelationContext) CorrelationID() (base_value_objects.Identifier, bool) {
	return context.correlation_id.get()
}

// ----- VALIDATORS -----------------------------------------------------------

// validate_trace_id checks the width and the digits of a W3C trace id.
func validate_trace_id(text string) error {
	return base_value_objects.ValidateHexDigits(text, trace_id_width)
}

// validate_span_id checks the width and the digits of a W3C span id.
func validate_span_id(text string) error {
	return base_value_objects.ValidateHexDigits(text, span_id_width)
}
