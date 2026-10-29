package value_objects

import (
	"maps"
	"reflect"
	"slices"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// ===== EventAttributes ======================================================

// EventAttributes are the free key-value pairs of an event. A key is a non-empty text. A value is a string, a boolean,
// a number (any Go integer or float type) or a flat list of those; a null, a nested object, a list inside a list and a
// list that holds an object are rejected (REQ-CTR-010). The limits of size (keys, list length, text length) are not
// checked here (T-03.5). The wire type of the contract is Attributes (generated); this is its validated form.
//
// The value owns a copy of the map that it was built from, and every accessor returns a copy, so no caller can change
// it afterwards.
type EventAttributes struct {
	entries map[string]any
}

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrAttributeKeyInvalid is domain.ErrAttributeKeyInvalid.
	ErrAttributeKeyInvalid = domain.ErrAttributeKeyInvalid
	// ErrAttributeValueInvalid is domain.ErrAttributeValueInvalid.
	ErrAttributeValueInvalid = domain.ErrAttributeValueInvalid
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewEventAttributes returns the EventAttributes of the entries, or the sentinels (ErrAttributeKeyInvalid,
// ErrAttributeValueInvalid) of every entry that breaks a rule, joined. A nil map is the empty attributes. A list is
// kept as a []any with the same elements, whatever its Go type was.
func NewEventAttributes(entries map[string]any) (EventAttributes, error) {
	if validation_error := ValidateEventAttributes(entries); validation_error != nil {
		return EventAttributes{}, validation_error
	}
	return EventAttributes{entries: copy_attribute_entries(entries)}, nil
}

// ----- METHODS --------------------------------------------------------------

// Len returns the number of attributes.
func (attributes EventAttributes) Len() int {
	return len(attributes.entries)
}

// Get returns the value of a key, as a copy, and whether the key exists.
func (attributes EventAttributes) Get(key string) (any, bool) {
	value, exists := attributes.entries[key]
	if !exists {
		return nil, false
	}
	return copy_attribute_value(value), true
}

// Entries returns a copy of the attributes. Changing it does not change the EventAttributes.
func (attributes EventAttributes) Entries() map[string]any {
	return copy_attribute_entries(attributes.entries)
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateEventAttributes returns nil when every key is non-empty and every value is a scalar or a flat list of
// scalars. Otherwise it returns ErrAttributeKeyInvalid and ErrAttributeValueInvalid for the entries that break a rule,
// joined, in the order of the keys.
func ValidateEventAttributes(entries map[string]any) error {
	var broken_rules []error
	for _, key := range slices.Sorted(maps.Keys(entries)) {
		if key == "" {
			broken_rules = append(broken_rules, ErrAttributeKeyInvalid)
		}
		if !is_attribute_value(entries[key]) {
			broken_rules = append(broken_rules, ErrAttributeValueInvalid)
		}
	}
	return JoinErrors(broken_rules...)
}

// is_attribute_value reports whether the value is a scalar or a flat list of scalars.
func is_attribute_value(value any) bool {
	if is_attribute_scalar(value) {
		return true
	}
	if value == nil {
		return false
	}
	list := reflect.ValueOf(value)
	if list.Kind() != reflect.Slice {
		return false
	}
	for position := 0; position < list.Len(); position++ {
		if !is_attribute_scalar(list.Index(position).Interface()) {
			return false
		}
	}
	return true
}

// is_attribute_scalar reports whether the value is a string, a boolean or a number.
func is_attribute_scalar(value any) bool {
	if value == nil {
		return false
	}
	switch reflect.TypeOf(value).Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// copy_attribute_entries copies a map of valid attributes; a list is copied into a new []any.
func copy_attribute_entries(entries map[string]any) map[string]any {
	copied := make(map[string]any, len(entries))
	for key, value := range entries {
		copied[key] = copy_attribute_value(value)
	}
	return copied
}

// copy_attribute_value returns the value as it is when it is a scalar and a new []any when it is a list.
func copy_attribute_value(value any) any {
	if is_attribute_scalar(value) {
		return value
	}
	list := reflect.ValueOf(value)
	copied := make([]any, list.Len())
	for position := range copied {
		copied[position] = list.Index(position).Interface()
	}
	return copied
}
