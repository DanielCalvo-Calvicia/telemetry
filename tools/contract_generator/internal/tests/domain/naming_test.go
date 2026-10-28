package domain_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

func TestNamesAreDerivedMechanically(test *testing.T) {
	cases := []struct {
		name        string
		pascal_case string
		upper_snake string
	}{
		{"log.entry.recorded", "LogEntryRecorded", "LOG_ENTRY_RECORDED"},
		{"event_schema_invalid", "EventSchemaInvalid", "EVENT_SCHEMA_INVALID"},
		{"TRACE", "Trace", "TRACE"},
		{"severity", "Severity", "SEVERITY"},
		{"a1_b2", "A1B2", "A1_B2"},
		{"", "", ""},
	}
	for _, each := range cases {
		if got := domain.PascalCase(each.name); got != each.pascal_case {
			test.Errorf("PascalCase(%q) = %q, want %q", each.name, got, each.pascal_case)
		}
		if got := domain.UpperSnake(each.name); got != each.upper_snake {
			test.Errorf("UpperSnake(%q) = %q, want %q", each.name, got, each.upper_snake)
		}
	}
}

func TestTheSchemaPathOfAnEventType(test *testing.T) {
	if got := domain.SchemaPathOf("log.entry.recorded"); got != "schemas/log/entry_recorded.schema.json" {
		test.Errorf("got %q", got)
	}
}
