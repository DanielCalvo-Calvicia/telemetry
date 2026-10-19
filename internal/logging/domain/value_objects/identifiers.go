package value_objects

// =============================================================================
// Identifiers
// =============================================================================

// Identifiers tell which producer, application instance and request a log entry belongs to.
// ProducerID identifies who emits the log, ApplicationID the running instance (one per process start)
// and ProcessID the request being served (one per request).
type Identifiers struct {
	ProducerID    string
	ApplicationID string
	ProcessID     string
}

// -----------------------------------------------------------------------------
// CONSTRUCTORS
// -----------------------------------------------------------------------------

// NewIdentifiers creates the Identifiers of a log entry.
func NewIdentifiers(producer_id string, application_id string, process_id string) Identifiers {
	return Identifiers{
		ProducerID:    producer_id,
		ApplicationID: application_id,
		ProcessID:     process_id,
	}
}
