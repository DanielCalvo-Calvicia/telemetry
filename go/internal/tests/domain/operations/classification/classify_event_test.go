package classification_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/classification"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// fixtures_file is the case file that the Python tests read too (the same file).
const fixtures_file = "../../../../../../integration-tests/fixtures/delivery/classification_cases.json"

type classification_case struct {
	Description string            `json:"description"`
	EventType   string            `json:"event_type"`
	Severity    string            `json:"severity"`
	Overrides   map[string]string `json:"overrides"`
	Expected    string            `json:"expected"`
}

func Test_every_shared_classification_case_gives_the_same_class(t *testing.T) {
	content, read_error := os.ReadFile(fixtures_file)
	if read_error != nil {
		t.Fatal(read_error)
	}
	var fixture struct {
		Cases []classification_case `json:"cases"`
	}
	if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("the fixture has no case")
	}
	for _, test_case := range fixture.Cases {
		t.Run(test_case.Description, func(t *testing.T) {
			overrides := map[string]value_objects.EventClass{}
			for event_type, event_class := range test_case.Overrides {
				overrides[event_type] = value_objects.EventClass(event_class)
			}
			got := classification.ClassifyEvent(test_case.EventType, value_objects.Severity(test_case.Severity), overrides)
			if string(got) != test_case.Expected {
				t.Fatalf("got %q, want %q", got, test_case.Expected)
			}
		})
	}
}

func Test_a_nil_map_of_overrides_is_an_empty_one(t *testing.T) {
	if got := classification.ClassifyEvent("log.entry.recorded", value_objects.SeverityTrace, nil); got != value_objects.EventClassBulk {
		t.Errorf("a trace log is %q", got)
	}
	if got := classification.ClassifyEvent("device.fault.raised", value_objects.SeverityInfo, nil); got != value_objects.EventClassNormal {
		t.Errorf("another type is %q", got)
	}
}

func Test_a_type_that_only_looks_like_a_log_type_is_normal(t *testing.T) {
	for _, event_type := range []string{"logs.entry", "log", "catalog.entry", ""} {
		if got := classification.ClassifyEvent(event_type, value_objects.SeverityDebug, nil); got != value_objects.EventClassNormal {
			t.Errorf("%q at DEBUG is %q", event_type, got)
		}
	}
}

func Test_the_assignment_is_for_the_exact_event_type(t *testing.T) {
	overrides := map[string]value_objects.EventClass{"log.entry": value_objects.EventClassCritical}
	if got := classification.ClassifyEvent("log.entry.recorded", value_objects.SeverityInfo, overrides); got != value_objects.EventClassNormal {
		t.Errorf("a prefix of the type is not an assignment: %q", got)
	}
}
