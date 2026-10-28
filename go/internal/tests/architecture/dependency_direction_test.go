package architecture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/architecture/checker"
)

func Test_the_real_module_keeps_the_dependencies_pointing_inward(t *testing.T) {
	require_no_violation(t, real_module_violations(t),
		checker.RuleDomainIsolated,
		checker.RuleApplicationImportsOnlyDomain,
		checker.RuleInfrastructureNotCompositionRoot,
		checker.RuleIOOnlyInExternal,
	)
}

func Test_a_clean_tree_has_no_violation(t *testing.T) {
	violations := fixture_violations(t, "clean")
	if len(violations) != 0 {
		t.Fatalf("the clean tree must have no violation, got %v", violations)
	}
}

func Test_a_planted_violation_of_each_dependency_rule_is_reported(t *testing.T) {
	cases := []struct {
		fixture     string
		rule        string
		file        string
		import_path string
	}{
		{"domain_imports_infrastructure", checker.RuleDomainIsolated, "internal/domain/bad.go", fixture_module_path + "/internal/infrastructure/config"},
		{"domain_imports_io", checker.RuleDomainIsolated, "internal/domain/bad.go", "net/http"},
		{"domain_imports_backend", checker.RuleDomainIsolated, "internal/domain/bad.go", "go.opentelemetry.io/otel"},
		{"application_imports_adapter", checker.RuleApplicationImportsOnlyDomain, "internal/application/bad.go", fixture_module_path + "/internal/infrastructure/outbound/console"},
		{"infrastructure_imports_composition_root", checker.RuleInfrastructureNotCompositionRoot, "internal/infrastructure/config/bad.go", fixture_module_path + "/internal/composition_root/containers"},
		{"io_outside_external", checker.RuleIOOnlyInExternal, "internal/infrastructure/outbound/console/console.go", "net/http"},
	}
	for _, planted := range cases {
		t.Run(planted.fixture, func(t *testing.T) {
			require_violation(t, fixture_violations(t, planted.fixture), planted.rule, planted.file, planted.import_path)
		})
	}
}

func Test_the_io_of_an_adapter_is_allowed_in_its_external_folder(t *testing.T) {
	for _, violation := range checker.OfRule(fixture_violations(t, "io_outside_external"), checker.RuleIOOnlyInExternal) {
		if violation.File == "internal/infrastructure/outbound/console/external/client.go" {
			t.Fatalf("the external folder may import I/O packages, got %v", violation)
		}
	}
}

func Test_a_file_that_cannot_be_parsed_is_an_error(t *testing.T) {
	root := t.TempDir()
	domain_folder := filepath.Join(root, "internal", "domain")
	if err := os.MkdirAll(domain_folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domain_folder, "broken.go"), []byte("package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checker.Check(root, fixture_module_path); err == nil {
		t.Fatal("a Go file that cannot be parsed must be an error, not an accepted tree")
	}
}

func Test_a_root_that_cannot_be_read_is_an_error(t *testing.T) {
	if _, err := checker.Check("not\x00a-valid-root", fixture_module_path); err == nil {
		t.Fatal("a root that cannot be read must be an error, not an accepted tree")
	}
}
