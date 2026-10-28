package canonical_json

import (
	"errors"
	"fmt"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// FormatUTCTime formats an instant as YYYY-MM-DDTHH:MM:SS.ffffffZ: UTC, exactly six decimals with the trailing zeros
// kept, always the Z (ADR-012). Nanoseconds are truncated, not rounded, to microseconds. An instant whose offset is
// not zero, or whose year is outside 0000 to 9999, is rejected with the code event_schema_invalid; it is never
// converted silently.
func FormatUTCTime(instant time.Time) (string, error) {
	if _, offset_seconds := instant.Zone(); offset_seconds != 0 {
		return "", domain.NewClassifiedError(
			value_objects.ErrorCodeEventSchemaInvalid, domain.ErrNonUTCTime,
			fmt.Errorf("the offset is %d seconds", offset_seconds))
	}
	utc_instant := instant.UTC()
	if utc_instant.Year() < 0 || utc_instant.Year() > 9999 {
		return "", domain.NewClassifiedError(
			value_objects.ErrorCodeEventSchemaInvalid, domain.ErrNonUTCTime,
			errors.New("the year is outside 0000 to 9999"))
	}
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%06dZ",
		utc_instant.Year(), int(utc_instant.Month()), utc_instant.Day(),
		utc_instant.Hour(), utc_instant.Minute(), utc_instant.Second(),
		utc_instant.Nanosecond()/1000), nil
}
