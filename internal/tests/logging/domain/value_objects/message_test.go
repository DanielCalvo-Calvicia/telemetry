package value_objects_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

func Test_message(t *testing.T) {
	if value_objects.NewMessage("hello").String() != "hello" {
		t.Error("String() must return the text")
	}
	if value_objects.NewMessage("").String() != "" {
		t.Error("an empty message is valid")
	}
}
