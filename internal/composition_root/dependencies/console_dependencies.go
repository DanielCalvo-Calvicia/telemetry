package dependencies

import (
	console "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/infrastructure/outbound/console"
)

// NewConsoleLoggerDependency builds the console sink of the logger: one JSON line per entry on the standard output.
// It also serves as the fallback sink that receives the report of a sink that failed.
func NewConsoleLoggerDependency() *console.ConsoleLogger {
	return console.NewConsoleLogger()
}
