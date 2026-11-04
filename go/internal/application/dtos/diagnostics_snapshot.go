package dtos

// =============================================================================
// DIAGNOSTICS SNAPSHOT DTO
// =============================================================================

// DiagnosticsSnapshot is a copy of the diagnostics at one moment: one entry per destination, in the order of the
// destination stats (by kind, and by index inside a kind), and the counters of the diagnostics channel (REQ-DGN-001,
// REQ-DGN-002). It holds no message, no field and no credential of any event.
type DiagnosticsSnapshot struct {
	Destinations []DestinationDiagnostics
	Channel      ChannelDiagnostics
}
