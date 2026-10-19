package http_sink

import (
	"context"
	"time"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/http_sink/error"
	external "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/http_sink/external"
	helpers "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/http_sink/helpers"
)

// =============================================================================
// HTTP LOGGER
// =============================================================================

// RequestTimeout is how long a send to the log receiver may take before it is abandoned (Q18-15). There is
// no retry: a failed entry is dropped and the logger service reports the failure.
const RequestTimeout = 3 * time.Second

// HTTPLogger is a log sink that posts every entry as JSON to a generic log receiver. The HTTP calls
// live in external/; this file only holds the client and the endpoint and implements the port.
type HTTPLogger struct {
	client   *external.Client
	endpoint string
}

var _ ports_outbound.LogSinkPort = (*HTTPLogger)(nil)

// NewHTTPLogger creates an HTTPLogger that posts to host:port/path with the standard RequestTimeout. The host
// includes the scheme, for example "http://localhost" (LOGGER_HOST); the port and the path (LOGGER_PORT,
// LOGGER_PATH) are required. It returns application.ErrLogSinkEndpointMissing when any of them is empty.
func NewHTTPLogger(host string, port string, path string) (*HTTPLogger, error) {
	return NewHTTPLoggerWithTimeout(host, port, path, RequestTimeout)
}

// NewHTTPLoggerWithTimeout is NewHTTPLogger with a custom timeout; tests use it to avoid waiting 3 seconds.
func NewHTTPLoggerWithTimeout(host string, port string, path string, timeout time.Duration) (*HTTPLogger, error) {
	if host == "" || port == "" || path == "" {
		return nil, application.ErrLogSinkEndpointMissing
	}

	return &HTTPLogger{
		client:   external.NewClient(timeout),
		endpoint: helpers.BuildEndpoint(host, port, path),
	}, nil
}

// Send encodes the entry as JSON and posts it to the log receiver; any failure (encoding, network, timeout or a
// status of 400 and above) is translated to application.ErrLogSinkDeliveryFailed and keeps its root cause.
func (http_logger *HTTPLogger) Send(ctx context.Context, log_entry dtos.LogEntryOutboundDTO) error {
	encoded_entry, err := helpers.EncodeLogEntry(log_entry)
	if err != nil {
		return adapter_error.MapPostError(err)
	}

	if err := http_logger.client.PostEntry(ctx, http_logger.endpoint, encoded_entry); err != nil {
		return adapter_error.MapPostError(err)
	}
	return nil
}
