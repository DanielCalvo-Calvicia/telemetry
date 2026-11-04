// Package helpers holds the pure part of the diagnostics channel: how a diagnostic is written as a line of text.
package helpers

import (
	"strconv"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
)

// =============================================================================
// DIAGNOSTIC TEXT
// =============================================================================

// FormatLine returns the line that the writer of the channel receives for the diagnostic, with the line feed:
// "telemetry diagnostics: code=<code> destination=<kind>/<index> reason=<reason> count=<n> suppressed=<n>".
func FormatLine(event dtos.DiagnosticEvent) string {
	return "telemetry diagnostics: code=" + string(event.Code) +
		" destination=" + event.Kind + "/" + strconv.Itoa(event.Index) +
		" reason=" + event.Reason +
		" count=" + strconv.Itoa(event.Count) +
		" suppressed=" + strconv.Itoa(event.Suppressed) + "\n"
}
