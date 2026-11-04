package outbound

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"

// =============================================================================
// DIAGNOSTICS CHANNEL PORT
// =============================================================================

// DiagnosticsChannelPort defines the channel on which the library tells the consumer that something went wrong in the
// delivery of telemetry (REQ-DGN-003). It is a path of its own: a diagnostic never goes through the pipeline, the queues,
// the masker or a destination, so a failure of telemetry can never create an event that can fail again. It is safe for
// concurrent use, and it never fails the caller.
type DiagnosticsChannelPort interface {
	// Publish offers one diagnostic to the channel. It never blocks, never fails and never panics; a diagnostic that the
	// rate limit refuses, or that finds the queue full, or that comes after Stop, is counted or ignored.
	Publish(event dtos.DiagnosticEvent)

	// Start starts what delivers the diagnostics to the callbacks and to the writer. It is idempotent, and a channel with
	// no callback and no writer starts nothing.
	Start()

	// Stop stops the delivery. It does not wait for a callback that hangs, and after it Publish does nothing. It is
	// idempotent.
	Stop()

	// Counters returns the counters of the channel.
	Counters() dtos.ChannelDiagnostics
}
