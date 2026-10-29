package dtos

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// CONFIGURATION DTO
// =============================================================================

// Configuration is the configuration that the consumer passes to the core. It holds the identity of the resource for
// now; T-03.6 completes it (profiles, limits, destinations and delivery).
type Configuration struct {
	Resource value_objects.ResourceIdentity
}
