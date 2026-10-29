package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/inbound/cli"
)

type fake_executor struct {
	report        domain.Report
	execute_error error
}

func (executor fake_executor) Execute() (domain.Report, error) {
	return executor.report, executor.execute_error
}

func factory_of(executor fake_executor, received *[]string) cli.Factory {
	return func(spec_path string, registry_path string, root_dir string, tasks_dir string) cli.Executor {
		*received = []string{spec_path, registry_path, root_dir, tasks_dir}
		return executor
	}
}

func a_report() domain.Report {
	return domain.Report{
		RegistryPath: "registry.yaml", SpecPath: "spec.md", Total: 4, Covered: 1,
		Areas:        []domain.AreaCount{{Area: "ARC", Count: 3}, {Area: "LOG", Count: 1}},
		Pending:      []domain.Waiting{{ID: "REQ-ARC-005", Area: "ARC", Pending: "T-03.9"}, {ID: "REQ-ARC-006", Area: "ARC", Pending: "T-03.9"}},
		Unassigned:   []domain.Waiting{{ID: "REQ-LOG-004", Area: "LOG", Pending: "unassigned"}},
		Capabilities: []domain.CapabilitySummary{{ID: "logging", Level: "beta", Status: "planned", Go: "todo", Python: "todo", Requirements: 1}},
		Maturity:     domain.Maturity{Level: "beta", Capability: "logging", Status: "planned"},
	}
}

func TestTheDefaultsAndTheFlagsReachTheUseCase(test *testing.T) {
	var received []string
	var output, messages bytes.Buffer
	if code := cli.Run(nil, &output, &messages, factory_of(fake_executor{report: a_report()}, &received)); code != cli.ExitOK {
		test.Fatalf("code %d, %s", code, messages.String())
	}
	if strings.Join(received, " ") != "docs/proposals/PR-001-telemetry-standard-go-python.md contracts/capabilities/registry.yaml . docs/tasks" {
		test.Errorf("defaults %v", received)
	}
	arguments := []string{"--spec", "s", "--registry", "r", "--root", "o", "--tasks", "t"}
	cli.Run(arguments, &output, &messages, factory_of(fake_executor{report: a_report()}, &received))
	if strings.Join(received, " ") != "s r o t" {
		test.Errorf("flags %v", received)
	}
}

func TestThePrintedReportHasTheSectionsAndTheLines(test *testing.T) {
	var received []string
	var output, messages bytes.Buffer
	cli.Run(nil, &output, &messages, factory_of(fake_executor{report: a_report()}, &received))
	want := []string{
		"specification: spec.md: 4 requirement identifiers in 2 areas",
		"  ARC 3, LOG 1",
		"requirements: 4 total, 1 covered by tests, 2 pending, 1 no task yet, 0 not accounted for",
		"  area ARC:\n    REQ-ARC-005 (ARC): pending T-03.9\n    REQ-ARC-006 (ARC): pending T-03.9\n",
		"no task yet:\n  area LOG:\n    REQ-LOG-004 (LOG): unassigned\n",
		"library maturity: beta (least complete capability: logging, planned)\n",
		"result: PASSED\n",
	}
	for _, text := range want {
		if !strings.Contains(output.String(), text) {
			test.Errorf("the output lacks %q:\n%s", text, output.String())
		}
	}
	if messages.Len() != 0 {
		test.Errorf("messages %q", messages.String())
	}
	var no_capability bytes.Buffer
	cli.Print(&no_capability, domain.Report{Maturity: domain.Maturity{Level: "beta"}})
	if !strings.Contains(no_capability.String(), "library maturity: beta (no capability is listed)") {
		test.Errorf("%s", no_capability.String())
	}
}

func TestABrokenRuleGivesExitOneAndNamesTheEntryOnTheErrorWriter(test *testing.T) {
	var received []string
	report := a_report()
	report.Findings = []error{errors.New(`registry.yaml: capability "logging": requirement REQ-LOG-001: write tests or pending`)}
	var output, messages bytes.Buffer
	if code := cli.Run(nil, &output, &messages, factory_of(fake_executor{report: report}, &received)); code != cli.ExitFailure {
		test.Errorf("code %d", code)
	}
	if !strings.Contains(output.String(), "result: FAILED (1 errors, listed below)") || !strings.Contains(messages.String(), `capability "logging": requirement REQ-LOG-001`) || !strings.Contains(messages.String(), "FAILED with 1 error(s)") {
		test.Errorf("output %q, messages %q", output.String(), messages.String())
	}
}

func TestAFailureToRunGivesExitOneAndTheMessageAndNoReport(test *testing.T) {
	var received []string
	var output, messages bytes.Buffer
	code := cli.Run(nil, &output, &messages, factory_of(fake_executor{execute_error: errors.New("registry.yaml: invalid YAML")}, &received))
	if code != cli.ExitFailure || output.Len() != 0 || !strings.Contains(messages.String(), "registry.yaml: invalid YAML") {
		test.Errorf("code %d, output %q, messages %q", code, output.String(), messages.String())
	}
}

func TestUnknownFlagsAndStrayArgumentsAreAUsageError(test *testing.T) {
	for _, arguments := range [][]string{{"--unknown"}, {"stray"}, {"--spec"}} {
		var received []string
		var output, messages bytes.Buffer
		if code := cli.Run(arguments, &output, &messages, factory_of(fake_executor{}, &received)); code != cli.ExitUsage {
			test.Errorf("%v: code %d", arguments, code)
		}
		if received != nil || messages.Len() == 0 {
			test.Errorf("%v: the use case must not run and a message is expected", arguments)
		}
	}
}
