package dtos

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// NEW EVENT INPUT DTO
// =============================================================================

// NewEventInput is what a capability gives the core to build one event. It is plain data: the fields that are values
// of the domain were validated by their constructors.
//
// There is NO field for recorded_at, on purpose: only the library sets it, from the clock port, so a caller cannot
// supply it (REQ-CTR-008, ADR-012). The same goes for event_id, schema_version and resource, which the core sets.
type NewEventInput struct {
	// EventType is the type of the event, as in the registry (for example log.entry.recorded).
	EventType string
	// Severity is the severity of the event.
	Severity value_objects.Severity
	// Data is the payload; it must match the schema of the event type. A nil map is the same as an empty one.
	Data map[string]any
	// OccurredAt is when the event happened. Nil means the time of the call. A data.observed_at overrides it.
	OccurredAt *value_objects.UTCInstant
	// Actor is who produced the event. Nil means no actor.
	Actor *value_objects.ActorIdentity
	// Context links the event to a trace and to the work that produced it. Nil means no context.
	Context *value_objects.CorrelationContext
	// Attributes are the extensions of the consumer. Nil means none.
	Attributes *value_objects.EventAttributes
}
