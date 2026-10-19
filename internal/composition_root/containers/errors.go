package containers

import "errors"

// #############################################################################
// CONTAINER ERRORS
// #############################################################################

var (
	ErrContainerBuildFailed = errors.New("logging container: building the container failed")
	ErrLoggerBuildFailed    = errors.New("logging container: building the logger failed")
)

// #############################################################################
// ERROR COMPOSITION
// #############################################################################

// JoinErrors composes a root cause with its contextual sentinel without
// discarding either.
func JoinErrors(errs ...error) error {
	return errors.Join(errs...)
}
