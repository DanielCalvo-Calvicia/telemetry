package application

import "errors"

// #############################################################################
// SERVICE ERRORS
// #############################################################################

// =============================================================================
// LoggerService
// =============================================================================
var (
	ErrLoggerEntryBuildFailed = errors.New("logger service: the log entry could not be built")
)

// =============================================================================
// LogSink
// =============================================================================
var (
	ErrLogSinkDeliveryFailed  = errors.New("log sink: the entry could not be delivered")
	ErrLogSinkEndpointMissing = errors.New("log sink: host, port and path of the log receiver are required")
)

// =============================================================================
// IDGenerator
// =============================================================================
var (
	ErrIDGenerationFailed = errors.New("id generator: a new identifier could not be generated")
)

// =============================================================================
// CallDepth
// =============================================================================
var (
	ErrCallDepthProcessIDFailed = errors.New("call depth: a process ID could not be generated for the request")
)

// #############################################################################
// ERROR COMPOSITION
// #############################################################################

// JoinErrors composes a root cause with its contextual sentinel without
// discarding either.
func JoinErrors(errs ...error) error {
	return errors.Join(errs...)
}
