package dtos

// =============================================================================
// CLOCK INFO DTO
// =============================================================================

// ClockInfo says what the clock is. A value is only used when its Known flag is true: when a flag is not known, its
// key is absent from the attributes of the event, never false.
type ClockInfo struct {
	Source      string
	SourceKnown bool
	Synced      bool
	SyncedKnown bool
}
