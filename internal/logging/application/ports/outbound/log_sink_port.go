package outbound

import (
	"context"

	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
)

// =============================================================================
// LOG SINK PORT
// =============================================================================

// LogSinkPort defines what the logger requires from a destination of log entries (the console, a log receiver).
type LogSinkPort interface {
	// Send delivers one finished log entry to the destination.
	Send(ctx context.Context, log_entry dtos.LogEntryOutboundDTO) error
}
