package outbound

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// CLOCK PORT
// =============================================================================

// ClockPort defines the time the core needs: the wall time in UTC, a monotonic reading and, when known, what the
// clock is (REQ-CTR-007, REQ-CTR-008).
type ClockPort interface {
	// Now returns the wall time as a UTC instant.
	Now(ctx context.Context) value_objects.UTCInstant

	// MonotonicNanoseconds returns a reading, in nanoseconds, from an arbitrary fixed point that never goes back.
	// The difference of two readings is the elapsed time.
	MonotonicNanoseconds(ctx context.Context) int64

	// ClockInfo returns the source of the clock and whether it is synchronized, each one only when it is known.
	ClockInfo(ctx context.Context) dtos.ClockInfo
}
