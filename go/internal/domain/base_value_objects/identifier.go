package base_value_objects

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ===== Identifier ===========================================================

// Identifier is a text that names something: it has at least one character that is not whitespace, no Unicode control
// character (category Cc) and it is valid UTF-8. It is never trimmed or changed. The empty Identifier is not valid;
// an absent optional field is modelled by the value object that holds it, not by an empty Identifier.
type Identifier string

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewIdentifier returns the Identifier of the text, or the sentinels of the rules that the text breaks.
func NewIdentifier(text string) (Identifier, error) {
	if validation_error := ValidateIdentifier(text); validation_error != nil {
		return "", validation_error
	}
	return Identifier(text), nil
}

// ----- METHODS --------------------------------------------------------------

// String returns the text of the identifier, unchanged.
func (identifier Identifier) String() string {
	return string(identifier)
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateIdentifier returns nil when the text is a valid Identifier. Otherwise it returns every rule that the text
// breaks (ErrIdentifierNotUTF8, ErrIdentifierBlank, ErrIdentifierHasControlCharacters), joined.
func ValidateIdentifier(text string) error {
	if !utf8.ValidString(text) {
		return ErrIdentifierNotUTF8
	}
	var broken_rules []error
	if strings.TrimFunc(text, unicode.IsSpace) == "" {
		broken_rules = append(broken_rules, ErrIdentifierBlank)
	}
	if strings.ContainsFunc(text, unicode.IsControl) {
		broken_rules = append(broken_rules, ErrIdentifierHasControlCharacters)
	}
	return JoinErrors(broken_rules...)
}
