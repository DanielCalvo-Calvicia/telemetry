package error

import (
	stderrors "errors"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
)

// joinErrors composes a root cause with its contextual sentinel without discarding either.
func joinErrors(errs ...error) error {
	return stderrors.Join(errs...)
}

// MapWriteError translates a failure while encoding or writing an entry into
// application.ErrLogSinkDeliveryFailed, keeping the root cause.
func MapWriteError(err error) error {
	return joinErrors(err, application.ErrLogSinkDeliveryFailed)
}
