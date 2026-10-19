package error

import (
	stderrors "errors"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
)

// joinErrors composes a root cause with its contextual sentinel without discarding either.
func joinErrors(errs ...error) error {
	return stderrors.Join(errs...)
}

// MapGenerateError translates a failure of the UUID library into application.ErrIDGenerationFailed,
// keeping the root cause.
func MapGenerateError(err error) error {
	return joinErrors(err, application.ErrIDGenerationFailed)
}
