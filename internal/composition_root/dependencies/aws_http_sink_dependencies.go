package dependencies

import (
	logging_config "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/config"
	aws_http "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/aws_http"
)

// NewAWSHTTPLoggerDependency builds the AWS HTTP sink of the logger from the host, the port and the path of the
// add_log endpoint of aws_microservice and the log group and the log stream that receive the entries
// (LOGGER_AWS_HTTP_*). It fails when one of them is empty.
func NewAWSHTTPLoggerDependency(logging_settings logging_config.TelemetryConfig) (*aws_http.AWSHTTPLogger, error) {
	return aws_http.NewAWSHTTPLogger(logging_settings.AWSHTTPHost, logging_settings.AWSHTTPPort, logging_settings.AWSHTTPPath, logging_settings.AWSHTTPLogGroup, logging_settings.AWSHTTPLogStream)
}
