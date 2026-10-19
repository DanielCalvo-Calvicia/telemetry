// Package logging is the public API of the telemetry library.
package logging

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/composition_root/containers"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/composition_root/dependencies"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/call_depth"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/ports/inbound"
	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/config"
)

// Logger is what a consumer asks of the logger: one method per severity.
type Logger = inbound.LoggerPort

// Entry is the log entry a consumer builds for every call.
type Entry = dtos.LogEntryInboundDTO

// Config is the logging configuration (env tags included) a consumer embeds in its own config.
type Config = config.TelemetryConfig

// CallContext is the identity and depth of a call, carried in the context.
type CallContext = call_depth.CallContext

// StartingIndentation is the indentation of the first call of a request.
const StartingIndentation = call_depth.StartingIndentation

// ErrCallDepthProcessIDFailed is returned when the process identifier of a request cannot be generated.
var ErrCallDepthProcessIDFailed = call_depth.ErrCallDepthProcessIDFailed

// New builds the logger from cfg and returns it with a shutdown function to call on exit. app_env selects the
// minimum level: "development" logs DEBUG and above, "production" INFO and above.
func New(ctx context.Context, app_env string, cfg Config) (Logger, func(context.Context) error, error) {
	c, err := containers.NewLoggingContainer(ctx, app_env, cfg)
	if err != nil {
		return nil, nil, err
	}
	return c.Logger, c.Shutdown, nil
}

// NewRequestContext starts the call context of a request with a new random process identifier.
func NewRequestContext(ctx context.Context, application_name string, application_id string) (context.Context, error) {
	return call_depth.NewRequestContext(ctx, dependencies.NewUUIDGeneratorDependency(), application_name, application_id)
}

// WithCallContext returns a context that carries call_context.
func WithCallContext(ctx context.Context, call_context CallContext) context.Context {
	return call_depth.WithCallContext(ctx, call_context)
}

// FromContext returns the call context carried by ctx and whether there is one.
func FromContext(ctx context.Context) (CallContext, bool) {
	return call_depth.FromContext(ctx)
}

// NextIndentation returns a context one level deeper than ctx and whether ctx carried a call context.
func NextIndentation(ctx context.Context) (context.Context, bool) {
	return call_depth.NextIndentation(ctx)
}
