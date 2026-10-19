package call_depth

import (
	"context"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
)

// =============================================================================
// CALL DEPTH
// =============================================================================

// ----------------------------------------------------------------------------
// ERRORS
// ----------------------------------------------------------------------------
var (
	ErrCallDepthProcessIDFailed = application.ErrCallDepthProcessIDFailed
)

// CallContext is the identity and position of a request in the call tree, carried through context.Context
// so that every log line of the request can tell who emitted it and how deep in the call it was written.
// Application is the application name, ApplicationID identifies the running instance (one per process
// start), ProcessID identifies the request (one per request) and Indentation starts at 1 at the inbound
// adapter and grows by one each time the call moves to a deeper layer. The step inside one layer (Depth)
// is not carried here: the caller reports it with each log line.
type CallContext struct {
	Application   string
	ApplicationID string
	ProcessID     string
	Indentation   int
}

// callContextKey is the key under which the CallContext is stored; being a private type, no other
// package can read or overwrite the value by accident.
type callContextKey struct{}

// StartingIndentation is the indentation of the first layer, the inbound adapter.
const StartingIndentation = 1

// ----------------------------------------------------------------------------
// CONSTRUCTORS
// ----------------------------------------------------------------------------

// NewRequestContext returns a copy of ctx that carries a new CallContext for a request: the given application
// name and instance ID, a new process ID from the generator and the starting indentation. A failure to
// generate the process ID is returned with ErrCallDepthProcessIDFailed and keeps its root cause; the
// caller decides whether the request goes on without call-depth context.
func NewRequestContext(ctx context.Context, id_generator ports_outbound.IDGeneratorPort, application_name string, application_id string) (context.Context, error) {
	process_id, err := id_generator.NewID(ctx)
	if err != nil {
		return ctx, application.JoinErrors(ErrCallDepthProcessIDFailed, err)
	}

	return WithCallContext(ctx, CallContext{
		Application:   application_name,
		ApplicationID: application_id,
		ProcessID:     process_id,
		Indentation:   StartingIndentation,
	}), nil
}

// ----------------------------------------------------------------------------
// METHODS
// ----------------------------------------------------------------------------

// WithCallContext returns a copy of ctx that carries the given CallContext.
func WithCallContext(ctx context.Context, call_context CallContext) context.Context {
	return context.WithValue(ctx, callContextKey{}, call_context)
}

// FromContext returns the CallContext carried by ctx and whether there is one.
func FromContext(ctx context.Context) (CallContext, bool) {
	call_context, found := ctx.Value(callContextKey{}).(CallContext)
	return call_context, found
}

// NextIndentation returns a copy of ctx one layer deeper (the indentation grows by one) and true; when ctx
// carries no CallContext it returns ctx unchanged and false. The original ctx is never modified.
func NextIndentation(ctx context.Context) (context.Context, bool) {
	call_context, found := FromContext(ctx)
	if !found {
		return ctx, false
	}

	call_context.Indentation++
	return WithCallContext(ctx, call_context), true
}
