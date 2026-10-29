package dtos

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// SPOOL RECORD DTO
// =============================================================================

// SpoolRecord is an event stored in the local spool with the id that the spool gave it. The id is what a caller
// acknowledges once the event is delivered.
type SpoolRecord struct {
	ID    string
	Event value_objects.Envelope
}
