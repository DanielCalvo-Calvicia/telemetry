package dtos

// =============================================================================
// SPOOL CLEANUP DTO
// =============================================================================

// SpoolCleanup is what a cleanup of the local spool removed: the number of events and the bytes it freed.
type SpoolCleanup struct {
	RemovedEvents int
	FreedBytes    int64
}
