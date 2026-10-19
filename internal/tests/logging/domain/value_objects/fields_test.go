package value_objects_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

func Test_new_fields_copies_the_map(t *testing.T) {
	source_map := map[string]any{"a": 1}
	fields := value_objects.NewFields(source_map)
	source_map["a"] = 2

	if fields["a"] != 1 {
		t.Error("NewFields must copy the source map")
	}
	if from_nil := value_objects.NewFields(nil); from_nil == nil || len(from_nil) != 0 {
		t.Error("NewFields(nil) must give empty, usable Fields")
	}
}

func Test_fields_merge(t *testing.T) {
	first_fields := value_objects.Fields{"a": 1, "b": 2}
	second_fields := value_objects.Fields{"b": 20, "c": 30}

	merged_fields := first_fields.Merge(second_fields)
	if len(merged_fields) != 3 || merged_fields["a"] != 1 || merged_fields["b"] != 20 || merged_fields["c"] != 30 {
		t.Errorf("Merge() = %v, want the second to take precedence", merged_fields)
	}

	merged_fields["a"] = "changed"
	if first_fields["a"] != 1 || second_fields["c"] != 30 {
		t.Error("Merge must not share its result with the inputs")
	}

	var nil_fields value_objects.Fields
	if merged := nil_fields.Merge(nil); merged == nil || len(merged) != 0 {
		t.Error("merging two nil Fields must give empty, usable Fields")
	}
	if merged := nil_fields.Merge(second_fields); len(merged) != 2 {
		t.Errorf("Merge() on nil Fields = %v", merged)
	}
	if merged := first_fields.Merge(nil); len(merged) != 2 {
		t.Errorf("Merge(nil) = %v", merged)
	}
}

func Test_fields_add(t *testing.T) {
	fields := value_objects.Fields{"a": 1}
	returned_fields := fields.Add("b", 2)
	if fields["b"] != 2 || returned_fields["b"] != 2 {
		t.Error("Add must store the pair in place and return the Fields")
	}

	var nil_fields value_objects.Fields
	if added := nil_fields.Add("a", 1); added["a"] != 1 || nil_fields != nil {
		t.Error("Add on nil Fields must return new Fields and leave the nil one alone")
	}
}
