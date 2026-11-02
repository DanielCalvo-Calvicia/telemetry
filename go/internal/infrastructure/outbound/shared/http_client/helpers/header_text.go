package helpers

// =============================================================================
// HEADER TEXT
// =============================================================================

// IsVisibleASCII tells whether every character of the text is visible ASCII (0x21 to 0x7E): no space and no control
// character, so the text cannot split a header (header injection). An empty text is true.
func IsVisibleASCII(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] < 0x21 || text[index] > 0x7E {
			return false
		}
	}
	return true
}

// IsHTTPToken tells whether the text is a valid HTTP token (RFC 7230), the shape of a header name. An empty text is
// false.
func IsHTTPToken(text string) bool {
	if text == "" {
		return false
	}
	for index := 0; index < len(text); index++ {
		character := text[index]
		switch {
		case character >= '0' && character <= '9', character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z':
		case character == '!', character == '#', character == '$', character == '%', character == '&', character == '\'',
			character == '*', character == '+', character == '-', character == '.', character == '^', character == '_',
			character == '`', character == '|', character == '~':
		default:
			return false
		}
	}
	return true
}
