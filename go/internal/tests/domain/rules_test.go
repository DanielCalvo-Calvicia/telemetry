package domain_test

import (
	"regexp"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

func TestTheSchemaVersionIsMajorDotMinor(test *testing.T) {
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+$`).MatchString(domain.SchemaVersion) {
		test.Fatalf("schema_version %q does not match ^[0-9]+\\.[0-9]+$", domain.SchemaVersion)
	}
}
