package architecture_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/architecture/checker"
)

const (
	real_module_path    = "github.com/DanielCalvo-Calvicia/telemetry/go"
	fixture_module_path = "example.test/telemetry/go"
)

// real_module_violations checks the Go core module itself: the tests run in go/internal/tests/architecture.
func real_module_violations(t *testing.T) []checker.Violation {
	t.Helper()
	violations, err := checker.Check(filepath.Join("..", "..", ".."), real_module_path)
	if err != nil {
		t.Fatalf("the real module could not be checked: %v", err)
	}
	return violations
}

// fixture_violations checks one planted-violation tree of the fixtures.
func fixture_violations(t *testing.T, case_name string) []checker.Violation {
	t.Helper()
	case_root := filepath.Join("..", "fixtures", "architecture", "testdata", case_name)
	violations, err := checker.Check(case_root, fixture_module_path)
	if err != nil {
		t.Fatalf("the fixture %s could not be checked: %v", case_name, err)
	}
	return violations
}

// require_violation fails unless the violations hold one of the rule on the file with the import, and unless its
// message names the offending file and the forbidden import.
func require_violation(t *testing.T, violations []checker.Violation, rule string, file string, import_path string) {
	t.Helper()
	for _, violation := range checker.OfRule(violations, rule) {
		if violation.File == file && violation.Import == import_path {
			message := violation.Message()
			if !strings.Contains(message, file) || !strings.Contains(message, import_path) {
				t.Fatalf("the message %q does not name the file %q and the import %q", message, file, import_path)
			}
			return
		}
	}
	t.Fatalf("no violation of %s on %s importing %s in %v", rule, file, import_path, violations)
}

// require_no_violation fails if the violations hold any of the rules.
func require_no_violation(t *testing.T, violations []checker.Violation, rules ...string) {
	t.Helper()
	for _, rule := range rules {
		if found := checker.OfRule(violations, rule); len(found) > 0 {
			var messages []string
			for _, violation := range found {
				messages = append(messages, violation.Message())
			}
			t.Fatalf("unexpected violations:\n%s", strings.Join(messages, "\n"))
		}
	}
}
