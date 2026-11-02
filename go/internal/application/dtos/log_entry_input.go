package dtos

// =============================================================================
// LOG ENTRY INPUT DTO
// =============================================================================

// LogEntryInput is what a caller gives the logger for one line: the category, the optional function, the message and
// the extra fields. It is plain data. The application name is not here: it is resource.service_name, from the
// configuration. The severity is the method that is called.
//
// Nothing else is recorded: no request body and no payload is captured automatically (REQ-LOG-006). Only what the
// caller puts in Fields reaches the event.
type LogEntryInput struct {
	// Category is what the line is about (for example http). Any text, an empty one included.
	Category string
	// Function is the function that logged the line. Empty means not given: the event then has no function.
	Function string
	// Message is the text of the line. Any text, an empty one included.
	Message string
	// Fields are the extra key-value pairs of the line; they become the attributes of the event.
	Fields map[string]any
}
