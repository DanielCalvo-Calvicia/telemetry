package value_objects

// =============================================================================
// Fields
// =============================================================================

// Fields holds the structured key-value data of a log entry.
type Fields map[string]any

// -----------------------------------------------------------------------------
// CONSTRUCTORS
// -----------------------------------------------------------------------------

// NewFields creates Fields with a copy of the given map; a nil map gives empty, usable Fields.
func NewFields(fields_to_copy map[string]any) Fields {
	copied_fields := make(Fields, len(fields_to_copy))
	for key, value := range fields_to_copy {
		copied_fields[key] = value
	}
	return copied_fields
}

// -----------------------------------------------------------------------------
// METHODS
// -----------------------------------------------------------------------------

// Merge returns new Fields with the pairs of both; other takes precedence. Neither input is modified or shared.
func (fields Fields) Merge(other Fields) Fields {
	merged_fields := make(Fields, len(fields)+len(other))
	for key, value := range fields {
		merged_fields[key] = value
	}
	for key, value := range other {
		merged_fields[key] = value
	}
	return merged_fields
}

// Add stores a pair in place and returns the Fields; on nil Fields it returns new Fields, so always use the result.
func (fields Fields) Add(key string, value any) Fields {
	if fields == nil {
		fields = make(Fields)
	}

	fields[key] = value
	return fields
}
