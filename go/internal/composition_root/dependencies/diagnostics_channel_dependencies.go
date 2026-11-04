package dependencies

import (
	"io"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/diagnostics_channel"
)

// NewDiagnosticsChannel builds the diagnostics channel of the core: a rate-limited, bounded path of its own to the
// callbacks of the consumer (called in order) and to the writer (nil is none). It starts nothing.
func NewDiagnosticsChannel(clock outbound.ClockPort, callbacks []func(event dtos.DiagnosticEvent), writer io.Writer) outbound.DiagnosticsChannelPort {
	return diagnostics_channel.NewDiagnosticsChannel(clock, callbacks, writer)
}
