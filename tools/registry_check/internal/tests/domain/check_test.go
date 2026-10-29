package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
)

var spec = domain.Spec{Path: "spec.md", IDs: []string{"REQ-AAA-001", "REQ-AAA-002", "REQ-BB-001"}}

var existence = domain.Existence{
	Files: map[string]bool{"tests/a_test.go": true, "tests/test_a.py": true},
	Tasks: map[string]bool{"T-01.1": true, "T-02": true},
}

func passing_tests() []domain.TestRef {
	return []domain.TestRef{{Language: "go", Path: "tests/a_test.go"}, {Language: "python", Path: "tests/test_a.py"}}
}

// valid_registry is a registry that passes; each test breaks one thing of it.
func valid_registry() domain.Registry {
	return domain.Registry{
		Path:    "registry.yaml",
		Version: 1,
		Capabilities: []domain.Capability{
			{
				ID: "alpha", Title: "Alpha", Description: "The first.", Level: "beta", Status: "in_progress",
				Go: domain.LanguageState{State: "passing"}, Python: domain.LanguageState{State: "built"},
				Requirements: []domain.Requirement{
					{ID: "REQ-AAA-001", Tests: passing_tests()},
					{ID: "REQ-AAA-002", Pending: "T-01.1"},
				},
			},
			{
				ID: "beta_area", Title: "Beta", Description: "The second.", Level: "beta", Status: "planned",
				Go: domain.LanguageState{State: "todo"}, Python: domain.LanguageState{State: "todo"},
				Requirements: []domain.Requirement{{ID: "REQ-BB-001", Pending: "unassigned"}},
			},
		},
	}
}

func expect_one_finding(test *testing.T, registry domain.Registry, sentinel error, texts ...string) domain.Report {
	test.Helper()
	report := domain.Check(registry, spec, existence)
	if len(report.Findings) != 1 || !errors.Is(report.Findings[0], sentinel) {
		test.Fatalf("want exactly one finding of %q, got %v", sentinel, report.Findings)
	}
	for _, text := range texts {
		if !strings.Contains(report.Findings[0].Error(), text) {
			test.Errorf("the message %q lacks %q", report.Findings[0], text)
		}
	}
	if !strings.HasPrefix(report.Findings[0].Error(), "registry.yaml: ") {
		test.Errorf("every message starts with the registry file: %q", report.Findings[0])
	}
	return report
}

func TestTheValidRegistryHasNoFinding(test *testing.T) {
	report := domain.Check(valid_registry(), spec, existence)
	if !report.Passed() {
		test.Fatalf("%v", report.Findings)
	}
	if report.Total != 3 || report.Covered != 1 || len(report.Pending) != 1 || len(report.Unassigned) != 1 || report.Unaccounted != 0 {
		test.Errorf("%+v", report)
	}
	if report.Areas[1] != (domain.AreaCount{Area: "BB", Count: 1}) {
		test.Errorf("a two letter area is allowed: %v", report.Areas)
	}
}

func TestEveryFieldRuleReportsItsOwnSentinel(test *testing.T) {
	cases := []struct {
		name     string
		mutate   func(registry *domain.Registry)
		sentinel error
		text     string
	}{
		{"version", func(r *domain.Registry) { r.Version = 2 }, domain.ErrUnsupportedVersion, "registry_version is 2"},
		{"capability id", func(r *domain.Registry) { r.Capabilities[0].ID = "Alpha-1" }, domain.ErrInvalidCapabilityID, `"Alpha-1"`},
		{"empty title", func(r *domain.Registry) { r.Capabilities[0].Title = "  " }, domain.ErrMissingField, "title"},
		{"empty description", func(r *domain.Registry) { r.Capabilities[0].Description = "" }, domain.ErrMissingField, "description"},
		{"level", func(r *domain.Registry) { r.Capabilities[0].Level = "alpha" }, domain.ErrInvalidLevel, `"alpha"`},
		{"status", func(r *domain.Registry) { r.Capabilities[0].Status = "" }, domain.ErrInvalidStatus, `""`},
		{"go state", func(r *domain.Registry) { r.Capabilities[0].Go.State = "done" }, domain.ErrInvalidState, "languages.go"},
		{"python state", func(r *domain.Registry) { r.Capabilities[0].Python.State = "" }, domain.ErrInvalidState, "languages.python"},
		{"reason of go", func(r *domain.Registry) { r.Capabilities[0].Go = domain.LanguageState{State: "not_applicable"} }, domain.ErrMissingReason, "languages.go"},
		{"no requirements", func(r *domain.Registry) { r.Capabilities[1].Requirements = nil }, domain.ErrNoRequirements, `capability "beta_area"`},
	}
	for _, scenario := range cases {
		registry := valid_registry()
		scenario.mutate(&registry)
		report := domain.Check(registry, spec, existence)
		found := false
		for _, finding := range report.Findings {
			found = found || (errors.Is(finding, scenario.sentinel) && strings.Contains(finding.Error(), scenario.text))
		}
		if !found {
			test.Errorf("%s: no finding of %q with %q in %v", scenario.name, scenario.sentinel, scenario.text, report.Findings)
		}
	}
}

func TestTheRequirementRules(test *testing.T) {
	cases := []struct {
		name        string
		requirement domain.Requirement
		sentinel    error
		text        string
	}{
		{"both evidences", domain.Requirement{ID: "REQ-AAA-002", Pending: "T-01.1", Tests: passing_tests()}, domain.ErrTwoEvidences, "REQ-AAA-002"},
		{"invalid pending", domain.Requirement{ID: "REQ-AAA-002", Pending: "later"}, domain.ErrInvalidPending, `"later"`},
		{"missing task", domain.Requirement{ID: "REQ-AAA-002", Pending: "T-09.9"}, domain.ErrTaskMissing, "T-09.9"},
		{"language", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "rust", Path: "tests/a_test.go"}}}, domain.ErrInvalidTest, `language "rust"`},
		{"absolute path", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: "/tests/a_test.go"}}}, domain.ErrInvalidTest, "clean slash path"},
		{"backslash", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: `tests\a_test.go`}}}, domain.ErrInvalidTest, "clean slash path"},
		{"drive", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: "C:/a_test.go"}}}, domain.ErrInvalidTest, "clean slash path"},
		{"parent", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: "../a_test.go"}}}, domain.ErrInvalidTest, "clean slash path"},
		{"inner parent", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: "tests/../tests/a_test.go"}}}, domain.ErrInvalidTest, "clean slash path"},
		{"dot", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: "."}}}, domain.ErrInvalidTest, "clean slash path"},
		{"empty path", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: ""}}}, domain.ErrInvalidTest, "clean slash path"},
		{"missing file", domain.Requirement{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: "tests/b_test.go"}}}, domain.ErrTestFileMissing, "tests/b_test.go"},
	}
	for _, scenario := range cases {
		registry := valid_registry()
		registry.Capabilities[0].Requirements[1] = scenario.requirement
		report := domain.Check(registry, spec, existence)
		found := false
		for _, finding := range report.Findings {
			found = found || (errors.Is(finding, scenario.sentinel) && strings.Contains(finding.Error(), scenario.text))
		}
		if !found {
			test.Errorf("%s: no finding of %q with %q in %v", scenario.name, scenario.sentinel, scenario.text, report.Findings)
		}
		if report.Covered != 1 {
			test.Errorf("%s: only REQ-AAA-001 is covered, got %d", scenario.name, report.Covered)
		}
	}
}

func TestATestOfALanguageThatIsNotApplicableIsAFinding(test *testing.T) {
	registry := valid_registry()
	registry.Capabilities[0].Python = domain.LanguageState{State: "not_applicable", Reason: "Go only"}
	expect_one_finding(test, registry, domain.ErrTestForInapplicableLanguage, "tests/test_a.py", "REQ-AAA-001")
	registry = valid_registry()
	registry.Capabilities[0].Go = domain.LanguageState{State: "not_applicable", Reason: "Python only"}
	expect_one_finding(test, registry, domain.ErrTestForInapplicableLanguage, "tests/a_test.go", "REQ-AAA-001")
}

func TestARequirementListedByTwoCapabilitiesOrNeverIsAFinding(test *testing.T) {
	registry := valid_registry()
	registry.Capabilities[1].Requirements = append(registry.Capabilities[1].Requirements, domain.Requirement{ID: "REQ-AAA-001", Pending: "unassigned"})
	expect_one_finding(test, registry, domain.ErrDuplicateRequirement, `capability "beta_area"`, `capability "alpha"`)
	registry = valid_registry()
	registry.Capabilities[1].Requirements = []domain.Requirement{{ID: "REQ-CCC-009", Pending: "unassigned"}}
	report := domain.Check(registry, spec, existence)
	if len(report.Findings) != 2 || !errors.Is(report.Findings[0], domain.ErrUnknownRequirement) || !errors.Is(report.Findings[1], domain.ErrUnlistedRequirement) {
		test.Errorf("%v", report.Findings)
	}
}

func TestASupportedCapabilityNeedsBothLanguagesPassingOrOneNotApplicableWithAReason(test *testing.T) {
	supported := func() domain.Registry {
		registry := valid_registry()
		registry.Capabilities[0].Status = "supported"
		registry.Capabilities[0].Go.State = "passing"
		registry.Capabilities[0].Python.State = "passing"
		registry.Capabilities[0].Requirements[1] = domain.Requirement{ID: "REQ-AAA-002", Tests: passing_tests()}
		return registry
	}
	if report := domain.Check(supported(), spec, existence); !report.Passed() {
		test.Fatalf("a capability with both languages passing and every requirement tested is supported: %v", report.Findings)
	}
	built := supported()
	built.Capabilities[0].Python.State = "built"
	expect_one_finding(test, built, domain.ErrNotSupportable, `python is "built"`)
	both_todo := supported()
	both_todo.Capabilities[0].Go.State = "todo"
	both_todo.Capabilities[0].Python.State = "todo"
	if report := domain.Check(both_todo, spec, existence); len(report.Findings) != 3 {
		test.Errorf("both languages todo: want go, python and the no-language finding, got %v", report.Findings)
	}
	both_not_applicable := supported()
	both_not_applicable.Capabilities[0].Go = domain.LanguageState{State: "not_applicable", Reason: "a"}
	both_not_applicable.Capabilities[0].Python = domain.LanguageState{State: "not_applicable", Reason: "b"}
	both_not_applicable.Capabilities[0].Requirements = []domain.Requirement{{ID: "REQ-AAA-001", Pending: "T-01.1"}, {ID: "REQ-AAA-002", Pending: "T-02"}}
	report := domain.Check(both_not_applicable, spec, existence)
	found_no_language := false
	for _, finding := range report.Findings {
		found_no_language = found_no_language || strings.Contains(finding.Error(), "no language is passing")
	}
	if !found_no_language {
		test.Errorf("supported with nothing passing must fail: %v", report.Findings)
	}
	pending := supported()
	pending.Capabilities[0].Requirements[1] = domain.Requirement{ID: "REQ-AAA-002", Pending: "T-01.1"}
	expect_one_finding(test, pending, domain.ErrNotSupportable, "REQ-AAA-002 has no test that exists")
	one_not_applicable := supported()
	one_not_applicable.Capabilities[0].Python = domain.LanguageState{State: "not_applicable", Reason: "Go only"}
	one_not_applicable.Capabilities[0].Requirements = []domain.Requirement{
		{ID: "REQ-AAA-001", Tests: []domain.TestRef{{Language: "go", Path: "tests/a_test.go"}}},
		{ID: "REQ-AAA-002", Tests: []domain.TestRef{{Language: "go", Path: "tests/a_test.go"}}},
	}
	if report := domain.Check(one_not_applicable, spec, existence); !report.Passed() {
		test.Errorf("%v", report.Findings)
	}
}

func TestTheMaturityIsThatOfTheLeastCompleteCapability(test *testing.T) {
	registry := valid_registry()
	report := domain.Check(registry, spec, existence)
	if report.Maturity != (domain.Maturity{Level: "beta", Capability: "beta_area", Status: "planned"}) {
		test.Errorf("a planned capability is less complete than one in progress: %+v", report.Maturity)
	}
	registry.Capabilities[1].Status = "in_progress"
	registry.Capabilities[1].Requirements = []domain.Requirement{{ID: "REQ-BB-001", Tests: []domain.TestRef{{Language: "go", Path: "tests/a_test.go"}}}}
	if report := domain.Check(registry, spec, existence); report.Maturity.Capability != "alpha" {
		test.Errorf("with the same status, the one with the smaller share of covered requirements: %+v", report.Maturity)
	}
	for position := range registry.Capabilities {
		registry.Capabilities[position].Level = "stable"
		registry.Capabilities[position].Status = "supported"
	}
	if report := domain.Check(registry, spec, existence); report.Maturity.Level != "stable" {
		test.Errorf("every capability stable: %+v", report.Maturity)
	}
	registry.Capabilities[1].Level = "beta"
	if report := domain.Check(registry, spec, existence); report.Maturity.Level != "beta" || report.Maturity.Capability != "beta_area" {
		test.Errorf("one beta capability makes the library beta: %+v", report.Maturity)
	}
	if empty := domain.Check(domain.Registry{Path: "registry.yaml", Version: 1}, spec, existence); empty.Maturity != (domain.Maturity{Level: "beta"}) {
		test.Errorf("no capability: %+v", empty.Maturity)
	}
}

func TestTheListsAreSortedByAreaAndIdentifier(test *testing.T) {
	registry := valid_registry()
	registry.Capabilities[1].Requirements = []domain.Requirement{{ID: "REQ-BB-001", Pending: "unassigned"}}
	registry.Capabilities[0].Requirements = []domain.Requirement{{ID: "REQ-AAA-002", Pending: "T-02"}, {ID: "REQ-AAA-001", Pending: "T-01.1"}}
	report := domain.Check(registry, spec, existence)
	if len(report.Pending) != 2 || report.Pending[0].ID != "REQ-AAA-001" || report.Pending[1].ID != "REQ-AAA-002" {
		test.Errorf("%v", report.Pending)
	}
	registry.Capabilities[0].Requirements = []domain.Requirement{{ID: "REQ-BB-001", Pending: "T-02"}, {ID: "REQ-AAA-001", Pending: "T-01.1"}}
	registry.Capabilities[1].Requirements = []domain.Requirement{{ID: "REQ-AAA-002", Pending: "T-02"}}
	report = domain.Check(registry, spec, existence)
	if report.Pending[0].Area != "AAA" || report.Pending[2].Area != "BB" {
		test.Errorf("area order: %v", report.Pending)
	}
}

func TestTheListedTestPathsAndPendingTasksAreSortedAndWithoutRepeats(test *testing.T) {
	registry := valid_registry()
	registry.Capabilities[1].Requirements = []domain.Requirement{
		{ID: "REQ-BB-001", Tests: []domain.TestRef{{Language: "go", Path: "tests/a_test.go"}}},
		{ID: "REQ-AAA-002", Pending: "T-01.1"},
	}
	paths := registry.TestPaths()
	if strings.Join(paths, ",") != "tests/a_test.go,tests/test_a.py" {
		test.Errorf("%v", paths)
	}
	if tasks := registry.PendingTasks(); strings.Join(tasks, ",") != "T-01.1" {
		test.Errorf("unassigned is not a task: %v", tasks)
	}
}
