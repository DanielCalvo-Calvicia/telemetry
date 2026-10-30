package call_context

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// REQUEST CONTEXT
// =============================================================================

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrCallContextIDFailed is application.ErrCallContextIDFailed.
	ErrCallContextIDFailed = application.ErrCallContextIDFailed
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewRequestContext returns a copy of ctx that carries a new CallContext for a request: the given application name and
// instance id, a new correlation id from the generator and the starting indentation. The old process id of a request
// is now the correlation id. The id is the text of a new event id (a UUID of version 7), which is unique; the port may
// get its own method for it later. A failure to generate the id is returned with ctx unchanged, as a
// *domain.ClassifiedError with the code event_build_failed (an internal failure of a port) that wraps
// ErrCallContextIDFailed and keeps its root cause; the caller decides whether the request goes on without a call
// context. An invalid application name or instance id is returned as NewCallContext returns it.
func NewRequestContext(ctx context.Context, id_generator outbound.IDGeneratorPort, application_name string, instance_id string) (context.Context, error) {
	correlation_id, id_error := id_generator.NewEventID(ctx)
	if id_error != nil {
		return ctx, domain.NewClassifiedError(value_objects.ErrorCodeEventBuildFailed, ErrCallContextIDFailed, id_error)
	}

	call_context, build_error := NewCallContext(
		WithApplication(application_name),
		WithInstanceID(instance_id),
		WithCorrelationID(correlation_id.String()),
	)
	if build_error != nil {
		return ctx, build_error
	}
	return WithCallContext(ctx, call_context), nil
}
