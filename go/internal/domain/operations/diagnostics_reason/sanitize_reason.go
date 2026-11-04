// Package diagnostics_reason holds the pure rule that cleans the reason of a diagnostic, so that it can carry no message,
// no value and no address (REQ-DGN-003, REQ-SEC-007).
package diagnostics_reason

import "strings"

// =============================================================================
// SANITIZE REASON
// =============================================================================

// MaxReasonChars is the most characters that the reason of a diagnostic keeps.
const MaxReasonChars = 64

// UnspecifiedReason is the reason of a diagnostic whose reason is empty.
const UnspecifiedReason = "unspecified"

// SanitizeReason returns the reason in a form that can carry no message, no value and no address: only the characters
// A-Z a-z 0-9 _ . : - are kept, any other is replaced by _, it is cut to MaxReasonChars characters and an empty reason
// is UnspecifiedReason.
func SanitizeReason(reason string) string {
	if reason == "" {
		return UnspecifiedReason
	}
	var cleaned strings.Builder
	kept := 0
	for _, character := range reason {
		if kept == MaxReasonChars {
			break
		}
		if is_allowed_character(character) {
			cleaned.WriteRune(character)
		} else {
			cleaned.WriteByte('_')
		}
		kept++
	}
	return cleaned.String()
}

// is_allowed_character says whether the character may stay in the reason of a diagnostic.
func is_allowed_character(character rune) bool {
	switch {
	case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9':
		return true
	}
	return character == '_' || character == '.' || character == ':' || character == '-'
}
