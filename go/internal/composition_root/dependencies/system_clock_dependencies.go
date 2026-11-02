// Package dependencies builds the default adapters of the core, one file per adapter.
package dependencies

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/system_clock"
)

// NewSystemClock builds the default clock of the core: the clock of the machine.
func NewSystemClock() outbound.ClockPort {
	return system_clock.NewSystemClock()
}
