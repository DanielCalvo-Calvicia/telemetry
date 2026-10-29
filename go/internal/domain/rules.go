package domain

// #### VALUE OBJECT RULES

// The raw values of the rules of the value objects. They are untyped; the value object that uses one wraps it.

// === UTCInstant ===

const (
	// UTCInstantLayout is the Go layout of the text of an instant: UTC, exactly six decimals (trailing zeros kept,
	// the nanoseconds are cut) and the letter Z (ADR-012).
	UTCInstantLayout = "2006-01-02T15:04:05.000000Z"
	// UTCInstantMinimumYear is the first year that an instant may have.
	UTCInstantMinimumYear = 1
	// UTCInstantMaximumYear is the last year that an instant may have.
	UTCInstantMaximumYear = 9999
)

// === EventID ===

const (
	// EventIDGroupSeparator separates the five groups of an event id.
	EventIDGroupSeparator = "-"
	// EventIDVersionDigit is the first digit of the third group of a UUID of version 7.
	EventIDVersionDigit = "7"
	// EventIDVariantDigits are the digits that the fourth group of an event id may start with (variant of RFC 4122).
	EventIDVariantDigits = "89ab"
)

// === ResourceIdentity ===

const (
	// TimeZonePattern is the shape of the time_zone of a resource: names made of letters, digits, underscore, plus and
	// minus, joined by slashes. It is a shape, not a check against the IANA database, because the field is for display.
	TimeZonePattern = `^[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+)*$`
)

// === CorrelationContext ===

const (
	// TraceIDWidth is the number of hexadecimal digits of a W3C trace id.
	TraceIDWidth = 32
	// SpanIDWidth is the number of hexadecimal digits of a W3C span id.
	SpanIDWidth = 16
)
