package value_objects

// ===== optional_field =======================================================

// optional_field is a field that a value object may not have. Absent and present-but-empty are different: the
// contract gives every optional text a minimum length of 1, so an empty text that was given is an error and an
// absent one is not. A zero optional_field is absent.
type optional_field[Value ~string] struct {
	value      Value
	is_present bool
}

// present_field returns an optional_field that holds the value, even when it is empty.
func present_field[Value ~string](value Value) optional_field[Value] {
	return optional_field[Value]{value: value, is_present: true}
}

// get returns the value and whether it is present.
func (field optional_field[Value]) get() (Value, bool) {
	return field.value, field.is_present
}

// validate_if_present returns nil for an absent field and the result of the validator for a present one.
func (field optional_field[Value]) validate_if_present(validator func(text string) error) error {
	if !field.is_present {
		return nil
	}
	return validator(string(field.value))
}
