package dtos

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// DIAGNOSTIC EVENT DTO
// =============================================================================

// DiagnosticEvent is one diagnostic: something went wrong in the delivery of telemetry. It is never a telemetry event:
// it does not go through the pipeline, the queues, the masker or a destination (REQ-DGN-003). It holds counters and
// identifiers only, never the message or the fields of an event, never a token. Code says what happened; Kind and Index
// say which destination (the kind and its position among the sinks of the kind); Reason is a short word such as the
// reason of a drop, cleaned of any character but A-Z a-z 0-9 _ . : - and cut to 64 characters; Count is the events it
// is about; Suppressed is the number of diagnostics that the rate limit suppressed before this one; AtMS is the wall
// clock in milliseconds since the Unix epoch.
type DiagnosticEvent struct {
	Code       value_objects.DiagnosticCode
	Kind       string
	Index      int
	Reason     string
	Count      int
	Suppressed int
	AtMS       int64
}
