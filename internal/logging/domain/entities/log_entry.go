package entities

import (
	"time"

	domain "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
	value_objects "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

// =============================================================================
// LOG ENTRY ENTITY
// =============================================================================

// LogEntry represents one structured log record: who emitted it, how severe it is, what it says, where it
// came from and where it sits in the call tree of the request.
type LogEntry struct {
	Identifiers value_objects.Identifiers      `json:"identifiers"`
	Level       value_objects.Level            `json:"level"`
	Message     value_objects.Message          `json:"message"`
	Fields      value_objects.Fields           `json:"fields,omitempty"`
	Timestamp   time.Time                      `json:"timestamp"`
	Location    value_objects.Location         `json:"location"`
	Indentation value_objects.IndentationDepth `json:"indentation"`
}

// ----------------------------------------------------------------------------
// ERRORS
// ----------------------------------------------------------------------------
var (
	ErrLogEntryCreationFailed = domain.ErrLogEntryCreationFailed
)

// ----------------------------------------------------------------------------
// CONSTRUCTORS
// ----------------------------------------------------------------------------

// NewLogEntry is the normal validated construction path: primitives in, every value object validated, the
// first failure wrapped with ErrLogEntryCreationFailed. The fields are copied, so the entry never shares
// the caller's map.
func NewLogEntry(
	producer_id string,
	application_id string,
	process_id string,
	application string,
	category string,
	function string,
	indentation int,
	depth int,
	level string,
	message string,
	fields map[string]any,
	timestamp time.Time,
) (LogEntry, error) {
	log_level, err := value_objects.NewLevel(level)
	if err != nil {
		return LogEntry{}, domain.JoinErrors(err, ErrLogEntryCreationFailed)
	}

	return LogEntry{
		Identifiers: value_objects.NewIdentifiers(producer_id, application_id, process_id),
		Level:       log_level,
		Message:     value_objects.NewMessage(message),
		Fields:      value_objects.NewFields(fields),
		Timestamp:   timestamp,
		Location:    value_objects.NewLocation(application, category, function),
		Indentation: value_objects.NewIndentation(indentation, depth),
	}, nil
}

// ----------------------------------------------------------------------------
// METHODS
// ----------------------------------------------------------------------------

// AddFields returns a copy of the entry with one more field; the original entry and its fields are not modified.
func (log_entry LogEntry) AddFields(key string, field any) LogEntry {
	log_entry.Fields = log_entry.Fields.Merge(value_objects.Fields{key: field})
	return log_entry
}

// NextIndentation returns a copy of the entry one layer deeper in the call tree.
func (log_entry LogEntry) NextIndentation() LogEntry {
	log_entry.Indentation = log_entry.Indentation.NextIndentation()
	return log_entry
}

// NextDepth returns a copy of the entry one step further inside the same layer.
func (log_entry LogEntry) NextDepth() LogEntry {
	log_entry.Indentation = log_entry.Indentation.NextDepth()
	return log_entry
}

// WithCategory returns a copy of the entry with the category set.
func (log_entry LogEntry) WithCategory(category string) LogEntry {
	log_entry.Location = log_entry.Location.WithCategory(category)
	return log_entry
}

// WithFunction returns a copy of the entry with the function name set.
func (log_entry LogEntry) WithFunction(function string) LogEntry {
	log_entry.Location = log_entry.Location.WithFunction(function)
	return log_entry
}
