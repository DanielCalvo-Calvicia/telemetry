package containers

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/composition_root/dependencies"
	logging_ports "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/inbound"
	logging_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	logging_services "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/services"
	logging_domain "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
	logging_operations "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/operations"
	logging_value_objects "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
	logging_config "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/config"
)

// =============================================================================
// LOGGING CONTAINER
// =============================================================================

// LoggingContainer is the assembled logger: the service behind LoggerPort and the shutdown of whatever needs flushing.
type LoggingContainer struct {
	Logger logging_ports.LoggerPort
	// shutdown flushes the sinks; the console and HTTP sinks hold nothing to flush yet.
	shutdown func(ctx context.Context) error
}

// NewLoggingContainer builds the console sink and the HTTP sink the settings switch on, the console as the fallback
// that receives the report of a sink that failed, the minimum level of app_env and the masker with the extra
// patterns of the settings.
func NewLoggingContainer(_ context.Context, app_env string, logging_settings logging_config.TelemetryConfig) (*LoggingContainer, error) {
	var log_sinks []logging_outbound.LogSinkPort
	console_sink := dependencies.NewConsoleLoggerDependency()
	if logging_settings.ConsoleEnabled {
		log_sinks = append(log_sinks, console_sink)
	}
	if logging_settings.HTTPEnabled {
		http_sink, err := dependencies.NewHTTPLoggerDependency(logging_settings)
		if err != nil {
			return nil, JoinErrors(err, ErrLoggerBuildFailed, ErrContainerBuildFailed)
		}
		log_sinks = append(log_sinks, http_sink)
	}

	minimum_level, err := logging_value_objects.NewLevel(logging_domain.MinimumLevelByEnvironment[app_env])
	if err != nil {
		return nil, JoinErrors(err, ErrLoggerBuildFailed, ErrContainerBuildFailed)
	}
	sensitive_data_masker, err := logging_operations.NewSensitiveDataMasker(logging_settings.MaskExtraPatterns)
	if err != nil {
		return nil, JoinErrors(err, ErrLoggerBuildFailed, ErrContainerBuildFailed)
	}

	logger := logging_services.NewLoggerService(log_sinks, console_sink, dependencies.NewSystemClockDependency(), minimum_level, sensitive_data_masker)
	return &LoggingContainer{Logger: logger, shutdown: func(context.Context) error { return nil }}, nil
}

// Shutdown flushes whatever the sinks buffer.
func (logging_container *LoggingContainer) Shutdown(ctx context.Context) error {
	return logging_container.shutdown(ctx)
}
