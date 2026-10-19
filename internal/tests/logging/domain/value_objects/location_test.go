package value_objects_test

import (
	"encoding/json"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

func Test_location(t *testing.T) {
	location := value_objects.NewLocation("app", "service", "Put")
	if location.Application != "app" || location.Category != "service" || location.Function != "Put" {
		t.Fatalf("NewLocation() = %+v", location)
	}

	changed_location := location.WithCategory("adapter").WithFunction("Get")
	if changed_location.Category != "adapter" || changed_location.Function != "Get" || changed_location.Application != "app" {
		t.Errorf("With... = %+v", changed_location)
	}
	if location.Category != "service" || location.Function != "Put" {
		t.Error("With... must return a copy and leave the original unchanged")
	}

	without_function, err := json.Marshal(value_objects.NewLocation("app", "service", ""))
	if err != nil || string(without_function) != `{"application":"app","category":"service"}` {
		t.Errorf("json = %s, %v; an empty function must be omitted", without_function, err)
	}
}
