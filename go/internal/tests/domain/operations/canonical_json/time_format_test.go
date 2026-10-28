package canonical_json_test

import (
	"errors"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func TestFormatUTCTimeKeepsSixDecimalsAndTheZ(test *testing.T) {
	cases := []struct {
		instant  time.Time
		expected string
	}{
		{time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC), "2026-10-28T10:00:00.000000Z"},
		{time.Date(2026, 10, 28, 10, 0, 1, 500_000_000, time.UTC), "2026-10-28T10:00:01.500000Z"},
		{time.Date(2026, 1, 2, 3, 4, 5, 123_000, time.UTC), "2026-01-02T03:04:05.000123Z"},
		{time.Date(2026, 10, 28, 10, 0, 0, 1_999, time.UTC), "2026-10-28T10:00:00.000001Z"},
		{time.Date(2026, 10, 28, 10, 0, 0, 999_999_999, time.UTC), "2026-10-28T10:00:00.999999Z"},
		{time.Date(2026, 10, 28, 10, 0, 0, 0, time.FixedZone("zero", 0)), "2026-10-28T10:00:00.000000Z"},
		{time.Date(5, 1, 2, 3, 4, 5, 0, time.UTC), "0005-01-02T03:04:05.000000Z"},
	}
	for _, current := range cases {
		got, format_error := canonical_json.FormatUTCTime(current.instant)
		if format_error != nil || got != current.expected {
			test.Errorf("%v: got %q (%v), want %q", current.instant, got, format_error, current.expected)
		}
	}
}

func TestFormatUTCTimeRejectsAnInstantThatIsNotUTC(test *testing.T) {
	madrid := time.FixedZone("CEST", 2*60*60)
	for _, instant := range []time.Time{
		time.Date(2026, 10, 28, 12, 0, 0, 0, madrid),
		time.Date(2026, 10, 28, 12, 0, 0, 0, time.FixedZone("west", -5*60*60)),
		time.Date(2026, 10, 28, 12, 0, 0, 0, time.FixedZone("odd", 30*60)),
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		got, format_error := canonical_json.FormatUTCTime(instant)
		if format_error == nil || got != "" {
			test.Errorf("%v was accepted as %q", instant, got)
			continue
		}
		if error_code(format_error) != value_objects.ErrorCodeEventSchemaInvalid || !errors.Is(format_error, domain.ErrNonUTCTime) {
			test.Errorf("%v: %v", instant, format_error)
		}
	}
}

func TestATimeInsideAValueIsWrittenWithTheFixedFormat(test *testing.T) {
	serialized, serialize_error := canonical_json.CanonicalJSON(map[string]any{
		"occurred_at": time.Date(2026, 10, 28, 10, 0, 0, 7_000, time.UTC),
	})
	if serialize_error != nil || string(serialized) != `{"occurred_at":"2026-10-28T10:00:00.000007Z"}` {
		test.Errorf("got %q %v", serialized, serialize_error)
	}
	_, serialize_error = canonical_json.CanonicalJSON(map[string]any{
		"occurred_at": time.Date(2026, 10, 28, 12, 0, 0, 0, time.FixedZone("CEST", 2*60*60)),
	})
	if serialize_error == nil || !errors.Is(serialize_error, domain.ErrNonUTCTime) {
		test.Errorf("a non-UTC time inside a value was accepted: %v", serialize_error)
	}
}
