package aws_http

import (
	"context"
	"time"

	application "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application"
	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	ports_outbound "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
	adapter_error "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/aws_http/error"
	external "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/aws_http/external"
	helpers "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/aws_http/helpers"
)

// =============================================================================
// AWS HTTP LOGGER
// =============================================================================

// RequestTimeout is how long a send to the AWS log receiver may take before it is abandoned. There is no retry:
// a failed entry is dropped and the logger service reports the failure.
const RequestTimeout = 3 * time.Second

// AWSHTTPLogger is a log sink that adapts every entry to the contract of the add_log endpoint of aws_microservice
// (POST /api/v1/cloudwatch/logs/add) and posts it, so the entry ends up in a CloudWatch Logs stream. The HTTP calls
// live in external/ and the contract in helpers/; this file only holds the client, the endpoint and the target
// log group and log stream, and implements the port.
type AWSHTTPLogger struct {
	client          *external.Client
	endpoint        string
	log_group_name  string
	log_stream_name string
}

var _ ports_outbound.LogSinkPort = (*AWSHTTPLogger)(nil)

// NewAWSHTTPLogger creates an AWSHTTPLogger that posts to host:port/path with the standard RequestTimeout. The host
// includes the scheme, for example "http://aws-microservice". It returns application.ErrLogSinkEndpointMissing
// when the host, the port or the path is empty and application.ErrLogSinkAWSTargetMissing when the log group or the
// log stream is empty.
func NewAWSHTTPLogger(host string, port string, path string, log_group_name string, log_stream_name string) (*AWSHTTPLogger, error) {
	return NewAWSHTTPLoggerWithTimeout(host, port, path, log_group_name, log_stream_name, RequestTimeout)
}

// NewAWSHTTPLoggerWithTimeout is NewAWSHTTPLogger with a custom timeout; tests use it to avoid waiting 3 seconds.
func NewAWSHTTPLoggerWithTimeout(host string, port string, path string, log_group_name string, log_stream_name string, timeout time.Duration) (*AWSHTTPLogger, error) {
	if host == "" || port == "" || path == "" {
		return nil, application.ErrLogSinkEndpointMissing
	}
	if log_group_name == "" || log_stream_name == "" {
		return nil, application.ErrLogSinkAWSTargetMissing
	}

	return &AWSHTTPLogger{
		client:          external.NewClient(timeout),
		endpoint:        helpers.BuildEndpoint(host, port, path),
		log_group_name:  log_group_name,
		log_stream_name: log_stream_name,
	}, nil
}

// Send adapts the entry to the add_log request and posts it; any failure (encoding, network, timeout or a status of
// 400 and above) is translated to application.ErrLogSinkDeliveryFailed and keeps its root cause.
func (aws_http_logger *AWSHTTPLogger) Send(ctx context.Context, log_entry dtos.LogEntryOutboundDTO) error {
	encoded_request, err := helpers.BuildAddLogRequest(log_entry, aws_http_logger.log_group_name, aws_http_logger.log_stream_name)
	if err != nil {
		return adapter_error.MapPostError(err)
	}

	if err := aws_http_logger.client.PostEntry(ctx, aws_http_logger.endpoint, encoded_request); err != nil {
		return adapter_error.MapPostError(err)
	}
	return nil
}
