package dependencies

import (
	uuidgen "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/uuidgen"
)

// NewUUIDGeneratorDependency builds the generator of the identifiers of the call context: random version 4 UUIDs.
func NewUUIDGeneratorDependency() *uuidgen.IDGenerator {
	return uuidgen.NewIDGenerator()
}
