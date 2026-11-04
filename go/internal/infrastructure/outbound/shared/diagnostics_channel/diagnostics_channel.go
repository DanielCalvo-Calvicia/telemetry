// Package diagnostics_channel is the adapter of the diagnostics channel port: a rate-limited, bounded path of its own that
// carries the diagnostics to the callbacks of the consumer and to a writer, apart from the telemetry pipeline.
package diagnostics_channel

import (
	"context"
	"io"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/diagnostics_rate"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/diagnostics_reason"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/diagnostics_channel/helpers"
)

// =============================================================================
// DIAGNOSTICS CHANNEL
// =============================================================================

// DiagnosticsChannel implements outbound.DiagnosticsChannelPort. Publish applies the rate limit (domain.DefaultDiagnosticsRateLimit
// diagnostics in each window of domain.DefaultDiagnosticsRateWindowMilliseconds, read on the monotonic clock) and queues the
// diagnostic in a bounded queue of domain.DefaultDiagnosticsQueueSize; one dispatcher goroutine, started by Start, takes
// them in order and calls every callback and then writes one line to the writer. A callback or a writer that panics is
// contained and counted. A callback that hangs blocks only the dispatcher: the queue then drops, and the caller of Publish
// never waits. Nothing starts in the constructor and the channel holds no global state.
type DiagnosticsChannel struct {
	clock       outbound.ClockPort
	callbacks   []func(event dtos.DiagnosticEvent)
	writer      io.Writer
	events      chan dtos.DiagnosticEvent
	stop_signal chan struct{}

	mutex           sync.Mutex
	window_start_ms int64
	window_count    int
	window_suppress int
	started         bool
	stopped         bool
	counters        dtos.ChannelDiagnostics
}

var _ outbound.DiagnosticsChannelPort = (*DiagnosticsChannel)(nil)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewDiagnosticsChannel returns a DiagnosticsChannel that reads the time on the clock (a nil clock reads zero), calls the
// callbacks in order (a nil one is ignored) and writes a line to the writer (nil is none). It starts nothing.
func NewDiagnosticsChannel(clock outbound.ClockPort, callbacks []func(event dtos.DiagnosticEvent), writer io.Writer) *DiagnosticsChannel {
	kept_callbacks := make([]func(event dtos.DiagnosticEvent), 0, len(callbacks))
	for _, callback := range callbacks {
		if callback != nil {
			kept_callbacks = append(kept_callbacks, callback)
		}
	}
	return &DiagnosticsChannel{
		clock:           clock,
		callbacks:       kept_callbacks,
		writer:          writer,
		events:          make(chan dtos.DiagnosticEvent, domain.DefaultDiagnosticsQueueSize),
		stop_signal:     make(chan struct{}),
		window_start_ms: diagnostics_rate.NoWindow,
	}
}

// ----- METHODS --------------------------------------------------------------

// Publish applies the rate limit and queues the diagnostic for the callbacks and the writer. A diagnostic that the limit
// refuses counts as suppressed; an allowed one carries the number suppressed before it and, when the queue is full,
// counts as dropped. The reason is cleaned (diagnostics_reason.SanitizeReason). It never waits, never fails and does nothing after
// Stop or on a nil channel. A channel with no callback and no writer only counts.
func (diagnostics_channel *DiagnosticsChannel) Publish(event dtos.DiagnosticEvent) {
	if diagnostics_channel == nil {
		return
	}
	now_ms := diagnostics_channel.now_ms()
	diagnostics_channel.mutex.Lock()
	defer diagnostics_channel.mutex.Unlock()
	if diagnostics_channel.stopped {
		return
	}
	plan := diagnostics_rate.PlanDiagnostic(now_ms, diagnostics_channel.window_start_ms, diagnostics_channel.window_count,
		diagnostics_channel.window_suppress, domain.DefaultDiagnosticsRateLimit, domain.DefaultDiagnosticsRateWindowMilliseconds)
	diagnostics_channel.window_start_ms = plan.WindowStartMS
	diagnostics_channel.window_count = plan.Count
	diagnostics_channel.window_suppress = plan.Suppressed
	if !plan.Allow {
		diagnostics_channel.counters.Suppressed++
		return
	}
	if len(diagnostics_channel.callbacks) == 0 && diagnostics_channel.writer == nil {
		diagnostics_channel.counters.Published++
		return
	}
	event.Suppressed = plan.AttachSuppressed
	event.Reason = diagnostics_reason.SanitizeReason(event.Reason)
	select {
	case diagnostics_channel.events <- event:
		diagnostics_channel.counters.Published++
	default:
		diagnostics_channel.counters.Dropped++
	}
}

// Start starts the dispatcher goroutine. It is idempotent, it does nothing after Stop, and it starts no goroutine when
// the channel has no callback and no writer.
func (diagnostics_channel *DiagnosticsChannel) Start() {
	if diagnostics_channel == nil {
		return
	}
	diagnostics_channel.mutex.Lock()
	defer diagnostics_channel.mutex.Unlock()
	if diagnostics_channel.started || diagnostics_channel.stopped {
		return
	}
	diagnostics_channel.started = true
	if len(diagnostics_channel.callbacks) == 0 && diagnostics_channel.writer == nil {
		return
	}
	go diagnostics_channel.dispatch()
}

// Stop makes Publish do nothing and tells the dispatcher to end. It does not wait for it: a callback that hangs is
// abandoned and its goroutine ends when the callback returns. The diagnostics that were already queued are delivered
// before the dispatcher ends. It is idempotent.
func (diagnostics_channel *DiagnosticsChannel) Stop() {
	if diagnostics_channel == nil {
		return
	}
	diagnostics_channel.mutex.Lock()
	defer diagnostics_channel.mutex.Unlock()
	if diagnostics_channel.stopped {
		return
	}
	diagnostics_channel.stopped = true
	close(diagnostics_channel.stop_signal)
}

// Counters returns the counters of the channel.
func (diagnostics_channel *DiagnosticsChannel) Counters() dtos.ChannelDiagnostics {
	if diagnostics_channel == nil {
		return dtos.ChannelDiagnostics{}
	}
	diagnostics_channel.mutex.Lock()
	defer diagnostics_channel.mutex.Unlock()
	return diagnostics_channel.counters
}

// ----- THE DISPATCHER -------------------------------------------------------

// dispatch is the loop of the dispatcher: it delivers the queued diagnostics in order until Stop, and then delivers what
// is still queued and returns. It holds no lock while it calls a callback or the writer.
func (diagnostics_channel *DiagnosticsChannel) dispatch() {
	for {
		select {
		case event := <-diagnostics_channel.events:
			diagnostics_channel.deliver(event)
		case <-diagnostics_channel.stop_signal:
			for {
				select {
				case event := <-diagnostics_channel.events:
					diagnostics_channel.deliver(event)
				default:
					return
				}
			}
		}
	}
}

// deliver calls every callback in order and then writes the line. A panic or an error of one of them is contained and
// counted; its value is never kept, because it could hold data.
func (diagnostics_channel *DiagnosticsChannel) deliver(event dtos.DiagnosticEvent) {
	for _, callback := range diagnostics_channel.callbacks {
		diagnostics_channel.contain(func() error {
			callback(event)
			return nil
		})
	}
	if diagnostics_channel.writer != nil {
		line := helpers.FormatLine(event)
		diagnostics_channel.contain(func() error {
			_, write_error := io.WriteString(diagnostics_channel.writer, line)
			return write_error
		})
	}
}

// contain runs the call and counts it as a callback error when it panics or returns an error.
func (diagnostics_channel *DiagnosticsChannel) contain(call func() error) {
	failed := false
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				failed = true
			}
		}()
		failed = call() != nil
	}()
	if failed {
		diagnostics_channel.mutex.Lock()
		diagnostics_channel.counters.CallbackErrors++
		diagnostics_channel.mutex.Unlock()
	}
}

// now_ms reads the monotonic clock in milliseconds.
func (diagnostics_channel *DiagnosticsChannel) now_ms() int64 {
	if diagnostics_channel.clock == nil {
		return 0
	}
	return diagnostics_channel.clock.MonotonicNanoseconds(context.Background()) / 1_000_000
}
