package architecture_test

import (
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/architecture/checker"
)

func Test_the_real_module_wires_the_adapters_only_in_the_composition_root(t *testing.T) {
	require_no_violation(t, real_module_violations(t), checker.RuleAdaptersWiredByCompositionRoot)
}

func Test_an_adapter_imported_outside_a_composition_root_is_reported(t *testing.T) {
	violations := fixture_violations(t, "adapter_outside_composition_root")
	require_violation(t, violations, checker.RuleAdaptersWiredByCompositionRoot,
		"internal/infrastructure/config/bad.go", fixture_module_path+"/internal/infrastructure/outbound/console")
}

func Test_a_composition_root_may_import_an_adapter(t *testing.T) {
	require_no_violation(t, fixture_violations(t, "clean"), checker.RuleAdaptersWiredByCompositionRoot)
}
