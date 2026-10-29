package application_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
)

type fake_registry struct {
	registry   domain.Registry
	read_error error
}

func (reader fake_registry) Read() (domain.Registry, error) {
	return reader.registry, reader.read_error
}

type fake_spec struct {
	spec       domain.Spec
	read_error error
}

func (reader fake_spec) Read() (domain.Spec, error) { return reader.spec, reader.read_error }

type fake_files struct {
	existing   map[string]bool
	find_error error
	asked      []string
}

func (files *fake_files) Exists(name string) (bool, error) {
	files.asked = append(files.asked, name)
	return files.existing[name], files.find_error
}

var error_of_the_disk = errors.New("the disk failed")

func registry_with_one_of_each() domain.Registry {
	return domain.Registry{Path: "registry.yaml", Version: 1, Capabilities: []domain.Capability{{
		ID: "alpha", Title: "A", Description: "A.", Level: "beta", Status: "in_progress",
		Go: domain.LanguageState{State: "built"}, Python: domain.LanguageState{State: "todo"},
		Requirements: []domain.Requirement{
			{ID: "REQ-AAA-001", Tests: []domain.TestRef{{Language: "go", Path: "tests/a_test.go"}}},
			{ID: "REQ-AAA-002", Pending: "T-01.1"},
		},
	}}}
}

var one_spec = fake_spec{spec: domain.Spec{Path: "spec.md", IDs: []string{"REQ-AAA-001", "REQ-AAA-002"}}}

func TestTheUseCaseAsksOnlyForTheFilesAndTasksTheRegistryLists(test *testing.T) {
	test_files := &fake_files{existing: map[string]bool{"tests/a_test.go": true}}
	tasks := &fake_files{existing: map[string]bool{"T-01.1": true}}
	report, execute_error := application.NewCheckRegistry(fake_registry{registry: registry_with_one_of_each()}, one_spec, test_files, tasks).Execute()
	if execute_error != nil || !report.Passed() || report.Covered != 1 || len(report.Pending) != 1 {
		test.Fatalf("%v %+v", execute_error, report)
	}
	if len(test_files.asked) != 1 || test_files.asked[0] != "tests/a_test.go" || len(tasks.asked) != 1 || tasks.asked[0] != "T-01.1" {
		test.Errorf("files %v, tasks %v", test_files.asked, tasks.asked)
	}
}

func TestWhatDoesNotExistIsAFindingNotAnError(test *testing.T) {
	report, execute_error := application.NewCheckRegistry(fake_registry{registry: registry_with_one_of_each()}, one_spec, &fake_files{}, &fake_files{}).Execute()
	if execute_error != nil || report.Passed() || len(report.Findings) != 2 {
		test.Errorf("%v %v", execute_error, report.Findings)
	}
}

func TestAFailureOfAPortStopsTheCheck(test *testing.T) {
	read_failure := errors.New("cannot read")
	registry := fake_registry{registry: registry_with_one_of_each()}
	cases := []struct {
		name     string
		use_case application.CheckRegistry
		want     error
	}{
		{"registry", application.NewCheckRegistry(fake_registry{read_error: read_failure}, one_spec, &fake_files{}, &fake_files{}), read_failure},
		{"spec", application.NewCheckRegistry(registry, fake_spec{read_error: read_failure}, &fake_files{}, &fake_files{}), read_failure},
		{"empty spec", application.NewCheckRegistry(registry, fake_spec{spec: domain.Spec{Path: "spec.md"}}, &fake_files{}, &fake_files{}), application.ErrNoRequirementsInSpec},
		{"test file", application.NewCheckRegistry(registry, one_spec, &fake_files{find_error: error_of_the_disk}, &fake_files{}), application.ErrCheckTestFile},
		{"task", application.NewCheckRegistry(registry, one_spec, &fake_files{existing: map[string]bool{"tests/a_test.go": true}}, &fake_files{find_error: error_of_the_disk}), error_of_the_disk},
	}
	for _, scenario := range cases {
		if _, execute_error := scenario.use_case.Execute(); !errors.Is(execute_error, scenario.want) {
			test.Errorf("%s: %v", scenario.name, execute_error)
		}
	}
}
