package inbound

import (
	"context"

	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
)

// =============================================================================
// LOGGER PORT
// =============================================================================

// LoggerPort defines what any package of the project can ask of the logger: one method per severity.
// Services and adapters of every AWS package import this port directly (D4=B).
type LoggerPort interface {
	// Trace logs an entry at TRACE level.
	Trace(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error
	// Debug logs an entry at DEBUG level.
	Debug(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error
	// Info logs an entry at INFO level.
	Info(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error
	// Warn logs an entry at WARN level.
	Warn(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error
	// Error logs an entry at ERROR level.
	Error(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error
	// Fatal logs an entry at FATAL level; it does not stop the process.
	Fatal(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error
}
