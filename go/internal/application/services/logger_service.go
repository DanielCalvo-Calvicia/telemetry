package services

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/inbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// LOGGER SERVICE
// =============================================================================

// EventEmitter is what the logger needs from the core: to turn one input into one event and deliver it. The
// *EventPipelineService satisfies it.
type EventEmitter interface {
	// Emit builds, masks, limits and delivers the event of the input and answers what happened.
	Emit(ctx context.Context, input dtos.NewEventInput) dtos.EmitResult
}

// LoggerService is the logger of the core: each method builds one log.entry.recorded event and hands it to the
// emitter. It holds no state that changes, so one instance serves any number of goroutines. It satisfies
// inbound.LoggerPort.
type LoggerService struct {
	emitter EventEmitter
}

var _ inbound.LoggerPort = (*LoggerService)(nil)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewLoggerService returns a LoggerService that hands its events to the emitter.
func NewLoggerService(emitter EventEmitter) *LoggerService {
	return &LoggerService{emitter: emitter}
}

// ----- METHODS --------------------------------------------------------------

// Trace records the entry at the severity TRACE. See log for what a call does.
func (service *LoggerService) Trace(ctx context.Context, entry dtos.LogEntryInput) {
	service.log(ctx, value_objects.SeverityTrace, entry)
}

// Debug records the entry at the severity DEBUG.
func (service *LoggerService) Debug(ctx context.Context, entry dtos.LogEntryInput) {
	service.log(ctx, value_objects.SeverityDebug, entry)
}

// Info records the entry at the severity INFO.
func (service *LoggerService) Info(ctx context.Context, entry dtos.LogEntryInput) {
	service.log(ctx, value_objects.SeverityInfo, entry)
}

// Warn records the entry at the severity WARN.
func (service *LoggerService) Warn(ctx context.Context, entry dtos.LogEntryInput) {
	service.log(ctx, value_objects.SeverityWarn, entry)
}

// Error records the entry at the severity ERROR.
func (service *LoggerService) Error(ctx context.Context, entry dtos.LogEntryInput) {
	service.log(ctx, value_objects.SeverityError, entry)
}

// Critical records the entry at the severity CRITICAL. It does not stop the process.
func (service *LoggerService) Critical(ctx context.Context, entry dtos.LogEntryInput) {
	service.log(ctx, value_objects.SeverityCritical, entry)
}

// ----- HELPERS --------------------------------------------------------------

// log builds the log.entry.recorded input of the entry and emits it. The data has the category and the message, and
// the function only when it is not empty. The fields become the attributes (LogFieldsToAttributes). The call context
// of ctx is attached by the pipeline, so it is not done here. No body or payload is added: only the fields of the
// caller (REQ-LOG-006). It never panics, never returns anything and does nothing on a nil service or a nil emitter; the
// result of the emitter is not used here, because a rejected event is counted in the diagnostics (T-05.5).
func (service *LoggerService) log(ctx context.Context, severity value_objects.Severity, entry dtos.LogEntryInput) {
	if service == nil || service.emitter == nil {
		return
	}
	defer func() {
		_ = recover()
	}()

	data := map[string]any{"category": entry.Category, "message": entry.Message}
	if entry.Function != "" {
		data["function"] = entry.Function
	}
	service.emitter.Emit(ctx, dtos.NewEventInput{
		EventType:  value_objects.EventTypeLogEntryRecorded,
		Severity:   severity,
		Data:       data,
		Attributes: LogFieldsToAttributes(entry.Fields),
	})
}
