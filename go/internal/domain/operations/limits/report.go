package limits

// ===== Report ===============================================================

// Report counts what ApplyLimits cut from one event. It holds integers only: the cut text is never kept, because it
// may be sensitive. The counters are returned with the result of the call; T-05.5 exposes them as diagnostics.
type Report struct {
	// AttributeKeysDropped is the number of attributes removed: the keys beyond the limit and the keys that were the
	// same as a key already kept after they were cut.
	AttributeKeysDropped int
	// AttributeKeysTruncated is the number of kept keys that were cut to the limit of characters.
	AttributeKeysTruncated int
	// AttributeValuesTruncated is the number of text values (each text of a list counts on its own) that were cut to
	// the limit of characters.
	AttributeValuesTruncated int
	// EventsRejected is 1 when the event was still larger than the limit after the cuts, and 0 otherwise.
	EventsRejected int
}
