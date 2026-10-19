package value_objects_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

func Test_identifiers(t *testing.T) {
	identifiers := value_objects.NewIdentifiers("producer", "application", "process")

	if identifiers.ProducerID != "producer" || identifiers.ApplicationID != "application" || identifiers.ProcessID != "process" {
		t.Errorf("NewIdentifiers() = %+v", identifiers)
	}
}
