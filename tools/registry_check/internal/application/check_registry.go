package application

import (
	"fmt"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
)

// CheckRegistry is the use case of the check: read the registry and the specification, find out which test files and
// tasks exist and apply the rules of the domain.
type CheckRegistry struct {
	registry   RegistryReader
	spec       SpecReader
	test_files TestFiles
	tasks      Tasks
}

// NewCheckRegistry builds the use case from its ports.
func NewCheckRegistry(registry RegistryReader, spec SpecReader, test_files TestFiles, tasks Tasks) CheckRegistry {
	return CheckRegistry{registry: registry, spec: spec, test_files: test_files, tasks: tasks}
}

// Execute runs the check. The error is for what stops the check from running (a file that cannot be read); a rule
// that is broken is in the findings of the report, so the report lists everything at once.
func (use_case CheckRegistry) Execute() (domain.Report, error) {
	registry, registry_error := use_case.registry.Read()
	if registry_error != nil {
		return domain.Report{}, registry_error
	}
	spec, spec_error := use_case.spec.Read()
	if spec_error != nil {
		return domain.Report{}, spec_error
	}
	if len(spec.IDs) == 0 {
		return domain.Report{}, fmt.Errorf("%s: %w", spec.Path, ErrNoRequirementsInSpec)
	}
	existence := domain.Existence{Files: map[string]bool{}, Tasks: map[string]bool{}}
	for _, test_path := range registry.TestPaths() {
		exists, find_error := use_case.test_files.Exists(test_path)
		if find_error != nil {
			return domain.Report{}, fmt.Errorf("%w: %s: %w", ErrCheckTestFile, test_path, find_error)
		}
		existence.Files[test_path] = exists
	}
	for _, task_id := range registry.PendingTasks() {
		exists, find_error := use_case.tasks.Exists(task_id)
		if find_error != nil {
			return domain.Report{}, find_error
		}
		existence.Tasks[task_id] = exists
	}
	return domain.Check(registry, spec, existence), nil
}
