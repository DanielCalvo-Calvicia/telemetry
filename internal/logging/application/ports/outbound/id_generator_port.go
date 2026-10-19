package outbound

import "context"

// =============================================================================
// ID GENERATOR PORT
// =============================================================================

// IDGeneratorPort defines what the application requires to obtain a new, unique identifier (the application
// ID of a process start, the process ID of a request); the concrete strategy (UUID, ULID, ...) is an
// infrastructure decision.
type IDGeneratorPort interface {
	// NewID returns a new unique identifier, or an error when none could be generated.
	NewID(ctx context.Context) (string, error)
}
