package dtos

// =============================================================================
// CHANNEL DIAGNOSTICS DTO
// =============================================================================

// ChannelDiagnostics are the counters of the diagnostics channel itself (REQ-DGN-003). Published is the diagnostics that
// passed the rate limit and were queued for the callbacks; Suppressed is the ones the rate limit refused; Dropped is the
// ones that found the queue of the channel full; CallbackErrors is the times that a callback or the writer panicked.
type ChannelDiagnostics struct {
	Published      int
	Suppressed     int
	Dropped        int
	CallbackErrors int
}
