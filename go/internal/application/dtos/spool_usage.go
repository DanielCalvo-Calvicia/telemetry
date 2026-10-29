package dtos

// =============================================================================
// SPOOL USAGE DTO
// =============================================================================

// SpoolUsage is how much the local spool holds: the number of events, their size in bytes and the quota in bytes.
type SpoolUsage struct {
	Events     int
	Bytes      int64
	QuotaBytes int64
}
