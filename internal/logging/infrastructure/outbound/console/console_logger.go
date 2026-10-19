package console

import (
	"context"
	"io"

	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/console/error"
	external "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/console/external"
	helpers "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/console/helpers"
)

// =============================================================================
// CONSOLE LOGGER
// =============================================================================

// ConsoleLogger is a log sink that prints every entry as one JSON line on the standard output. Writing to
// the output is done in external/; this file only holds the writer and implements the port.
type ConsoleLogger struct {
	writer io.Writer
}

var _ ports_outbound.LogSinkPort = (*ConsoleLogger)(nil)

// NewConsoleLogger creates a ConsoleLogger that writes to the standard output.
func NewConsoleLogger() *ConsoleLogger {
	return NewConsoleLoggerWithWriter(external.StandardOutput())
}

// NewConsoleLoggerWithWriter creates a ConsoleLogger that writes to the given writer; tests use it to capture the output.
func NewConsoleLoggerWithWriter(writer io.Writer) *ConsoleLogger {
	return &ConsoleLogger{writer: writer}
}

// Send encodes the entry as JSON and writes it as one line; any failure is translated to
// application.ErrLogSinkDeliveryFailed and keeps its root cause.
func (console_logger *ConsoleLogger) Send(_ context.Context, log_entry dtos.LogEntryOutboundDTO) error {
	encoded_entry, err := helpers.EncodeLogEntry(log_entry)
	if err != nil {
		return adapter_error.MapWriteError(err)
	}

	if err := external.WriteLine(console_logger.writer, encoded_entry); err != nil {
		return adapter_error.MapWriteError(err)
	}
	return nil
}
