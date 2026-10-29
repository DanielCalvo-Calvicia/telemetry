package value_objects_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func TestParseUTCInstantAcceptsExactlyTheSixDecimalZFormat(test *testing.T) {
	for _, text := range []string{"2026-10-28T10:00:00.123456Z", "2026-10-28T10:00:00.000000Z", "0001-01-01T00:00:00.000000Z", "9999-12-31T23:59:59.999999Z"} {
		parsed, parse_error := value_objects.ParseUTCInstant(text)
		if parse_error != nil || parsed.String() != text {
			test.Errorf("%s: got %s, %v", text, parsed.String(), parse_error)
		}
	}
}

func TestParseUTCInstantRejectsEveryOtherText(test *testing.T) {
	for _, text := range []string{
		"", "yesterday", "2026-10-28T10:00:00Z", "2026-10-28T10:00:00.123Z", "2026-10-28T10:00:00.1234567Z",
		"2026-10-28T10:00:00.123456+00:00", "2026-10-28T12:00:00.123456+02:00", "2026-10-28 10:00:00.123456Z",
		"2026-10-28T10:00:00.123456z", "2026-13-28T10:00:00.123456Z", "0000-01-01T00:00:00.000000Z", " 2026-10-28T10:00:00.123456Z",
	} {
		_, parse_error := value_objects.ParseUTCInstant(text)
		if !errors.Is(parse_error, domain.ErrUTCInstantTextInvalid) || !errors.Is(parse_error, value_objects.ErrUTCInstantTextInvalid) {
			test.Errorf("%q: got %v", text, parse_error)
		}
	}
}
