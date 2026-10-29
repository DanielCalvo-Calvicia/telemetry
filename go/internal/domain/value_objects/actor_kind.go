package value_objects

import (
	"slices"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// ===== ActorKind ============================================================

// ActorKind itself is generated (enums_generated.go) from contracts/enums/actor_kind.json. This file adds the check
// that a text is one of its values; it never repeats the values.

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrActorKindInvalid is domain.ErrActorKindInvalid, so a caller of this package can use it without importing domain.
	ErrActorKindInvalid = domain.ErrActorKindInvalid
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewActorKind returns the ActorKind of the text, or ErrActorKindInvalid when the text is not a value of the contract.
func NewActorKind(text string) (ActorKind, error) {
	if validation_error := ValidateActorKind(text); validation_error != nil {
		return "", validation_error
	}
	return ActorKind(text), nil
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateActorKind returns nil when the text is exactly one of ActorKindValues(), and ErrActorKindInvalid otherwise.
func ValidateActorKind(text string) error {
	if !slices.Contains(ActorKindValues(), ActorKind(text)) {
		return ErrActorKindInvalid
	}
	return nil
}
