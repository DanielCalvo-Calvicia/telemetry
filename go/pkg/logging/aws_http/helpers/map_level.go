package helpers

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// MAP LEVEL
// =============================================================================

// MapLevel returns the level that the add_log endpoint knows for a severity: the same text, except CRITICAL, which the
// old levels called FATAL.
func MapLevel(severity value_objects.Severity) string {
	if severity == value_objects.SeverityCritical {
		return "FATAL"
	}
	return string(severity)
}
