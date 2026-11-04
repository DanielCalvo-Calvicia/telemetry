package dtos

// =============================================================================
// SEND RESULT DTO
// =============================================================================

// SendResult is what a sink or the spool answers to a call: how many events were accepted, dropped or failed. Each
// event of the call is counted once. Reason says why events were dropped or failed and is empty when none were
// (REQ-DLV-008 counts the drops per reason).
type SendResult struct {
	Accepted int
	Dropped  int
	Failed   int
	Reason   string
	// RetryAfterMS is the wait in milliseconds that the destination asked for before the events that failed are sent again
	// (the Retry-After of an HTTP answer): it is set only when events failed with a transient failure and the destination
	// gave the hint, and 0 means no hint.
	RetryAfterMS int64
}
