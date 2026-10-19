package uuidgen

import (
	"context"

	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/uuidgen/error"
	external "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/uuidgen/external"
)

// =============================================================================
// ID GENERATOR
// =============================================================================

// IDGenerator implements ports_outbound.IDGeneratorPort with random (version 4) UUIDs; the call into
// google/uuid lives in external/, never here directly.
type IDGenerator struct {
	generate func() (string, error)
}

var _ ports_outbound.IDGeneratorPort = (*IDGenerator)(nil)

// NewIDGenerator creates an IDGenerator that generates random UUIDs.
func NewIDGenerator() *IDGenerator {
	return NewIDGeneratorWithFunction(external.GenerateUUID)
}

// NewIDGeneratorWithFunction creates an IDGenerator that calls the given function to generate each identifier;
// tests use it to exercise the failure path.
func NewIDGeneratorWithFunction(generate func() (string, error)) *IDGenerator {
	return &IDGenerator{generate: generate}
}

// NewID returns a new identifier; a failure is translated to application.ErrIDGenerationFailed and keeps its
// root cause. Unlike the old code there is no fallback that fakes a UUID with a weaker random source: the
// caller decides what to do when no identifier can be generated.
func (id_generator *IDGenerator) NewID(_ context.Context) (string, error) {
	new_id, err := id_generator.generate()
	if err != nil {
		return "", adapter_error.MapGenerateError(err)
	}
	return new_id, nil
}
