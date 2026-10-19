package value_objects

// =============================================================================
// Message
// =============================================================================

// Message is the text of a log entry; any text, including an empty one, is valid.
type Message string

// -----------------------------------------------------------------------------
// CONSTRUCTORS
// -----------------------------------------------------------------------------

// NewMessage creates a Message.
func NewMessage(raw_message string) Message {
	return Message(raw_message)
}

// -----------------------------------------------------------------------------
// METHODS
// -----------------------------------------------------------------------------

// String returns the message as text.
func (message Message) String() string {
	return string(message)
}
