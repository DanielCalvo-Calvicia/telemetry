package architecture_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/architecture/checker"
)

func Test_the_real_module_has_no_consumer_that_imports_internal(t *testing.T) {
	require_no_violation(t, real_module_violations(t), checker.RuleConsumerNeverImportsInternal)
}

func Test_a_consumer_that_imports_an_internal_package_is_reported(t *testing.T) {
	violations := fixture_violations(t, "consumer_imports_internal")
	require_violation(t, violations, checker.RuleConsumerNeverImportsInternal,
		"examples/smoke/main.go", fixture_module_path+"/internal/domain")
}

func Test_a_consumer_that_imports_only_the_public_package_is_accepted(t *testing.T) {
	require_no_violation(t, fixture_violations(t, "clean"), checker.RuleConsumerNeverImportsInternal)
}

func Test_a_folder_that_does_not_exist_has_no_violation(t *testing.T) {
	violations, err := checker.Check(t.TempDir(), fixture_module_path)
	if err != nil || len(violations) != 0 {
		t.Fatalf("an empty root must be accepted, got %v, %v", violations, err)
	}
}
