// Package domain holds the model of the registry of capabilities and the rules the check applies to it. It imports
// nothing from the other layers and does no I/O: what exists on disk is given to it as facts.
package domain

// The values a field of the registry may take.
const (
	// LevelStable marks a capability whose contract and behavior are frozen for the 1.x line.
	LevelStable = "stable"
	// LevelBeta marks a capability that may still change.
	LevelBeta = "beta"

	// StatusSupported marks a capability whose conformance tests pass in both languages (REQ-CNF-007).
	StatusSupported = "supported"
	// StatusInProgress marks a capability that is being built.
	StatusInProgress = "in_progress"
	// StatusPlanned marks a capability that is not started.
	StatusPlanned = "planned"

	// StateTodo is a language that has not started the capability.
	StateTodo = "todo"
	// StateBuilt is a language that implements the capability but whose conformance tests do not all pass yet.
	StateBuilt = "built"
	// StatePassing is a language whose conformance tests of the capability all pass.
	StatePassing = "passing"
	// StateNotApplicable is a language that will never have the capability; it needs a written reason.
	StateNotApplicable = "not_applicable"

	// LanguageGo is the Go implementation, in the entries of a test.
	LanguageGo = "go"
	// LanguagePython is the Python implementation, in the entries of a test.
	LanguagePython = "python"

	// PendingUnassigned is the value of "pending" of a requirement that no task mentions yet: a gap of the plan.
	PendingUnassigned = "unassigned"

	// SupportedVersion is the only version of the format of the registry the check understands.
	SupportedVersion = 1
)

// TestRef is one conformance test of a requirement: a test file of one language.
type TestRef struct {
	// Language is "go" or "python".
	Language string
	// Path is the slash path of the test file, relative to the root of the repository.
	Path string
}

// Requirement is one requirement of the specification that a capability lists, with its evidence: the tests that
// cover it or the task that will write them.
type Requirement struct {
	// ID is the identifier in the specification, such as REQ-ARC-001.
	ID string
	// Tests are the conformance tests that exist.
	Tests []TestRef
	// Pending is the task that will write the test (T-04.7) or "unassigned" when no task mentions the requirement.
	Pending string
}

// LanguageState is the state of one language in a capability.
type LanguageState struct {
	// State is one of todo, built, passing and not_applicable.
	State string
	// Reason is why the language does not apply; it is required for not_applicable.
	Reason string
}

// Capability is one entry of the registry.
type Capability struct {
	// ID is the stable snake_case identifier.
	ID string
	// Title is a short name for people.
	Title string
	// Level is stable or beta.
	Level string
	// Status is supported, in_progress or planned.
	Status string
	// Description says what the capability is for.
	Description string
	// PublicAPI lists the public API of the capability, as built or as planned.
	PublicAPI []string
	// Schemas lists the schemas of the capability under contracts/.
	Schemas []string
	// Configuration lists the configuration options of the capability.
	Configuration []string
	// Errors lists the error semantics (codes and sentinels) of the capability.
	Errors []string
	// Ports lists the ports of the capability.
	Ports []string
	// Adapters lists the adapters of the capability.
	Adapters []string
	// Go is the state of the Go implementation.
	Go LanguageState
	// Python is the state of the Python implementation.
	Python LanguageState
	// Requirements are the requirements of the specification that the capability owns.
	Requirements []Requirement
}

// Registry is the registry of capabilities read from one file.
type Registry struct {
	// Path is the path of the file, used to name it in every message.
	Path string
	// Version is registry_version.
	Version int
	// Capabilities are the entries, in the order of the file.
	Capabilities []Capability
}

// Spec is the list of requirement identifiers defined by the specification.
type Spec struct {
	// Path is the path of the specification file, used to name it in every message.
	Path string
	// IDs are the identifiers, without repeats, sorted.
	IDs []string
}

// Existence is what exists on disk, found by the application before the rules run.
type Existence struct {
	// Files maps a test path to whether the file exists.
	Files map[string]bool
	// Tasks maps a task identifier to whether its file exists in the tasks folder.
	Tasks map[string]bool
}

// TestPaths returns the paths of every test the registry lists, without repeats, sorted.
func (registry Registry) TestPaths() []string {
	seen := map[string]bool{}
	for _, capability := range registry.Capabilities {
		for _, requirement := range capability.Requirements {
			for _, test := range requirement.Tests {
				seen[test.Path] = true
			}
		}
	}
	return sorted_keys(seen)
}

// PendingTasks returns the identifiers of the tasks the registry points to, without repeats, sorted. "unassigned" is
// not a task and is not returned.
func (registry Registry) PendingTasks() []string {
	seen := map[string]bool{}
	for _, capability := range registry.Capabilities {
		for _, requirement := range capability.Requirements {
			if requirement.Pending != "" && requirement.Pending != PendingUnassigned {
				seen[requirement.Pending] = true
			}
		}
	}
	return sorted_keys(seen)
}
