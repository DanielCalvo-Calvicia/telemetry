package canonical_json

import (
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// FormatUTCTime formats an instant as YYYY-MM-DDTHH:MM:SS.ffffffZ: UTC, exactly six decimals with the trailing zeros
// kept, always the Z (ADR-012). Nanoseconds are truncated, not rounded, to microseconds. An instant whose offset is
// not zero, or whose year is outside 0001 to 9999, is rejected with the code event_schema_invalid; it is never
// converted silently. The format lives in value_objects.UTCInstant; this function only classifies its error.
func FormatUTCTime(instant time.Time) (string, error) {
	utc_instant, construction_error := value_objects.NewUTCInstant(instant)
	if construction_error != nil {
		return "", domain.NewClassifiedError(value_objects.ErrorCodeEventSchemaInvalid, construction_error, nil)
	}
	return utc_instant.String(), nil
}
