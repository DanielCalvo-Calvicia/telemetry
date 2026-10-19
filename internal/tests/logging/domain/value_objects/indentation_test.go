package value_objects_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

func Test_indentation(t *testing.T) {
	start := value_objects.NewIndentation(1, 3)
	if start.Indentation != 1 || start.Depth != 3 {
		t.Fatalf("NewIndentation() = %+v", start)
	}

	if next_layer := start.NextIndentation(); next_layer.Indentation != 2 || next_layer.Depth != 1 {
		t.Errorf("NextIndentation() = %+v, want indentation 2 and depth 1", next_layer)
	}
	if next_step := start.NextDepth(); next_step.Indentation != 1 || next_step.Depth != 4 {
		t.Errorf("NextDepth() = %+v, want indentation 1 and depth 4", next_step)
	}
	if start.Indentation != 1 || start.Depth != 3 {
		t.Error("the original value must not change")
	}
	if start.String() != "Indentation: 1, Depth: 3" {
		t.Errorf("String() = %q", start.String())
	}
}
