// Package call_context carries the identity and the correlation of a call through the program, in context.Context, so
// that a log can attach them when they exist (REQ-LOG-005). It holds no global state (REQ-ARC-006) and it carries only
// identifiers and the call depth, never a secret, a body or a payload (REQ-LOG-006).
package call_context

import (
	"context"
	"fmt"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// CALL CONTEXT
// =============================================================================

// CallContext is the identity and the position of a call, carried through context.Context. It is an immutable value:
// it has no setter and the getters return copies. A field that was not set is an empty text. The application and the
// instance identify the running program, the execution id the unit of work, the correlation id links related events,
// the trace id and the span id follow the W3C Trace Context, and the indentation starts at StartingIndentation at the
// inbound adapter and grows by one each time the call moves to a deeper layer. An incoming id is data, not
// authorization (REQ-TRC-003): it is only validated, never trusted.
type CallContext struct {
	application    string
	instance_id    string
	execution_id   string
	correlation_id string
	trace_id       string
	span_id        string
	indentation    int
}

// Option sets a field of a CallContext. An empty text means that the field is not set.
type Option func(call_context *CallContext)

// callContextKey is the key under which the CallContext is stored; being a private type, no other package can read or
// overwrite the value by accident.
type callContextKey struct{}

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrCallContextFieldInvalid is application.ErrCallContextFieldInvalid.
	ErrCallContextFieldInvalid = application.ErrCallContextFieldInvalid
	// ErrCallContextIDAllZeros is application.ErrCallContextIDAllZeros.
	ErrCallContextIDAllZeros = application.ErrCallContextIDAllZeros
)

// ----- CONSTANTS ------------------------------------------------------------

// StartingIndentation is the indentation of the first layer, the inbound adapter.
const StartingIndentation = 1

const (
	field_application    = "application"
	field_instance_id    = "instance_id"
	field_execution_id   = "execution_id"
	field_correlation_id = "correlation_id"
	field_trace_id       = "trace_id"
	field_span_id        = "span_id"
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewCallContext returns the CallContext of the options, with the starting indentation. With no option, or with empty
// texts, the fields are not set and it is valid. application, instance id, execution id and correlation id must be
// valid identifiers when set (not blank, no control characters; they are never trimmed). The trace id must be 32 and
// the span id 16 lowercase hexadecimal digits and neither may be all zeros. Every problem is reported together, in the
// order application, instance_id, execution_id, correlation_id, trace_id, span_id, as a *domain.ClassifiedError with
// the code event_schema_invalid that wraps ErrCallContextFieldInvalid; its detail names the field and the rule and
// never the value.
func NewCallContext(options ...Option) (CallContext, error) {
	call_context := CallContext{indentation: StartingIndentation}
	for _, option := range options {
		option(&call_context)
	}

	var field_failures []error
	field_failures = append(field_failures,
		identifier_failure(field_application, call_context.application),
		identifier_failure(field_instance_id, call_context.instance_id),
		identifier_failure(field_execution_id, call_context.execution_id),
		identifier_failure(field_correlation_id, call_context.correlation_id),
		hex_failure(field_trace_id, call_context.trace_id, domain.TraceIDWidth),
		hex_failure(field_span_id, call_context.span_id, domain.SpanIDWidth),
	)
	detail := domain.JoinErrors(field_failures...)
	if detail != nil {
		return CallContext{}, domain.NewClassifiedError(value_objects.ErrorCodeEventSchemaInvalid, ErrCallContextFieldInvalid, detail)
	}
	return call_context, nil
}

// WithApplication sets the name of the application.
func WithApplication(application_name string) Option {
	return func(call_context *CallContext) { call_context.application = application_name }
}

// WithInstanceID sets the id of the running instance of the application.
func WithInstanceID(instance_id string) Option {
	return func(call_context *CallContext) { call_context.instance_id = instance_id }
}

// WithExecutionID sets the id of the execution.
func WithExecutionID(execution_id string) Option {
	return func(call_context *CallContext) { call_context.execution_id = execution_id }
}

// WithCorrelationID sets the id that links related events.
func WithCorrelationID(correlation_id string) Option {
	return func(call_context *CallContext) { call_context.correlation_id = correlation_id }
}

// WithTraceID sets the W3C trace id: 32 lowercase hexadecimal digits, not all zeros.
func WithTraceID(trace_id string) Option {
	return func(call_context *CallContext) { call_context.trace_id = trace_id }
}

// WithSpanID sets the W3C span id: 16 lowercase hexadecimal digits, not all zeros.
func WithSpanID(span_id string) Option {
	return func(call_context *CallContext) { call_context.span_id = span_id }
}

// ----- METHODS --------------------------------------------------------------

// Application returns the name of the application, or an empty text when it is not set.
func (call_context CallContext) Application() string { return call_context.application }

// InstanceID returns the id of the instance, or an empty text when it is not set.
func (call_context CallContext) InstanceID() string { return call_context.instance_id }

// ExecutionID returns the id of the execution, or an empty text when it is not set.
func (call_context CallContext) ExecutionID() string { return call_context.execution_id }

// CorrelationID returns the correlation id, or an empty text when it is not set.
func (call_context CallContext) CorrelationID() string { return call_context.correlation_id }

// TraceID returns the trace id, or an empty text when it is not set.
func (call_context CallContext) TraceID() string { return call_context.trace_id }

// SpanID returns the span id, or an empty text when it is not set.
func (call_context CallContext) SpanID() string { return call_context.span_id }

// Indentation returns the depth of the call: StartingIndentation at the inbound adapter, one more for each deeper
// layer. The step inside one layer is not carried here: the caller reports it with each log line.
func (call_context CallContext) Indentation() int { return call_context.indentation }

// WithCallContext returns a copy of ctx that carries the given CallContext.
func WithCallContext(ctx context.Context, call_context CallContext) context.Context {
	return context.WithValue(ctx, callContextKey{}, call_context)
}

// FromContext returns the CallContext carried by ctx and whether there is one. A nil ctx has none.
func FromContext(ctx context.Context) (CallContext, bool) {
	if ctx == nil {
		return CallContext{}, false
	}
	call_context, found := ctx.Value(callContextKey{}).(CallContext)
	return call_context, found
}

// NextIndentation returns a copy of ctx one layer deeper (the indentation grows by one) and true; when ctx carries no
// CallContext it returns ctx unchanged and false. The original ctx is never modified.
func NextIndentation(ctx context.Context) (context.Context, bool) {
	call_context, found := FromContext(ctx)
	if !found {
		return ctx, false
	}

	call_context.indentation++
	return WithCallContext(ctx, call_context), true
}

// ----- VALIDATORS -----------------------------------------------------------

// identifier_failure returns nil for an empty text (not set) or a valid identifier, and otherwise the failure of the
// field, which names the field and the rule and never the text.
func identifier_failure(field string, text string) error {
	if text == "" {
		return nil
	}
	cause := base_value_objects.ValidateIdentifier(text)
	if cause == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", field, cause)
}

// hex_failure returns nil for an empty text (not set) or a valid W3C id, and otherwise the failure of the field. An id
// of only zeros is not valid.
func hex_failure(field string, text string, width int) error {
	if text == "" {
		return nil
	}
	cause := base_value_objects.ValidateHexDigits(text, width)
	if cause == nil && strings.Trim(text, "0") == "" {
		cause = ErrCallContextIDAllZeros
	}
	if cause == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", field, cause)
}
