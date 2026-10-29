package value_objects

import (
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// ===== UTCInstant ===========================================================

// UTCInstant is a moment in UTC, kept with microsecond precision, in the years 0001 to 9999. It refuses another zone
// instead of converting it: a time that is not in UTC is a mistake of the caller (ADR-012). The time zone of a
// resource is for display and is never used here.
type UTCInstant struct {
	instant time.Time
}

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrNonUTCTime is domain.ErrNonUTCTime, so a caller of this package can use it without importing domain.
	ErrNonUTCTime = domain.ErrNonUTCTime
	// ErrUTCInstantTextInvalid is domain.ErrUTCInstantTextInvalid.
	ErrUTCInstantTextInvalid = domain.ErrUTCInstantTextInvalid
)

// ----- CONSTANTS ------------------------------------------------------------

const (
	utc_instant_layout       = domain.UTCInstantLayout
	utc_instant_minimum_year = domain.UTCInstantMinimumYear
	utc_instant_maximum_year = domain.UTCInstantMaximumYear
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewUTCInstant returns the UTCInstant of the time. The time must have an offset of zero and a year from 1 to 9999,
// otherwise it returns ErrNonUTCTime. The nanoseconds below the microsecond are cut, not rounded, and any monotonic
// clock reading is dropped.
func NewUTCInstant(instant time.Time) (UTCInstant, error) {
	if validation_error := ValidateUTCInstant(instant); validation_error != nil {
		return UTCInstant{}, validation_error
	}
	return UTCInstant{instant: instant.UTC().Truncate(time.Microsecond)}, nil
}

// ParseUTCInstant returns the UTCInstant of a text in exactly the format of String: YYYY-MM-DDTHH:MM:SS.ffffffZ, six
// decimals and the letter Z. Any other text, an offset instead of the Z and a text with another number of decimals
// included, returns ErrUTCInstantTextInvalid. The text is never part of the error.
func ParseUTCInstant(text string) (UTCInstant, error) {
	parsed_time, parse_error := time.Parse(utc_instant_layout, text)
	if parse_error != nil {
		return UTCInstant{}, ErrUTCInstantTextInvalid
	}
	utc_instant, instant_error := NewUTCInstant(parsed_time)
	if instant_error != nil || utc_instant.String() != text {
		return UTCInstant{}, ErrUTCInstantTextInvalid
	}
	return utc_instant, nil
}

// ----- METHODS --------------------------------------------------------------

// Time returns the instant as a time.Time in UTC, with microsecond precision.
func (utc_instant UTCInstant) Time() time.Time {
	return utc_instant.instant
}

// String returns the instant as YYYY-MM-DDTHH:MM:SS.ffffffZ: six decimals with the trailing zeros kept, and the Z.
func (utc_instant UTCInstant) String() string {
	return utc_instant.instant.Format(utc_instant_layout)
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateUTCInstant returns nil when the time has an offset of zero and a year from 1 to 9999, and ErrNonUTCTime
// otherwise.
func ValidateUTCInstant(instant time.Time) error {
	if _, offset_seconds := instant.Zone(); offset_seconds != 0 {
		return ErrNonUTCTime
	}
	if year := instant.UTC().Year(); year < utc_instant_minimum_year || year > utc_instant_maximum_year {
		return ErrNonUTCTime
	}
	return nil
}
