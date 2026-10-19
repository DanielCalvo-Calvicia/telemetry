package outbound

import (
	"context"
	"time"
)

// =============================================================================
// CLOCK PORT
// =============================================================================

// ClockPort defines what the application requires to know the current time; the concrete clock is an
// infrastructure decision, so the application never calls time.Now itself.
type ClockPort interface {
	// Now returns the current time.
	Now(ctx context.Context) time.Time
}
