package helpers_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/diagnostics_channel/helpers"
)

func Test_the_line_has_the_documented_format_and_ends_with_a_line_feed(t *testing.T) {
	line := helpers.FormatLine(dtos.DiagnosticEvent{
		Code: value_objects.DiagnosticCodeEventsDropped, Kind: "log", Index: 1, Reason: "queue_full_newest", Count: 3, Suppressed: 5,
	})
	expected := "telemetry diagnostics: code=events_dropped destination=log/1 reason=queue_full_newest count=3 suppressed=5\n"
	if line != expected {
		t.Fatalf("%q", line)
	}
}
