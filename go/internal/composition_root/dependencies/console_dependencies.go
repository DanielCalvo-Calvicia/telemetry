package dependencies

import (
	"io"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console"
)

// NewConsoleDestination builds the console destination that writes to the standard output of the process. An invalid
// setting returns the *config.ValidationError with every problem.
func NewConsoleDestination(settings console.Settings) (outbound.LogSinkPort, error) {
	console_destination, build_error := console.NewStandardOutputDestination(settings)
	if build_error != nil {
		return nil, build_error
	}
	return console_destination, nil
}

// NewConsoleDestinationWithWriter builds the console destination that writes to the writer given (a test, a file, a
// pipe). An invalid setting returns the *config.ValidationError with every problem.
func NewConsoleDestinationWithWriter(settings console.Settings, writer io.Writer) (outbound.LogSinkPort, error) {
	console_destination, build_error := console.NewConsoleDestination(settings, writer)
	if build_error != nil {
		return nil, build_error
	}
	return console_destination, nil
}
