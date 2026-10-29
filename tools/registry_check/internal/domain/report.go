package domain

// Waiting is a requirement of the specification that no test covers yet.
type Waiting struct {
	// ID is the requirement identifier.
	ID string
	// Area is the three letters between REQ- and the number: ARC in REQ-ARC-014.
	Area string
	// Pending is the task that will write the test, or "unassigned".
	Pending string
}

// AreaCount is the number of requirements the specification defines in one area.
type AreaCount struct {
	// Area is the area code.
	Area string
	// Count is the number of requirements of the area.
	Count int
}

// CapabilitySummary is one line of the table of capabilities of the report.
type CapabilitySummary struct {
	// ID is the capability identifier.
	ID string
	// Level is the level as written in the registry.
	Level string
	// Status is the status as written in the registry.
	Status string
	// Go is the state of the Go implementation.
	Go string
	// Python is the state of the Python implementation.
	Python string
	// Requirements is the number of requirements the capability lists.
	Requirements int
	// Covered is how many of them are covered by tests that exist.
	Covered int
}

// Maturity is the maturity of the library: the one of the least complete capability (REQ-CNF-008).
type Maturity struct {
	// Level is the level of the least complete capability: beta unless every capability is stable.
	Level string
	// Capability is the identifier of the least complete capability; empty when the registry has none.
	Capability string
	// Status is the status of that capability.
	Status string
}

// Report is the result of the check: the counts, what is not covered yet and every rule that is broken.
type Report struct {
	// RegistryPath is the registry file that was checked.
	RegistryPath string
	// SpecPath is the specification the identifiers were read from.
	SpecPath string
	// Total is the number of requirement identifiers the specification defines.
	Total int
	// Areas is the number of identifiers per area, sorted by area.
	Areas []AreaCount
	// Covered is the number of requirements with tests that exist.
	Covered int
	// Pending are the requirements that wait for a task, sorted by area and identifier.
	Pending []Waiting
	// Unassigned are the requirements no task mentions, sorted by area and identifier.
	Unassigned []Waiting
	// Unaccounted is the number of requirements that are neither covered, pending nor unassigned: unlisted, or listed
	// without evidence or with a test that does not exist.
	Unaccounted int
	// Capabilities is the table of capabilities, in the order of the registry.
	Capabilities []CapabilitySummary
	// Maturity is the maturity of the library.
	Maturity Maturity
	// Findings are the broken rules, each one naming the registry file and the entry.
	Findings []error
}

// Passed reports whether no rule is broken. Pending and unassigned requirements do not break a rule: they are listed.
func (report Report) Passed() bool {
	return len(report.Findings) == 0
}
