package value_objects

import (
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
)

// ===== EventID ==============================================================

// EventID is the id of an event: a UUID of version 7 in lowercase, hyphenated 8-4-4-4-12, with the RFC 4122 variant
// (REQ-CTR-009). It validates an id that exists; it never generates one (generation is a port of the application).
type EventID struct {
	text string
}

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrEventIDInvalid is domain.ErrEventIDInvalid, so a caller of this package can use it without importing domain.
	ErrEventIDInvalid = domain.ErrEventIDInvalid
)

// ----- CONSTANTS ------------------------------------------------------------

const (
	event_id_group_separator = domain.EventIDGroupSeparator
	event_id_version_digit   = domain.EventIDVersionDigit
	event_id_variant_digits  = domain.EventIDVariantDigits
)

// event_id_group_widths are the number of hexadecimal digits of each of the five groups of an event id.
var event_id_group_widths = [...]int{8, 4, 4, 4, 12}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewEventID returns the EventID of the text, or ErrEventIDInvalid when the text is not a UUID of version 7.
func NewEventID(text string) (EventID, error) {
	if validation_error := ValidateEventID(text); validation_error != nil {
		return EventID{}, validation_error
	}
	return EventID{text: text}, nil
}

// ----- METHODS --------------------------------------------------------------

// String returns the id as it was given: lowercase and hyphenated.
func (event_id EventID) String() string {
	return event_id.text
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateEventID returns nil when the text is a lowercase hyphenated UUID of version 7 with the variant 8, 9, a or b,
// and ErrEventIDInvalid otherwise.
func ValidateEventID(text string) error {
	groups := strings.Split(text, event_id_group_separator)
	if len(groups) != len(event_id_group_widths) {
		return ErrEventIDInvalid
	}
	for position, group := range groups {
		if base_value_objects.ValidateHexDigits(group, event_id_group_widths[position]) != nil {
			return ErrEventIDInvalid
		}
	}
	if !strings.HasPrefix(groups[2], event_id_version_digit) {
		return ErrEventIDInvalid
	}
	if !strings.ContainsAny(groups[3][:1], event_id_variant_digits) {
		return ErrEventIDInvalid
	}
	return nil
}
