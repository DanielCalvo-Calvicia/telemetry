package base_value_objects

import "strings"

// ===== HexDigits ============================================================

// HexDigits is a text of lowercase hexadecimal digits of an exact width, such as the trace id (32) and the span id
// (16) of the W3C trace context. The width is a rule of the construction; the value does not remember it.
type HexDigits string

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewHexDigits returns the HexDigits of the text, or the sentinels of the rules that the text breaks.
func NewHexDigits(value string, width int) (HexDigits, error) {
	if validation_error := ValidateHexDigits(value, width); validation_error != nil {
		return "", validation_error
	}
	return HexDigits(value), nil
}

// ----- METHODS --------------------------------------------------------------

// String returns the digits, unchanged.
func (hex_digits HexDigits) String() string {
	return string(hex_digits)
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateHexDigits returns nil when the value has exactly width characters and each one is a lowercase hexadecimal
// digit. A width below 1 is never satisfied. Otherwise it returns ErrHexDigitsInvalid, ErrHexDigitsWrongWidth or both.
func ValidateHexDigits(value string, width int) error {
	var broken_rules []error
	if strings.Trim(value, LowercaseHexDigits) != "" {
		broken_rules = append(broken_rules, ErrHexDigitsInvalid)
	}
	if width < 1 || len(value) != width {
		broken_rules = append(broken_rules, ErrHexDigitsWrongWidth)
	}
	return JoinErrors(broken_rules...)
}
