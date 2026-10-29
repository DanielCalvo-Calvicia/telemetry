package base_value_objects

import "errors"

// #### BASE VALUE OBJECT ERRORS

// The sentinels of this package. They are local on purpose: this package imports no other domain package, so it
// cannot use the sentinels of the domain root. They are provisional; the mapping to the catalog codes is T-03.7.

// === Identifier ===

var (
	// ErrIdentifierBlank is returned for a text that has no character other than whitespace, the empty text included.
	ErrIdentifierBlank = errors.New("the identifier needs at least one character that is not whitespace")
	// ErrIdentifierHasControlCharacters is returned for a text that holds a Unicode control character (category Cc),
	// such as a tab or a newline.
	ErrIdentifierHasControlCharacters = errors.New("the identifier must not hold control characters")
	// ErrIdentifierNotUTF8 is returned for a text that is not valid UTF-8.
	ErrIdentifierNotUTF8 = errors.New("the identifier must be valid UTF-8")
)

// === HexDigits ===

var (
	// ErrHexDigitsInvalid is returned for a text that holds a character other than a lowercase hexadecimal digit.
	ErrHexDigitsInvalid = errors.New("the text must hold only lowercase hexadecimal digits")
	// ErrHexDigitsWrongWidth is returned for a text whose length is not the width that was asked for.
	ErrHexDigitsWrongWidth = errors.New("the text does not have the exact width of the hexadecimal digits")
)

// #### ERROR COMPOSITION

// JoinErrors joins the non-nil errors into one, so errors.Is finds each of them. It returns nil when there is none.
func JoinErrors(errs ...error) error {
	return errors.Join(errs...)
}
