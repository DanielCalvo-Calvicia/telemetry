package services

import (
	"context"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	call_depth "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/call_depth"
	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	ports_inbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/inbound"
	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	entities "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/entities"
	operations "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/operations"
	value_objects "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

// =============================================================================
// LOGGER SERVICE
// =============================================================================

// ----------------------------------------------------------------------------
// ERRORS
// ----------------------------------------------------------------------------
var (
	ErrLoggerEntryBuildFailed = application.ErrLoggerEntryBuildFailed
)

// LoggerService builds a log entry from what the caller reports, stamps it with the current time and sends
// it to every log sink. A sink that fails never fails the caller: the entry is dropped for that sink and the
// failure is reported to the fallback sink (Q18-15). Every entry is masked before it reaches a sink, so no
// sink ever receives sensitive data.
type LoggerService struct {
	log_sinks             []ports_outbound.LogSinkPort
	fallback_sink         ports_outbound.LogSinkPort
	clock                 ports_outbound.ClockPort
	minimum_level         value_objects.Level
	sensitive_data_masker operations.SensitiveDataMasker
}

var _ ports_inbound.LoggerPort = (*LoggerService)(nil) // compile-time interface assertion

// NewLoggerService creates a LoggerService that sends every entry at or above the minimum level to the
// given sinks; entries below it are dropped. The fallback sink (normally the console) receives a report of
// every sink failure; it may be nil when there is none. The masker hides sensitive data in every entry.
func NewLoggerService(
	log_sinks []ports_outbound.LogSinkPort,
	fallback_sink ports_outbound.LogSinkPort,
	clock ports_outbound.ClockPort,
	minimum_level value_objects.Level,
	sensitive_data_masker operations.SensitiveDataMasker,
) *LoggerService {
	return &LoggerService{
		log_sinks:             log_sinks,
		fallback_sink:         fallback_sink,
		clock:                 clock,
		minimum_level:         minimum_level,
		sensitive_data_masker: sensitive_data_masker,
	}
}

// Trace logs an entry at TRACE level.
func (logger_service *LoggerService) Trace(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error {
	return logger_service.log(ctx, value_objects.LevelTrace, log_entry)
}

// Debug logs an entry at DEBUG level.
func (logger_service *LoggerService) Debug(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error {
	return logger_service.log(ctx, value_objects.LevelDebug, log_entry)
}

// Info logs an entry at INFO level.
func (logger_service *LoggerService) Info(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error {
	return logger_service.log(ctx, value_objects.LevelInfo, log_entry)
}

// Warn logs an entry at WARN level.
func (logger_service *LoggerService) Warn(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error {
	return logger_service.log(ctx, value_objects.LevelWarn, log_entry)
}

// Error logs an entry at ERROR level.
func (logger_service *LoggerService) Error(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error {
	return logger_service.log(ctx, value_objects.LevelError, log_entry)
}

// Fatal logs an entry at FATAL level; it does not stop the process.
func (logger_service *LoggerService) Fatal(ctx context.Context, log_entry dtos.LogEntryInboundDTO) error {
	return logger_service.log(ctx, value_objects.LevelFatal, log_entry)
}

// log drops the entry when its level is below the minimum; otherwise it builds the entry and sends it to
// every sink. A failing sink does not stop the others and does not
// fail the caller: its failure is reported to the fallback sink. The only error returned is a failure to
// build the entry.
func (logger_service *LoggerService) log(ctx context.Context, level value_objects.Level, log_entry_dto dtos.LogEntryInboundDTO) error {
	if !level.IsAtLeast(logger_service.minimum_level) {
		return nil
	}

	log_entry_dto = fillFromCallContext(ctx, log_entry_dto)

	log_entry, err := entities.NewLogEntry(
		log_entry_dto.ProducerID,
		log_entry_dto.ApplicationID,
		log_entry_dto.ProcessID,
		log_entry_dto.Application,
		log_entry_dto.Category,
		log_entry_dto.Function,
		log_entry_dto.Indentation,
		log_entry_dto.Depth,
		level.String(),
		log_entry_dto.Message,
		log_entry_dto.Fields,
		logger_service.clock.Now(ctx),
	)
	if err != nil {
		return application.JoinErrors(ErrLoggerEntryBuildFailed, err)
	}

	log_entry_outbound := toLogEntryOutboundDTO(logger_service.sensitive_data_masker.MaskLogEntry(log_entry))
	for _, log_sink := range logger_service.log_sinks {
		if err := log_sink.Send(ctx, log_entry_outbound); err != nil {
			logger_service.reportSinkFailure(ctx, log_entry_outbound, err)
		}
	}
	return nil
}

// reportSinkFailure writes an ERROR entry to the fallback sink telling that an entry was dropped. It keeps the
// identifiers of the dropped entry so both can be correlated, but never repeats its message or fields, which
// may hold sensitive data. A failure of the fallback sink itself is ignored: there is nowhere left to report it.
func (logger_service *LoggerService) reportSinkFailure(ctx context.Context, dropped_entry dtos.LogEntryOutboundDTO, sink_error error) {
	if logger_service.fallback_sink == nil {
		return
	}

	_ = logger_service.fallback_sink.Send(ctx, dtos.LogEntryOutboundDTO{
		ProducerID:    dropped_entry.ProducerID,
		ApplicationID: dropped_entry.ApplicationID,
		ProcessID:     dropped_entry.ProcessID,
		Application:   dropped_entry.Application,
		Category:      "logger",
		Function:      "log",
		Indentation:   dropped_entry.Indentation,
		Depth:         dropped_entry.Depth,
		Level:         value_objects.LevelError.String(),
		Message:       "log sink failed: the entry was dropped",
		Fields: map[string]any{
			"error":         logger_service.sensitive_data_masker.MaskText(sink_error.Error()),
			"dropped_level": dropped_entry.Level,
		},
		Timestamp: dropped_entry.Timestamp,
	})
}

// toLogEntryOutboundDTO flattens a log entry into the payload the sinks receive.
func toLogEntryOutboundDTO(log_entry entities.LogEntry) dtos.LogEntryOutboundDTO {
	return dtos.LogEntryOutboundDTO{
		ProducerID:    log_entry.Identifiers.ProducerID,
		ApplicationID: log_entry.Identifiers.ApplicationID,
		ProcessID:     log_entry.Identifiers.ProcessID,
		Application:   log_entry.Location.Application,
		Category:      log_entry.Location.Category,
		Function:      log_entry.Location.Function,
		Indentation:   log_entry.Indentation.Indentation,
		Depth:         log_entry.Indentation.Depth,
		Level:         log_entry.Level.String(),
		Message:       log_entry.Message.String(),
		Fields:        log_entry.Fields,
		Timestamp:     log_entry.Timestamp,
	}
}

// fillFromCallContext overwrites the identity and the indentation of the entry with the ones the request
// carries in its context (see the call_depth package), as the old logger did. A context without them is not an
// error (the old logger failed the request): the values the caller reported are kept.
func fillFromCallContext(ctx context.Context, log_entry_dto dtos.LogEntryInboundDTO) dtos.LogEntryInboundDTO {
	call_context, found := call_depth.FromContext(ctx)
	if !found {
		return log_entry_dto
	}

	log_entry_dto.Application = call_context.Application
	log_entry_dto.ApplicationID = call_context.ApplicationID
	log_entry_dto.ProcessID = call_context.ProcessID
	log_entry_dto.Indentation = call_context.Indentation
	return log_entry_dto
}
