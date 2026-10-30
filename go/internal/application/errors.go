package application

import "errors"

// #### EVENT BUILD ERRORS

// The sentinels of the application layer. The classified errors of the domain (event_type_unknown,
// event_schema_invalid) are returned as they are; these ones say that the event could not be built for a reason that
// is not the data of the caller.
var (
	// ErrEventBuildFailed is returned, joined with its cause, when an event could not be built: a port failed, a port
	// panicked or a value that the core made was not valid. It never carries the data of the event.
	ErrEventBuildFailed = errors.New("the event could not be built")
	// ErrEventBuildPanicked is the cause that is joined with ErrEventBuildFailed when a panic was recovered while the
	// event was built. The panic value is not kept, because it could hold data of the event.
	ErrEventBuildPanicked = errors.New("a panic was recovered while the event was built")
	// ErrObservedAtInvalid is the detail of an event_schema_invalid error: data.observed_at is present but is not a
	// string in the time format of the contract (YYYY-MM-DDTHH:MM:SS.ffffffZ).
	ErrObservedAtInvalid = errors.New("data.observed_at must be a string in the format YYYY-MM-DDTHH:MM:SS.ffffffZ")
)

// #### MASKING ERRORS

var (
	// ErrEnvelopeMaskingFailed is returned, with an empty envelope, when an envelope could not be masked: a panic was
	// recovered while it was masked. The caller drops the event; it never receives the unmasked envelope (ADR-005,
	// ADR-006). The panic value is not kept, because it could hold data of the event.
	ErrEnvelopeMaskingFailed = errors.New("the envelope could not be masked")
)

// #### LIMITS ERRORS

var (
	// ErrEnvelopeLimitsFailed is returned, with an empty envelope, when the limits could not be applied to an envelope:
	// a panic was recovered while they were applied. The caller drops the event (ADR-005). The panic value is not kept,
	// because it could hold data of the event.
	ErrEnvelopeLimitsFailed = errors.New("the limits could not be applied to the envelope")
)

// #### ERROR COMPOSITION

// JoinErrors joins the non-nil errors into one, so errors.Is finds each of them. It returns nil when there is none.
func JoinErrors(errs ...error) error {
	return errors.Join(errs...)
}
