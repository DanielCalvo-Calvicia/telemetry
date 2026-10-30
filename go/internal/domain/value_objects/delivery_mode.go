package value_objects

import (
	"slices"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// ===== DeliveryMode =========================================================

// DeliveryMode itself is generated (enums_generated.go) from contracts/enums/delivery_mode.json. This file adds the
// check that a text is one of its values; it never repeats the values.

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrDeliveryModeUnknown is domain.ErrDeliveryModeUnknown, so a caller of this package can use it without importing
	// domain.
	ErrDeliveryModeUnknown = domain.ErrDeliveryModeUnknown
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewDeliveryMode returns the DeliveryMode of the text, or ErrDeliveryModeUnknown when the text is not a value of the
// contract.
func NewDeliveryMode(text string) (DeliveryMode, error) {
	if validation_error := ValidateDeliveryMode(text); validation_error != nil {
		return "", validation_error
	}
	return DeliveryMode(text), nil
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateDeliveryMode returns nil when the text is exactly one of DeliveryModeValues(), and ErrDeliveryModeUnknown
// otherwise.
func ValidateDeliveryMode(text string) error {
	if !slices.Contains(DeliveryModeValues(), DeliveryMode(text)) {
		return ErrDeliveryModeUnknown
	}
	return nil
}
