// Package uuid_generator is the adapter of the identifier port that makes UUIDs of version 7 without a third-party
// library (REQ-ARC-005).
package uuid_generator

import (
	"context"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/uuid_generator/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/uuid_generator/external"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/uuid_generator/helpers"
)

// =============================================================================
// UUID GENERATOR
// =============================================================================

// UUIDGenerator implements outbound.IDGeneratorPort with UUIDs of version 7. The clock and the random source live in
// external/, never here directly. The time prefix never goes back inside one generator, even when the clock does; the
// order of two ids made in the same millisecond is not defined. It is safe for concurrent use.
type UUIDGenerator struct {
	read_milliseconds func() int64
	read_random       func(buffer []byte) error

	mutex             sync.Mutex
	last_milliseconds int64
}

var _ outbound.IDGeneratorPort = (*UUIDGenerator)(nil)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewUUIDGenerator returns a UUIDGenerator that reads the clock and the random source of the machine.
func NewUUIDGenerator() *UUIDGenerator {
	return NewUUIDGeneratorWithFunctions(external.ReadMilliseconds, external.ReadRandom)
}

// NewUUIDGeneratorWithFunctions returns a UUIDGenerator that reads the unix time in milliseconds with
// read_milliseconds and fills a buffer with random bytes with read_random; tests use it to control both and to
// exercise the failure of the random source.
func NewUUIDGeneratorWithFunctions(read_milliseconds func() int64, read_random func(buffer []byte) error) *UUIDGenerator {
	return &UUIDGenerator{read_milliseconds: read_milliseconds, read_random: read_random}
}

// ----- METHODS --------------------------------------------------------------

// NewEventID returns a new event id: a UUID of version 7. A failure of the random source is a classified error
// (event_build_failed) that wraps application.ErrIDGenerationFailed and keeps the cause; there is no fallback to a
// weaker source.
func (uuid_generator *UUIDGenerator) NewEventID(_ context.Context) (value_objects.EventID, error) {
	uuid_text, generate_error := uuid_generator.generate()
	if generate_error != nil {
		return value_objects.EventID{}, generate_error
	}
	return value_objects.NewEventID(uuid_text)
}

// NewInstanceID returns the text of a new UUID of version 7 as the instance id of a resource, with the same failure as
// NewEventID.
func (uuid_generator *UUIDGenerator) NewInstanceID(_ context.Context) (base_value_objects.Identifier, error) {
	uuid_text, generate_error := uuid_generator.generate()
	if generate_error != nil {
		return "", generate_error
	}
	return base_value_objects.NewIdentifier(uuid_text)
}

// ----- HELPERS --------------------------------------------------------------

// generate makes the text of one UUID of version 7.
func (uuid_generator *UUIDGenerator) generate() (string, error) {
	var random_bytes [helpers.RandomByteCount]byte
	if read_error := uuid_generator.read_random(random_bytes[:]); read_error != nil {
		return "", adapter_error.MapGenerateError(read_error)
	}
	return helpers.LayoutUUIDv7(uuid_generator.next_milliseconds(), random_bytes), nil
}

// next_milliseconds returns the time of the clock, or the last one that was used when the clock went back.
func (uuid_generator *UUIDGenerator) next_milliseconds() int64 {
	milliseconds := uuid_generator.read_milliseconds()
	uuid_generator.mutex.Lock()
	defer uuid_generator.mutex.Unlock()
	if milliseconds < uuid_generator.last_milliseconds {
		milliseconds = uuid_generator.last_milliseconds
	}
	uuid_generator.last_milliseconds = milliseconds
	return milliseconds
}
