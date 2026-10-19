package dependencies

import (
	logging_config "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/config"
	http_sink "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/http_sink"
)

// NewHTTPLoggerDependency builds the HTTP sink of the logger from the host, the port and the path of the log receiver
// (LOGGER_HOST, LOGGER_PORT, LOGGER_PATH). It fails when one of them is empty.
func NewHTTPLoggerDependency(logging_settings logging_config.TelemetryConfig) (*http_sink.HTTPLogger, error) {
	return http_sink.NewHTTPLogger(logging_settings.Host, logging_settings.Port, logging_settings.Path)
}
