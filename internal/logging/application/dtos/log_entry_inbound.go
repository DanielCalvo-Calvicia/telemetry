package dtos

// =============================================================================
// LOG ENTRY INBOUND DTO
// =============================================================================

// LogEntryInboundDTO carries what a caller (a service or an adapter of any AWS package) tells the logger
// about one log line. The level is chosen by the logger method that receives it and the timestamp is set by
// the logger, so neither is part of the DTO. It is a plain struct, not an alias of a pkg/dtos type, because
// telemetry is internal and has no externally reachable inbound adapter.
type LogEntryInboundDTO struct {
	ProducerID    string
	ApplicationID string
	ProcessID     string
	Application   string
	Category      string
	Function      string
	Indentation   int
	Depth         int
	Message       string
	Fields        map[string]any
}
