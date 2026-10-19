package systemclock

import (
	"context"
	"time"

	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	external "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/systemclock/external"
)

// =============================================================================
// SYSTEM CLOCK
// =============================================================================

// SystemClock implements ports_outbound.ClockPort with the clock of the machine; reading it lives in
// external/, never here directly.
type SystemClock struct{}

var _ ports_outbound.ClockPort = (*SystemClock)(nil)

// NewSystemClock creates a SystemClock.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now returns the current time of the machine.
func (system_clock *SystemClock) Now(_ context.Context) time.Time {
	return external.ReadTime()
}
