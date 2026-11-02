package services

import "strings"

// =============================================================================
// DESTINATION KIND
// =============================================================================

// DestinationKind is the kind of sink an event goes to. It follows from the first segment of the event type.
type DestinationKind string

// The kinds of destination, one per sink port.
const (
	DestinationKindLog             DestinationKind = "log"
	DestinationKindEvent           DestinationKind = "event"
	DestinationKindMetrics         DestinationKind = "metrics"
	DestinationKindTrace           DestinationKind = "trace"
	DestinationKindDeviceTelemetry DestinationKind = "device_telemetry"
	DestinationKindAITelemetry     DestinationKind = "ai_telemetry"
)

// ----- FUNCTIONS ------------------------------------------------------------

// DestinationKindOf returns the kind of destination of an event type by its first segment (the text before the first
// dot): log goes to the log sinks, metric to the metrics sinks, trace to the trace sinks, device to the device
// telemetry sinks and ai to the AI telemetry sinks. Every other event type, an empty one included, goes to the event
// sinks. It is a pure function.
func DestinationKindOf(event_type string) DestinationKind {
	first_segment, _, _ := strings.Cut(event_type, ".")
	switch first_segment {
	case "log":
		return DestinationKindLog
	case "metric":
		return DestinationKindMetrics
	case "trace":
		return DestinationKindTrace
	case "device":
		return DestinationKindDeviceTelemetry
	case "ai":
		return DestinationKindAITelemetry
	}
	return DestinationKindEvent
}
