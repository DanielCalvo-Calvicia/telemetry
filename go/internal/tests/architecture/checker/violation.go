// Package checker reads the import lists of a Go module laid out in the layers of the standard and reports the
// imports that break the direction of the dependencies (REQ-ARC-001, REQ-ARC-002, REQ-ARC-003).
package checker

import "fmt"

// The rules the checker applies. Each violation carries the id of the rule it breaks.
const (
	// RuleDomainIsolated: the domain imports nothing from the other layers and no I/O, database, telemetry or AI package.
	RuleDomainIsolated = "domain-isolated"
	// RuleApplicationImportsOnlyDomain: the application imports the standard library, the domain and itself.
	RuleApplicationImportsOnlyDomain = "application-imports-only-domain"
	// RuleInfrastructureNotCompositionRoot: the infrastructure never imports the composition root.
	RuleInfrastructureNotCompositionRoot = "infrastructure-not-composition-root"
	// RuleAdaptersWiredByCompositionRoot: only a composition root imports a concrete outbound adapter.
	RuleAdaptersWiredByCompositionRoot = "adapters-wired-only-by-the-composition-root"
	// RuleIOOnlyInExternal: the I/O packages are imported from the external/ folder of an adapter only.
	RuleIOOnlyInExternal = "io-only-in-external"
	// RuleConsumerNeverImportsInternal: a consumer of the library never imports an internal/ package.
	RuleConsumerNeverImportsInternal = "consumer-never-imports-internal"
)

// Violation is one import that breaks a rule.
type Violation struct {
	// Rule is the id of the rule that is broken.
	Rule string
	// File is the path of the offending file, relative to the root that was checked, with forward slashes.
	File string
	// Import is the forbidden import path.
	Import string
}

// Message names the rule, the offending file and the forbidden import.
func (violation Violation) Message() string {
	return fmt.Sprintf("%s: %s imports %q", violation.Rule, violation.File, violation.Import)
}
