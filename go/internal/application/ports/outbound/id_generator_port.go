package outbound

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// ID GENERATOR PORT
// =============================================================================

// IDGeneratorPort defines how the core gets new identifiers: the id of an event and the instance id of a resource.
type IDGeneratorPort interface {
	// NewEventID returns a new event id: a UUID of version 7.
	NewEventID(ctx context.Context) (value_objects.EventID, error)

	// NewInstanceID returns the instance id of a resource, used when the consumer gives none.
	NewInstanceID(ctx context.Context) (base_value_objects.Identifier, error)
}
