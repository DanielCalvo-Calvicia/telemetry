// Package error classifies the failures of the file destination. The text of the cause (the file system may put a path
// in it) is kept for errors.Is and errors.As and is never part of a reason.
package error

import "errors"

// =============================================================================
// FILE ERROR
// =============================================================================

const (
	// ReasonWriteFailed is the reason of a line that the file did not take.
	ReasonWriteFailed = "write_failed"
	// ReasonRotationFailed is the reason of an event that was not written because the rotation of the files failed.
	ReasonRotationFailed = "rotation_failed"
	// ReasonSerializationFailed is the reason of an event that could not be encoded to a line.
	ReasonSerializationFailed = "serialization_failed"
	// ReasonDestinationFailed is the reason of an event that a failure inside the destination (a panic) did not let it
	// count.
	ReasonDestinationFailed = "destination_failed"
)

// The delivery error codes of the catalog arrive with T-05, so these errors have no catalog code yet.
var (
	// ErrWriteFailed is the sentinel of a file that could not be written.
	ErrWriteFailed = errors.New("the file could not be written")
	// ErrRotationFailed is the sentinel of files that could not be rotated.
	ErrRotationFailed = errors.New("the files could not be rotated")
	// ErrSerializationFailed is the sentinel of an event that could not be encoded to a line.
	ErrSerializationFailed = errors.New("the event could not be encoded for the file")
	// ErrDestinationUnusable is the sentinel of a destination that cannot work: it is nil or it failed inside.
	ErrDestinationUnusable = errors.New("the file destination is not usable")
)

// MapWriteError classifies a failure to write the file: it joins ErrWriteFailed and the cause, so errors.Is finds both.
func MapWriteError(cause error) error {
	return errors.Join(ErrWriteFailed, cause)
}

// MapRotationError classifies a failure to rotate the files: it joins ErrRotationFailed and the cause.
func MapRotationError(cause error) error {
	return errors.Join(ErrRotationFailed, cause)
}

// MapSerializationError classifies a failure to encode an event: it joins ErrSerializationFailed and the cause.
func MapSerializationError(cause error) error {
	return errors.Join(ErrSerializationFailed, cause)
}

// MapDestinationError classifies a failure inside the destination (a nil destination or a recovered panic): it joins
// ErrDestinationUnusable and the cause, which may be nil.
func MapDestinationError(cause error) error {
	return errors.Join(ErrDestinationUnusable, cause)
}
