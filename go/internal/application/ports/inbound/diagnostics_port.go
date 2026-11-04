package inbound

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"

// =============================================================================
// DIAGNOSTICS PORT
// =============================================================================

// DiagnosticsPort defines how a consumer reads how the delivery is doing (REQ-DGN-002).
type DiagnosticsPort interface {
	// Diagnostics returns a copy of the diagnostics of every destination and of the diagnostics channel. It never
	// panics and it still answers after the shutdown.
	Diagnostics() dtos.DiagnosticsSnapshot
}
