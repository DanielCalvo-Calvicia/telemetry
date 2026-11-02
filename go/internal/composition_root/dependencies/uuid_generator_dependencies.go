package dependencies

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/uuid_generator"
)

// NewUUIDGenerator builds the default identifier generator of the core: UUIDs of version 7.
func NewUUIDGenerator() outbound.IDGeneratorPort {
	return uuid_generator.NewUUIDGenerator()
}
