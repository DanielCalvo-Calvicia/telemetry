// Package error classifies the failures of the HTTP destination. The destination does no I/O itself (the shared client
// does), so the only failure that it owns is an unexpected one inside it. No text of a cause is ever part of a reason.
package error

import "errors"

// =============================================================================
// DESTINATION ERROR
// =============================================================================

// ReasonDestinationFailed is the reason of an event that a failure inside the destination (a panic, a nil destination)
// did not let it count.
const ReasonDestinationFailed = "destination_failed"

// The delivery error codes of the catalog arrive with T-05, so this error has no catalog code yet.
var (
	// ErrDestinationUnusable is the sentinel of a destination that cannot work: it is nil or it failed inside.
	ErrDestinationUnusable = errors.New("the HTTP destination is not usable")
)

// MapDestinationError classifies a failure inside the destination (a nil destination or a recovered panic): it joins
// ErrDestinationUnusable and the cause, which may be nil.
func MapDestinationError(cause error) error {
	return errors.Join(ErrDestinationUnusable, cause)
}
