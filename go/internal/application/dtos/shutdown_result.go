package dtos

// =============================================================================
// SHUTDOWN RESULT DTO
// =============================================================================

// ShutdownResult is what the core answers when it is closed: how many events were sent, how many were written to the
// spool and how many were dropped while it closed. It is all zero until T-05.4 gives the core a queue to flush.
type ShutdownResult struct {
	Sent    int
	Spooled int
	Dropped int
}
