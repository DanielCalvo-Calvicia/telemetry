package e2e_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/composition_root"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/inbound/cli"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/tests/fixtures"
)

// check runs the whole check, with the real adapters, on a registry file against the fixture specification, root and
// tasks folder.
func check(registry_path string) (domain.Report, error) {
	return composition_root.NewChecker(fixtures.SpecPath(), registry_path, fixtures.RootDir(), fixtures.TasksDir()).Execute()
}

func check_fixture(test *testing.T, name string) domain.Report {
	test.Helper()
	report, check_error := check(fixtures.RegistryPath(name))
	if check_error != nil {
		test.Fatalf("%s: the check could not run: %v", name, check_error)
	}
	return report
}

// expect_finding fails unless the report has a finding that wraps the sentinel and whose message contains every text.
func expect_finding(test *testing.T, report domain.Report, sentinel error, texts ...string) {
	test.Helper()
	for _, finding := range report.Findings {
		if !errors.Is(finding, sentinel) {
			continue
		}
		all_found := true
		for _, text := range texts {
			all_found = all_found && strings.Contains(finding.Error(), text)
		}
		if all_found {
			return
		}
	}
	test.Errorf("no finding wraps %q with %q; findings: %v", sentinel, texts, report.Findings)
}

func TestTheValidRegistryPassesAndCountsEveryRequirement(test *testing.T) {
	report := check_fixture(test, "valid")
	if !report.Passed() {
		test.Fatalf("findings: %v", report.Findings)
	}
	if report.Total != 3 || report.Covered != 2 || len(report.Pending) != 1 || len(report.Unassigned) != 0 || report.Unaccounted != 0 {
		test.Errorf("total %d, covered %d, pending %v, unassigned %v, unaccounted %d", report.Total, report.Covered, report.Pending, report.Unassigned, report.Unaccounted)
	}
	if report.Pending[0].ID != "REQ-AAA-002" || report.Pending[0].Area != "AAA" || report.Pending[0].Pending != "T-01.2" {
		test.Errorf("pending %+v", report.Pending[0])
	}
	if len(report.Areas) != 2 || report.Areas[0] != (domain.AreaCount{Area: "AAA", Count: 2}) || report.Areas[1] != (domain.AreaCount{Area: "BBB", Count: 1}) {
		test.Errorf("areas %v (the repeated definition must be counted once and the inline mention not at all)", report.Areas)
	}
}

func TestAnUncoveredRequirementFailsAndIsNotCounted(test *testing.T) {
	report := check_fixture(test, "uncovered_requirement")
	if report.Passed() {
		test.Fatal("a requirement with neither tests nor pending must fail")
	}
	expect_finding(test, report, domain.ErrNoEvidence, "uncovered_requirement.yaml", `capability "alpha"`, "REQ-AAA-002")
	if report.Covered != 2 || report.Unaccounted != 1 || len(report.Pending) != 0 {
		test.Errorf("covered %d, unaccounted %d, pending %v", report.Covered, report.Unaccounted, report.Pending)
	}
}

func TestAMissingTestFileFailsAndTheRequirementIsNotCovered(test *testing.T) {
	report := check_fixture(test, "missing_test_file")
	expect_finding(test, report, domain.ErrTestFileMissing, "missing_test_file.yaml", "REQ-AAA-001", "tests/does_not_exist_test.txt")
	if report.Passed() || report.Covered != 1 || report.Unaccounted != 1 {
		test.Errorf("passed %v, covered %d, unaccounted %d", report.Passed(), report.Covered, report.Unaccounted)
	}
}

func TestASupportedCapabilityWithATodoLanguageFails(test *testing.T) {
	report := check_fixture(test, "supported_with_todo_language")
	expect_finding(test, report, domain.ErrNotSupportable, "supported_with_todo_language.yaml", `capability "alpha"`, `python is "todo"`)
	if report.Passed() {
		test.Error("must fail")
	}
}

func TestASupportedCapabilityWithAPendingRequirementFails(test *testing.T) {
	report := check_fixture(test, "supported_with_a_pending_requirement")
	expect_finding(test, report, domain.ErrNotSupportable, `capability "alpha"`, "REQ-AAA-002 has no test that exists")
	if report.Passed() {
		test.Error("must fail")
	}
}

func TestNotApplicableNeedsAReason(test *testing.T) {
	report := check_fixture(test, "not_applicable_without_reason")
	expect_finding(test, report, domain.ErrMissingReason, "not_applicable_without_reason.yaml", `capability "alpha"`, "languages.python")
	if report.Passed() {
		test.Error("must fail")
	}
	with_reason := check_fixture(test, "not_applicable_with_reason")
	if !with_reason.Passed() {
		test.Errorf("a not_applicable with a reason must pass: %v", with_reason.Findings)
	}
	if with_reason.Covered != 3 || with_reason.Capabilities[0].Status != domain.StatusSupported {
		test.Errorf("covered %d, capabilities %+v", with_reason.Covered, with_reason.Capabilities)
	}
}

func TestAnUnknownRequirementFails(test *testing.T) {
	report := check_fixture(test, "unknown_requirement")
	expect_finding(test, report, domain.ErrUnknownRequirement, "unknown_requirement.yaml", "REQ-AAA-999")
	if report.Passed() {
		test.Error("must fail")
	}
}

func TestARequirementOfTheSpecificationThatNoCapabilityListsFails(test *testing.T) {
	report := check_fixture(test, "unlisted_requirement")
	expect_finding(test, report, domain.ErrUnlistedRequirement, "unlisted_requirement.yaml", "REQ-BBB-001 (BBB)", "spec.md")
	if report.Passed() || report.Unaccounted != 1 {
		test.Errorf("passed %v, unaccounted %d", report.Passed(), report.Unaccounted)
	}
}

func TestAPendingTaskThatDoesNotExistFails(test *testing.T) {
	report := check_fixture(test, "pending_task_missing")
	expect_finding(test, report, domain.ErrTaskMissing, "pending_task_missing.yaml", "REQ-AAA-002", "T-77.7")
	if report.Passed() {
		test.Error("must fail")
	}
	if len(report.Pending) != 1 {
		test.Errorf("the requirement still waits for a task, pending %v", report.Pending)
	}
}

func TestPendingUnassignedPassesButIsListedAsAGapAndNeverAsCovered(test *testing.T) {
	report := check_fixture(test, "pending_unassigned")
	if !report.Passed() {
		test.Fatalf("findings: %v", report.Findings)
	}
	if len(report.Unassigned) != 1 || report.Unassigned[0].ID != "REQ-BBB-001" || report.Unassigned[0].Pending != domain.PendingUnassigned {
		test.Errorf("unassigned %v", report.Unassigned)
	}
	if report.Covered != 1 || len(report.Pending) != 1 {
		test.Errorf("covered %d, pending %v", report.Covered, report.Pending)
	}
	var output bytes.Buffer
	cli.Print(&output, report)
	if !strings.Contains(output.String(), "no task yet:\n  area BBB:\n    REQ-BBB-001 (BBB): unassigned\n") {
		test.Errorf("the gap must be listed in its own section:\n%s", output.String())
	}
}

func TestInvalidLevelStatusAndStateFail(test *testing.T) {
	report := check_fixture(test, "invalid_values")
	expect_finding(test, report, domain.ErrInvalidLevel, "invalid_values.yaml", `capability "alpha"`, `"gold"`)
	expect_finding(test, report, domain.ErrInvalidStatus, `capability "alpha"`, `"finished"`)
	expect_finding(test, report, domain.ErrInvalidState, `capability "alpha"`, "languages.python", `"maybe"`)
	if len(report.Findings) != 3 || report.Maturity.Capability != "alpha" {
		test.Errorf("a broken entry must rank least complete: %+v, findings %v", report.Maturity, report.Findings)
	}
}

func TestADuplicatedCapabilityOrRequirementFails(test *testing.T) {
	expect_finding(test, check_fixture(test, "duplicate_capability"), domain.ErrDuplicateCapability, `capability "alpha"`)
	expect_finding(test, check_fixture(test, "duplicate_requirement"), domain.ErrDuplicateRequirement, "REQ-AAA-002", `already listed by capability "alpha"`)
	expect_finding(test, check_fixture(test, "requirement_in_two_capabilities"), domain.ErrDuplicateRequirement, `capability "beta_area"`, "REQ-AAA-002", `capability "alpha"`)
}

func TestBrokenYAMLAnUnknownKeyAndAnEmptyFileNameTheFile(test *testing.T) {
	for _, name := range []string{"broken", "unknown_key", "empty"} {
		_, check_error := check(fixtures.RegistryPath(name))
		if !errors.Is(check_error, application.ErrInvalidYAML) || !strings.Contains(check_error.Error(), name+".yaml") {
			test.Errorf("%s: %v", name, check_error)
		}
	}
}

func TestAMissingRegistryOrSpecOrTasksFolderStopsTheCheckNamingThePath(test *testing.T) {
	missing := filepath.Join(test.TempDir(), "no_such_file")
	_, registry_error := check(missing)
	if !errors.Is(registry_error, application.ErrReadRegistry) || !strings.Contains(registry_error.Error(), "no_such_file") {
		test.Errorf("registry: %v", registry_error)
	}
	_, spec_error := composition_root.NewChecker(missing, fixtures.RegistryPath("valid"), fixtures.RootDir(), fixtures.TasksDir()).Execute()
	if !errors.Is(spec_error, application.ErrReadSpec) || !strings.Contains(spec_error.Error(), "no_such_file") {
		test.Errorf("spec: %v", spec_error)
	}
	_, tasks_error := composition_root.NewChecker(fixtures.SpecPath(), fixtures.RegistryPath("valid"), fixtures.RootDir(), missing).Execute()
	if !errors.Is(tasks_error, application.ErrReadTasks) || !strings.Contains(tasks_error.Error(), "no_such_file") {
		test.Errorf("tasks: %v", tasks_error)
	}
}

func TestASpecificationWithoutRequirementsIsAnErrorAndNotAVacuousPass(test *testing.T) {
	empty_spec := fixtures.Write(test, "empty_spec.md", "# nothing is defined here\n")
	_, check_error := composition_root.NewChecker(empty_spec, fixtures.RegistryPath("valid"), fixtures.RootDir(), fixtures.TasksDir()).Execute()
	if !errors.Is(check_error, application.ErrNoRequirementsInSpec) || !strings.Contains(check_error.Error(), "empty_spec.md") {
		test.Errorf("%v", check_error)
	}
}

func TestAFolderIsNotATestFileAndATaskIdInsideAnotherNameIsNotATask(test *testing.T) {
	folder_registry := fixtures.Variant(test, "valid", "tests/beta_test.txt", "tests/a_folder")
	report, check_error := check(folder_registry)
	if check_error != nil {
		test.Fatal(check_error)
	}
	expect_finding(test, report, domain.ErrTestFileMissing, "tests/a_folder")
	task_registry := fixtures.Variant(test, "valid", "pending: T-01.2", "pending: T-09")
	report, check_error = check(task_registry)
	if check_error != nil {
		test.Fatal(check_error)
	}
	expect_finding(test, report, domain.ErrTaskMissing, "T-09")
}

func TestAParentTaskIsAValidPending(test *testing.T) {
	report, check_error := check(fixtures.Variant(test, "valid", "pending: T-01.2", "pending: T-02"))
	if check_error != nil || !report.Passed() {
		test.Errorf("%v %v", check_error, report.Findings)
	}
}

func TestEveryRuleBreachIsReportedAtOnce(test *testing.T) {
	registry := fixtures.Variant(test, "valid", "pending: T-01.2", "pending: soon")
	registry_content, _ := os.ReadFile(registry)
	broken := strings.Replace(string(registry_content), "tests/beta_test.txt", "tests/gone_test.txt", 1)
	broken = strings.Replace(broken, "level: beta", "level: gold", 1)
	report, check_error := check(fixtures.Write(test, "two_breaches.yaml", broken))
	if check_error != nil {
		test.Fatal(check_error)
	}
	expect_finding(test, report, domain.ErrInvalidPending, `"soon"`)
	expect_finding(test, report, domain.ErrTestFileMissing, "tests/gone_test.txt")
	expect_finding(test, report, domain.ErrInvalidLevel, `"gold"`)
}

func TestTheCommandLineExitsZeroOrOneAndPrintsTheReport(test *testing.T) {
	arguments := func(registry_path string) []string {
		return []string{"--spec", fixtures.SpecPath(), "--registry", registry_path, "--root", fixtures.RootDir(), "--tasks", fixtures.TasksDir()}
	}
	var output, messages bytes.Buffer
	if code := cli.Run(arguments(fixtures.RegistryPath("valid")), &output, &messages, composition_root.NewChecker); code != cli.ExitOK || messages.Len() != 0 {
		test.Errorf("valid: code %d, messages %q", code, messages.String())
	}
	for _, text := range []string{"3 total, 2 covered by tests, 1 pending, 0 no task yet", "REQ-AAA-002 (AAA): pending T-01.2", "library maturity: beta (least complete capability: ", "result: PASSED"} {
		if !strings.Contains(output.String(), text) {
			test.Errorf("the output lacks %q:\n%s", text, output.String())
		}
	}
	output.Reset()
	messages.Reset()
	if code := cli.Run(arguments(fixtures.RegistryPath("uncovered_requirement")), &output, &messages, composition_root.NewChecker); code != cli.ExitFailure {
		test.Errorf("uncovered: code %d", code)
	}
	if !strings.Contains(output.String(), "result: FAILED") || !strings.Contains(messages.String(), "uncovered_requirement.yaml") || !strings.Contains(messages.String(), "REQ-AAA-002") {
		test.Errorf("output:\n%s\nmessages:\n%s", output.String(), messages.String())
	}
}
