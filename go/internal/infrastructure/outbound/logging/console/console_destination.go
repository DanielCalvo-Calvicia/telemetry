// Package console is the adapter that writes events to the standard output as JSON lines or as readable text, with a
// minimum severity (REQ-LOG-001, REQ-LOG-003). It is the reference tree of the adapters of REQ-ARC-014: the I/O lives in
// external/, the error translation in error/ and the encoders in helpers/.
package console

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console/error"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console/external"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console/helpers"
)

// =============================================================================
// CONSOLE DESTINATION
// =============================================================================

// ConsoleDestination implements outbound.LogSinkPort and outbound.EventSinkPort: it writes each event as one line to a
// writer, normally the standard output. A mutex makes the lines of concurrent callers never interleave. It never returns
// an error and never panics to the caller of Send (ADR-005); WriteEvent returns the classified error of a write to the
// caller of the adapter, never to a log call.
type ConsoleDestination struct {
	writer           io.Writer
	encode           func(event value_objects.Envelope) ([]byte, error)
	minimum_severity value_objects.Severity
	write_mutex      sync.Mutex
}

var (
	_ outbound.LogSinkPort   = (*ConsoleDestination)(nil)
	_ outbound.EventSinkPort = (*ConsoleDestination)(nil)
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewConsoleDestination validates the settings and returns a destination that writes to the writer. An invalid setting
// returns the *config.ValidationError of ValidateSettings as it is.
func NewConsoleDestination(settings Settings, writer io.Writer) (*ConsoleDestination, error) {
	if validation_error := ValidateSettings(settings); validation_error != nil {
		return nil, validation_error
	}
	encode := helpers.EncodeJSONLine
	if settings.Format == domain.ConsoleFormatText {
		encode = helpers.EncodeTextLine
	}
	return &ConsoleDestination{
		writer:           writer,
		encode:           encode,
		minimum_severity: value_objects.Severity(settings.MinimumSeverity),
	}, nil
}

// NewStandardOutputDestination validates the settings and returns a destination that writes to the standard output of
// the process.
func NewStandardOutputDestination(settings Settings) (*ConsoleDestination, error) {
	return NewConsoleDestination(settings, external.StandardOutput())
}

// ----- SEND -----------------------------------------------------------------

// Send writes the events, one line each, and counts each event once. An event below the minimum severity is not
// written and counts as accepted: the destination took it and its policy decided not to write it. An event that cannot
// be encoded counts as failed with the reason serialization_failed; a line that the writer does not take counts as
// failed with the reason write_failed, and the next events are still tried. The reason of the result is the first one.
// A panic inside the destination or in the writer is recovered: the events not counted yet are failed with the reason
// destination_failed. A nil destination counts every event as failed with that reason.
func (destination *ConsoleDestination) Send(_ context.Context, events []value_objects.Envelope) (result dtos.SendResult) {
	if destination == nil {
		return dtos.SendResult{Failed: len(events), Reason: adapter_error.ReasonDestinationFailed}
	}
	destination.write_mutex.Lock()
	defer destination.write_mutex.Unlock()
	defer func() {
		if recover() != nil {
			not_counted := len(events) - result.Accepted - result.Dropped - result.Failed
			if not_counted > 0 {
				result.Failed += not_counted
				if result.Reason == "" {
					result.Reason = adapter_error.ReasonDestinationFailed
				}
			}
		}
	}()

	for _, event := range events {
		reason, deliver_error := destination.deliver(event)
		if deliver_error != nil {
			result.Failed++
			if result.Reason == "" {
				result.Reason = reason
			}
			continue
		}
		result.Accepted++
	}
	return result
}

// WriteEvent writes one event, respecting the minimum severity: an event below it is not written and the result is nil.
// A failure comes back as a classified error: it is errors.Is adapter_error.ErrWriteFailed (the writer did not take the
// line), ErrSerializationFailed (the event has no line) or ErrDestinationUnusable (a nil destination or a panic), and
// the cause is kept. The error goes to the caller of the adapter and never to a log call (ADR-005). The delivery error
// codes of the catalog arrive with T-05, so it has no catalog code yet.
func (destination *ConsoleDestination) WriteEvent(event value_objects.Envelope) (write_error error) {
	if destination == nil {
		return adapter_error.MapDestinationError(nil)
	}
	destination.write_mutex.Lock()
	defer destination.write_mutex.Unlock()
	defer func() {
		if recovered := recover(); recovered != nil {
			write_error = adapter_error.MapDestinationError(fmt.Errorf("panic: %v", recovered))
		}
	}()
	_, write_error = destination.deliver(event)
	return write_error
}

// ----- HELPERS --------------------------------------------------------------

// deliver writes one event when it reaches the minimum severity. It returns the reason and the classified error of a
// failure, and nil when the event was written or filtered. The caller holds the mutex.
func (destination *ConsoleDestination) deliver(event value_objects.Envelope) (string, error) {
	if !value_objects.SeverityAtLeast(event.Severity, destination.minimum_severity) {
		return "", nil
	}
	line, encode_error := destination.encode(event)
	if encode_error != nil {
		return adapter_error.ReasonSerializationFailed, adapter_error.MapSerializationError(encode_error)
	}
	if write_error := external.WriteLine(destination.writer, line); write_error != nil {
		return adapter_error.ReasonWriteFailed, adapter_error.MapWriteError(write_error)
	}
	return "", nil
}
