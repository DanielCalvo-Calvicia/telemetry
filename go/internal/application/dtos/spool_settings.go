package dtos

// =============================================================================
// SPOOL SETTINGS DTO
// =============================================================================

// SpoolSettings are the settings of the local spool of the durable delivery mode: the directory that the consumer gave
// (empty when the mode does not need one) and the disk quota per destination, in bytes. The values are already
// validated.
type SpoolSettings struct {
	Directory  string
	QuotaBytes int64
}
