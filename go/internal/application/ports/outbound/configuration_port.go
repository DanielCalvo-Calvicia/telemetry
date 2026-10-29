package outbound

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
)

// =============================================================================
// CONFIGURATION PORT
// =============================================================================

// ConfigurationPort defines how the core gets its configuration. It hands the core the configuration that the
// consumer passed in; it reads no file and no environment (REQ-ARC-007, ADR-004).
type ConfigurationPort interface {
	// Configuration returns the configuration the consumer passed in.
	Configuration(ctx context.Context) (dtos.Configuration, error)
}
