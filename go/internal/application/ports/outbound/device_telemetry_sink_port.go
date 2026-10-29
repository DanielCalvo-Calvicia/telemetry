package outbound

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// DEVICE TELEMETRY SINK PORT
// =============================================================================

// DeviceTelemetrySinkPort defines the capability of delivering device telemetry events. It receives events that are
// already validated and already masked. It never returns an error and never panics to the caller: a failure is in the
// result (ADR-005, REQ-DLV-005).
type DeviceTelemetrySinkPort interface {
	// Send delivers the events and reports how many were accepted, dropped or failed.
	Send(ctx context.Context, events []value_objects.Envelope) dtos.SendResult
}
