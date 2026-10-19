package dtos

import "time"

// =============================================================================
// LOG ENTRY OUTBOUND DTO
// =============================================================================

// LogEntryOutboundDTO carries one finished log entry to a log sink. Its flat shape, with these exact field
// names and no JSON tags, is the payload the console sink prints and the HTTP sink posts to the log receiver;
// it is the same payload as the old logger.
type LogEntryOutboundDTO struct {
	ProducerID    string
	ApplicationID string
	ProcessID     string
	Application   string
	Category      string
	Function      string
	Indentation   int
	Depth         int
	Level         string
	Message       string
	Fields        map[string]any
	Timestamp     time.Time
}
