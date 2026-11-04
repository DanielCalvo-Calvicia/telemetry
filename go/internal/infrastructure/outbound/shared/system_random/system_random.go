// Package system_random is the adapter of the random port that reads the random source of the machine.
package system_random

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/system_random/external"
)

// =============================================================================
// SYSTEM RANDOM
// =============================================================================

// SystemRandom implements outbound.RandomPort with the random source of the machine; reading it lives in external/,
// never here directly.
type SystemRandom struct {
	read_float64 func() float64
}

var _ outbound.RandomPort = (*SystemRandom)(nil)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewSystemRandom returns a SystemRandom that reads the random source of the machine.
func NewSystemRandom() *SystemRandom {
	return NewSystemRandomWithFunction(external.ReadFloat64)
}

// NewSystemRandomWithFunction returns a SystemRandom that reads its numbers with read_float64; tests use it to control
// the source.
func NewSystemRandomWithFunction(read_float64 func() float64) *SystemRandom {
	return &SystemRandom{read_float64: read_float64}
}

// ----- METHODS --------------------------------------------------------------

// Float64 returns a number in [0, 1]. A number the source gives outside that range (or not a number) is cut to it, so
// a caller can trust the range.
func (system_random *SystemRandom) Float64() float64 {
	number := system_random.read_float64()
	if number != number || number < 0 {
		return 0
	}
	if number > 1 {
		return 1
	}
	return number
}
