// Package console is the public console destination of the telemetry library: it writes the events of a core to the
// standard output as JSON lines or as readable text, with a minimum severity. A project builds it with NewDestination
// and passes it to the core: telemetry.New(config, telemetry.WithLogSink(sink)). The package reads no environment
// variable and no file: the project passes the Settings in.
package console

import (
	"io"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/dependencies"
	internal_console "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

// Settings are the settings of the console destination: the Format (json or text) and the MinimumSeverity (TRACE, DEBUG,
// INFO, WARN, ERROR or CRITICAL, in upper case).
type Settings = internal_console.Settings

// DefaultSettings returns the settings with their defaults: the json format and a minimum severity of INFO.
func DefaultSettings() Settings { return internal_console.DefaultSettings() }

// NewDestination validates the settings and returns a destination that writes to the standard output. An invalid
// setting returns a *telemetry.ValidationError with every problem. Pass the result to telemetry.WithLogSink.
func NewDestination(settings Settings) (telemetry.LogSinkPort, error) {
	return dependencies.NewConsoleDestination(settings)
}

// NewDestinationWithWriter validates the settings and returns a destination that writes to the writer given instead of
// the standard output. An invalid setting returns a *telemetry.ValidationError with every problem.
func NewDestinationWithWriter(settings Settings, writer io.Writer) (telemetry.LogSinkPort, error) {
	return dependencies.NewConsoleDestinationWithWriter(settings, writer)
}
