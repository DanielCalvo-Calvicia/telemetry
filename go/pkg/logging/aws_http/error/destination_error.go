// Package error classifies the failures of the AWS HTTP adapter. The adapter does no I/O itself (the shared client of
// the core does), so the only failures that it owns are an unexpected one inside it and an event that cannot be turned
// into an add_log request. No text of a cause is ever part of a reason.
package error

import "errors"

// =============================================================================
// DESTINATION ERROR
// =============================================================================

const (
	// ReasonDestinationFailed is the reason of an event that a failure inside the adapter (a panic, a nil destination)
	// did not let it count.
	ReasonDestinationFailed = "destination_failed"
	// ReasonTranslationFailed is the reason of a log event that cannot be turned into an add_log request (a text that is
	// not valid UTF-8, an attribute with no JSON form, an occurred_at that is not a time). Retrying cannot fix it.
	ReasonTranslationFailed = "serialization_failed"
)

// The delivery error codes of the catalog arrive with T-05, so these errors have no catalog code yet.
var (
	// ErrDestinationUnusable is the sentinel of a destination that cannot work: it is nil or it failed inside.
	ErrDestinationUnusable = errors.New("the AWS HTTP destination is not usable")
	// ErrEventNotTranslatable is the sentinel of an event that cannot become an add_log request.
	ErrEventNotTranslatable = errors.New("the event cannot be translated to an add_log request")
)

// MapDestinationError classifies a failure inside the destination (a nil destination or a recovered panic): it joins
// ErrDestinationUnusable and the cause, which may be nil.
func MapDestinationError(cause error) error {
	return errors.Join(ErrDestinationUnusable, cause)
}

// MapTranslationError classifies the failure to translate one event: it joins ErrEventNotTranslatable and the cause,
// which may be nil.
func MapTranslationError(cause error) error {
	return errors.Join(ErrEventNotTranslatable, cause)
}
