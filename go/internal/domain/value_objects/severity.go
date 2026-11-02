package value_objects

import (
	"slices"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// ===== Severity =============================================================

// Severity itself is generated (enums_generated.go) from contracts/enums/severity.json. This file adds the check that a
// text is one of its values; it never repeats the values.

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrSeverityInvalid is domain.ErrSeverityInvalid, so a caller of this package can use it without importing domain.
	ErrSeverityInvalid = domain.ErrSeverityInvalid
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewSeverity returns the Severity of the text, or ErrSeverityInvalid when the text is not a value of the contract.
func NewSeverity(text string) (Severity, error) {
	if validation_error := ValidateSeverity(text); validation_error != nil {
		return "", validation_error
	}
	return Severity(text), nil
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateSeverity returns nil when the text is exactly one of SeverityValues(), and ErrSeverityInvalid otherwise.
func ValidateSeverity(text string) error {
	if !slices.Contains(SeverityValues(), Severity(text)) {
		return ErrSeverityInvalid
	}
	return nil
}

// ----- ORDER ----------------------------------------------------------------

// SeverityRank returns the position of the severity in the order of SeverityValues() (TRACE 0 up to CRITICAL 5), so a
// destination can compare two severities. A text that is not a severity ranks -1. The order is the one of the
// contract; this function never repeats the values.
func SeverityRank(severity Severity) int {
	return slices.Index(SeverityValues(), severity)
}

// SeverityAtLeast tells whether the severity is the minimum or above it. It is false when either is not a severity.
func SeverityAtLeast(severity Severity, minimum Severity) bool {
	severity_rank := SeverityRank(severity)
	minimum_rank := SeverityRank(minimum)
	return severity_rank >= 0 && minimum_rank >= 0 && severity_rank >= minimum_rank
}
