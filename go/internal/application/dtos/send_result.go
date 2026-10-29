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
}
