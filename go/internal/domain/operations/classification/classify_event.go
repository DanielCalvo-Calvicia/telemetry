// Package classification holds the pure rule that gives an event its class (REQ-DLV-009).
package classification

import (
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// CLASSIFY EVENT
// =============================================================================

// ClassifyEvent returns the class of an event: the priority it has when a queue is full. The rule: an assignment for
// the EXACT event type wins (the overrides are the configuration, delivery.event_classes); otherwise an event type that
// starts with "log." is bulk at the severities TRACE and DEBUG and normal at every other severity; any other event type
// is normal. The class is not the severity: a CRITICAL log is normal. It is a priority for a full queue and never a
// guarantee of delivery (REQ-DLV-009). A nil map of overrides is an empty one. It is pure.
func ClassifyEvent(event_type string, severity value_objects.Severity, overrides map[string]value_objects.EventClass) value_objects.EventClass {
	if assigned_class, is_assigned := overrides[event_type]; is_assigned {
		return assigned_class
	}
	if strings.HasPrefix(event_type, "log.") && (severity == value_objects.SeverityTrace || severity == value_objects.SeverityDebug) {
		return value_objects.EventClassBulk
	}
	return value_objects.EventClassNormal
}
