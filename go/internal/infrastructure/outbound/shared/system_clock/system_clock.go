// Package system_clock is the adapter of the clock port that reads the clock of the machine.
package system_clock

import (
	"context"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/system_clock/external"
)

// =============================================================================
// SYSTEM CLOCK
// =============================================================================

// SystemClock implements outbound.ClockPort with the clock of the machine; reading it lives in external/, never here
// directly.
type SystemClock struct {
	read_time      func() time.Time
	read_monotonic func() int64
}

var _ outbound.ClockPort = (*SystemClock)(nil)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewSystemClock returns a SystemClock that reads the clock of the machine. Its monotonic reading counts from the
// moment it was created.
func NewSystemClock() *SystemClock {
	return NewSystemClockWithFunctions(external.ReadTime, external.NewMonotonicReader())
}

// NewSystemClockWithFunctions returns a SystemClock that reads the wall time with read_time and the monotonic reading
// with read_monotonic; tests use it to control both.
func NewSystemClockWithFunctions(read_time func() time.Time, read_monotonic func() int64) *SystemClock {
	return &SystemClock{read_time: read_time, read_monotonic: read_monotonic}
}

// ----- METHODS --------------------------------------------------------------

// Now returns the wall time in UTC, with microsecond precision. A time that the domain refuses (a year outside 1 to
// 9999) is the zero instant of the domain, never a panic.
func (system_clock *SystemClock) Now(_ context.Context) value_objects.UTCInstant {
	utc_instant, instant_error := value_objects.NewUTCInstant(system_clock.read_time().UTC())
	if instant_error != nil {
		return value_objects.UTCInstant{}
	}
	return utc_instant
}

// MonotonicNanoseconds returns the nanoseconds elapsed since the clock was created; the reading never goes back.
func (system_clock *SystemClock) MonotonicNanoseconds(_ context.Context) int64 {
	return system_clock.read_monotonic()
}

// ClockInfo returns an empty ClockInfo: the system clock does not know its source nor whether it is synchronized, so
// the core writes no clock attribute for it (an unknown value is absent, never false).
func (system_clock *SystemClock) ClockInfo(_ context.Context) dtos.ClockInfo {
	return dtos.ClockInfo{}
}
