package dtos

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// CONFIGURATION DTO
// =============================================================================

// Configuration is the validated configuration that the consumer passes to the core (T-03.6): the identity of the
// resource, how events are delivered, the size limits, the local spool and the options of the masking. The core never
// reads a file or the environment; the consumer builds it in code or loads it with its own tool.
type Configuration struct {
	Resource value_objects.ResourceIdentity
	Delivery DeliverySettings
	Limits   value_objects.Limits
	Spool    SpoolSettings
	Masking  masking.MaskerOptions
}
