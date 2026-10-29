package e2e_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/composition_root"
)

// repository_root walks up from this file until it finds go.work, the marker of the root of the repository.
func repository_root() (string, bool) {
	_, source, _, _ := runtime.Caller(0)
	current := filepath.Dir(source)
	for {
		if _, stat_error := os.Stat(filepath.Join(current, "go.work")); stat_error == nil {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func TestTheRealRegistryPassesAgainstTheRealSpecificationTestsAndTasks(test *testing.T) {
	root, found := repository_root()
	if !found {
		test.Skip("the root of the repository (go.work) was not found: the real registry cannot be checked from here")
	}
	spec_path := filepath.Join(root, "docs", "proposals", "PR-001-telemetry-standard-go-python.md")
	registry_path := filepath.Join(root, "contracts", "capabilities", "registry.yaml")
	for _, required := range []string{spec_path, registry_path} {
		if _, stat_error := os.Stat(required); stat_error != nil {
			test.Skipf("%s is not here (a partial checkout?): %v", required, stat_error)
		}
	}
	report, check_error := composition_root.NewChecker(spec_path, registry_path, root, filepath.Join(root, "docs", "tasks")).Execute()
	if check_error != nil {
		test.Fatal(check_error)
	}
	if !report.Passed() {
		test.Fatalf("the real registry breaks %d rule(s):\n%s", len(report.Findings), join_findings(report.Findings))
	}
	if report.Total == 0 || report.Unaccounted != 0 {
		test.Errorf("total %d, not accounted for %d", report.Total, report.Unaccounted)
	}
	if report.Covered+len(report.Pending)+len(report.Unassigned) != report.Total {
		test.Errorf("covered %d + pending %d + unassigned %d is not the total %d", report.Covered, len(report.Pending), len(report.Unassigned), report.Total)
	}
}

func join_findings(findings []error) string {
	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, "  "+finding.Error())
	}
	return strings.Join(lines, "\n")
}
