package logging

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/composition_root/containers"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/composition_root/dependencies"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/outbound"
)

// Sink is a destination of log entries. NewConsoleSink, NewHTTPSink and NewAWSHTTPSink build the ready-made ones; a
// consumer may also implement Sink for a destination of its own.
type Sink = outbound.LogSinkPort

// SinkEntry is the finished log entry a Sink receives: the flat payload the console prints and the HTTP sinks post.
type SinkEntry = dtos.LogEntryOutboundDTO

// NewConsoleSink builds the console sink: one JSON line per entry on the standard output.
func NewConsoleSink() Sink {
	return dependencies.NewConsoleLoggerDependency()
}

// NewHTTPSink builds the HTTP sink: it posts every entry as flat JSON to a generic log receiver at host:port/path.
// The host includes the scheme, for example "http://localhost". It fails when the host, the port or the path is
// empty.
func NewHTTPSink(host string, port string, path string) (Sink, error) {
	http_sink, err := dependencies.NewHTTPLoggerDependency(Config{Host: host, Port: port, Path: path})
	if err != nil {
		return nil, err
	}
	return http_sink, nil
}

// NewAWSHTTPSink builds the AWS HTTP sink: it adapts every entry to the add_log endpoint of aws_microservice
// (POST /api/v1/cloudwatch/logs/add) at host:port/path, so the entry lands in the CloudWatch Logs stream log_stream_name
// of the log group log_group_name (the endpoint creates them when they do not exist). The host includes the scheme,
// for example "http://aws-microservice". It fails when any of the five values is empty.
func NewAWSHTTPSink(host string, port string, path string, log_group_name string, log_stream_name string) (Sink, error) {
	aws_http_sink, err := dependencies.NewAWSHTTPLoggerDependency(Config{
		AWSHTTPHost: host, AWSHTTPPort: port, AWSHTTPPath: path,
		AWSHTTPLogGroup: log_group_name, AWSHTTPLogStream: log_stream_name,
	})
	if err != nil {
		return nil, err
	}
	return aws_http_sink, nil
}

// NewWithSinks builds the logger over the sinks the caller built, instead of the sinks a Config switches on. app_env
// selects the minimum level as in New and mask_extra_patterns are regular expressions added to the built-in detectors
// of sensitive data (nil for none). It returns the logger with a shutdown function to call on exit.
func NewWithSinks(ctx context.Context, app_env string, mask_extra_patterns []string, sinks ...Sink) (Logger, func(context.Context) error, error) {
	c, err := containers.NewLoggingContainerFromSinks(ctx, app_env, mask_extra_patterns, sinks)
	if err != nil {
		return nil, nil, err
	}
	return c.Logger, c.Shutdown, nil
}
