// Package inbound holds the ports that a project calls to use the core.
package inbound

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
)

// =============================================================================
// LOGGER PORT
// =============================================================================

// LoggerPort defines the capability of recording one log line at a severity. Each method builds a log.entry.recorded
// event and hands it to the log sinks through the pipeline of the core.
//
// No method returns a value, on purpose: a log call never returns an error and never panics (ADR-005, REQ-DLV-002).
// An event that is rejected (invalid, too big, no sink) is counted in the diagnostics of the core (T-05.5); the caller
// of the logger never has to handle a failure of the telemetry. A call does not stop the process at CRITICAL.
//
// The call context that ctx carries (trace, span, execution) is attached to the event by the pipeline. No request body
// and no payload is captured: only what the caller passes in the fields of the entry (REQ-LOG-006).
type LoggerPort interface {
	// Trace records the entry at the severity TRACE.
	Trace(ctx context.Context, entry dtos.LogEntryInput)
	// Debug records the entry at the severity DEBUG.
	Debug(ctx context.Context, entry dtos.LogEntryInput)
	// Info records the entry at the severity INFO.
	Info(ctx context.Context, entry dtos.LogEntryInput)
	// Warn records the entry at the severity WARN.
	Warn(ctx context.Context, entry dtos.LogEntryInput)
	// Error records the entry at the severity ERROR.
	Error(ctx context.Context, entry dtos.LogEntryInput)
	// Critical records the entry at the severity CRITICAL. It does not stop the process.
	Critical(ctx context.Context, entry dtos.LogEntryInput)
}
