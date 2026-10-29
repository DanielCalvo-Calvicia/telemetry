package domain

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	capability_id_pattern = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	task_id_pattern       = regexp.MustCompile(`^T-[0-9]{2}(\.[0-9]+)?$`)

	level_ranks  = map[string]int{LevelBeta: 0, LevelStable: 1}
	status_ranks = map[string]int{StatusPlanned: 0, StatusInProgress: 1, StatusSupported: 2}
	states       = map[string]bool{StateTodo: true, StateBuilt: true, StatePassing: true, StateNotApplicable: true}
)

// checker holds the registry, the facts and the findings while the rules run.
type checker struct {
	registry  Registry
	spec_ids  map[string]bool
	existence Existence
	findings  []error
	listed    map[string]string
	covered   map[string]bool
	pending   map[string]string
}

// Check applies every rule of the registry and builds the report. It never stops at the first breach: the report
// holds all of them. The rules are the ones of the task T-02.7: the format of every field, the evidence of every
// requirement (tests that exist or a pending task that exists), that every requirement of the specification is
// listed once, and that a supported capability passes in both languages (REQ-CNF-007).
func Check(registry Registry, spec Spec, existence Existence) Report {
	rules := &checker{
		registry:  registry,
		spec_ids:  map[string]bool{},
		existence: existence,
		listed:    map[string]string{},
		covered:   map[string]bool{},
		pending:   map[string]string{},
	}
	for _, id := range spec.IDs {
		rules.spec_ids[id] = true
	}
	if registry.Version != SupportedVersion {
		rules.add(ErrUnsupportedVersion, "registry_version is %d, the check understands %d", registry.Version, SupportedVersion)
	}
	seen_capabilities := map[string]bool{}
	summaries := make([]CapabilitySummary, 0, len(registry.Capabilities))
	for _, capability := range registry.Capabilities {
		rules.check_capability_header(capability, seen_capabilities)
		covered := rules.check_requirements(capability)
		rules.check_supported(capability, covered)
		summaries = append(summaries, summary_of(capability, len(covered)))
	}
	for _, id := range spec.IDs {
		if _, is_listed := rules.listed[id]; !is_listed {
			rules.add(ErrUnlistedRequirement, "%s (%s) is defined in %s", id, area_of(id), spec.Path)
		}
	}
	return rules.report(spec, summaries)
}

// add records a finding: the registry file, the message and the sentinel, joined so errors.Is finds the sentinel.
func (rules *checker) add(sentinel error, format string, arguments ...any) {
	rules.findings = append(rules.findings, fmt.Errorf("%s: %s: %w", rules.registry.Path, fmt.Sprintf(format, arguments...), sentinel))
}

func (rules *checker) check_capability_header(capability Capability, seen map[string]bool) {
	where := fmt.Sprintf("capability %q", capability.ID)
	switch {
	case !capability_id_pattern.MatchString(capability.ID):
		rules.add(ErrInvalidCapabilityID, "%s: the id must be snake_case", where)
	case seen[capability.ID]:
		rules.add(ErrDuplicateCapability, "%s", where)
	}
	seen[capability.ID] = true
	if strings.TrimSpace(capability.Title) == "" {
		rules.add(ErrMissingField, "%s: title", where)
	}
	if strings.TrimSpace(capability.Description) == "" {
		rules.add(ErrMissingField, "%s: description", where)
	}
	if _, valid := level_ranks[capability.Level]; !valid {
		rules.add(ErrInvalidLevel, "%s: level %q is not stable or beta", where, capability.Level)
	}
	if _, valid := status_ranks[capability.Status]; !valid {
		rules.add(ErrInvalidStatus, "%s: status %q is not supported, in_progress or planned", where, capability.Status)
	}
	rules.check_language(where, "go", capability.Go)
	rules.check_language(where, "python", capability.Python)
	if len(capability.Requirements) == 0 {
		rules.add(ErrNoRequirements, "%s", where)
	}
}

func (rules *checker) check_language(where string, language string, language_state LanguageState) {
	if !states[language_state.State] {
		rules.add(ErrInvalidState, "%s: languages.%s.state %q is not todo, built, passing or not_applicable", where, language, language_state.State)
		return
	}
	if language_state.State == StateNotApplicable && strings.TrimSpace(language_state.Reason) == "" {
		rules.add(ErrMissingReason, "%s: languages.%s", where, language)
	}
}

// check_requirements checks every requirement of a capability and returns the ones that tests cover.
func (rules *checker) check_requirements(capability Capability) map[string]bool {
	covered := map[string]bool{}
	for _, requirement := range capability.Requirements {
		where := fmt.Sprintf("capability %q: requirement %s", capability.ID, requirement.ID)
		if !rules.spec_ids[requirement.ID] {
			rules.add(ErrUnknownRequirement, "%s: not defined as a bold paragraph start in the specification", where)
			continue
		}
		if owner, is_listed := rules.listed[requirement.ID]; is_listed {
			rules.add(ErrDuplicateRequirement, "%s: already listed by capability %q (a requirement belongs to one capability)", where, owner)
			continue
		}
		rules.listed[requirement.ID] = capability.ID
		switch {
		case len(requirement.Tests) == 0 && requirement.Pending == "":
			rules.add(ErrNoEvidence, "%s: write tests or pending", where)
		case len(requirement.Tests) > 0 && requirement.Pending != "":
			rules.add(ErrTwoEvidences, "%s", where)
		case requirement.Pending != "":
			rules.check_pending(where, requirement)
		default:
			if rules.check_tests(where, capability, requirement) {
				covered[requirement.ID] = true
				rules.covered[requirement.ID] = true
			}
		}
	}
	return covered
}

func (rules *checker) check_pending(where string, requirement Requirement) {
	switch {
	case requirement.Pending == PendingUnassigned:
		rules.pending[requirement.ID] = PendingUnassigned
	case !task_id_pattern.MatchString(requirement.Pending):
		rules.add(ErrInvalidPending, "%s: pending %q is neither a task such as T-04.7 nor %q", where, requirement.Pending, PendingUnassigned)
	case !rules.existence.Tasks[requirement.Pending]:
		rules.pending[requirement.ID] = requirement.Pending
		rules.add(ErrTaskMissing, "%s: pending task %s has no file in the tasks folder", where, requirement.Pending)
	default:
		rules.pending[requirement.ID] = requirement.Pending
	}
}

// check_tests checks every test of a requirement and reports whether all of them are valid and exist.
func (rules *checker) check_tests(where string, capability Capability, requirement Requirement) bool {
	valid := true
	for _, test := range requirement.Tests {
		switch {
		case test.Language != LanguageGo && test.Language != LanguagePython:
			rules.add(ErrInvalidTest, "%s: test %q has language %q, it must be go or python", where, test.Path, test.Language)
			valid = false
		case !valid_test_path(test.Path):
			rules.add(ErrInvalidTest, "%s: test path %q must be a clean slash path relative to the root of the repository", where, test.Path)
			valid = false
		case is_not_applicable(capability, test.Language):
			rules.add(ErrTestForInapplicableLanguage, "%s: test %s is %s and the capability marks %s not_applicable", where, test.Path, test.Language, test.Language)
			valid = false
		case !rules.existence.Files[test.Path]:
			rules.add(ErrTestFileMissing, "%s: %s", where, test.Path)
			valid = false
		}
	}
	return valid
}

// check_supported applies REQ-CNF-007: supported only when both languages pass (or one is not applicable, with a
// reason, and the other passes) and every requirement is covered by a test that exists.
func (rules *checker) check_supported(capability Capability, covered map[string]bool) {
	if capability.Status != StatusSupported {
		return
	}
	where := fmt.Sprintf("capability %q", capability.ID)
	passing := 0
	for _, language := range []struct {
		name  string
		state LanguageState
	}{{"go", capability.Go}, {"python", capability.Python}} {
		switch {
		case language.state.State == StatePassing:
			passing++
		case language.state.State == StateNotApplicable && strings.TrimSpace(language.state.Reason) != "":
		default:
			rules.add(ErrNotSupportable, "%s: status is supported but %s is %q", where, language.name, language.state.State)
		}
	}
	if passing == 0 {
		rules.add(ErrNotSupportable, "%s: status is supported but no language is passing", where)
	}
	for _, requirement := range capability.Requirements {
		if !covered[requirement.ID] {
			rules.add(ErrNotSupportable, "%s: status is supported but %s has no test that exists", where, requirement.ID)
		}
	}
}

func is_not_applicable(capability Capability, language string) bool {
	if language == LanguageGo {
		return capability.Go.State == StateNotApplicable
	}
	return capability.Python.State == StateNotApplicable
}

// valid_test_path accepts a clean, relative slash path that stays inside the repository and names a file.
func valid_test_path(test_path string) bool {
	if test_path == "" || strings.ContainsAny(test_path, `\:`) || strings.HasPrefix(test_path, "/") {
		return false
	}
	return path.Clean(test_path) == test_path && test_path != "." && !strings.HasPrefix(test_path, "../")
}

func summary_of(capability Capability, covered int) CapabilitySummary {
	return CapabilitySummary{
		ID: capability.ID, Level: capability.Level, Status: capability.Status,
		Go: capability.Go.State, Python: capability.Python.State,
		Requirements: len(capability.Requirements), Covered: covered,
	}
}

// report builds the counts and the sorted lists from what the rules found.
func (rules *checker) report(spec Spec, summaries []CapabilitySummary) Report {
	report := Report{
		RegistryPath: rules.registry.Path,
		SpecPath:     spec.Path,
		Total:        len(spec.IDs),
		Areas:        count_areas(spec.IDs),
		Capabilities: summaries,
		Maturity:     least_complete(summaries),
		Findings:     rules.findings,
	}
	for _, id := range spec.IDs {
		waiting := Waiting{ID: id, Area: area_of(id), Pending: rules.pending[id]}
		switch {
		case rules.covered[id]:
			report.Covered++
		case waiting.Pending == PendingUnassigned:
			report.Unassigned = append(report.Unassigned, waiting)
		case waiting.Pending != "":
			report.Pending = append(report.Pending, waiting)
		default:
			report.Unaccounted++
		}
	}
	sort_waiting(report.Pending)
	sort_waiting(report.Unassigned)
	return report
}

func sort_waiting(waiting []Waiting) {
	sort.Slice(waiting, func(left int, right int) bool {
		if waiting[left].Area != waiting[right].Area {
			return waiting[left].Area < waiting[right].Area
		}
		return waiting[left].ID < waiting[right].ID
	})
}

// area_of returns the area code of a requirement identifier: ARC for REQ-ARC-014.
func area_of(id string) string {
	segments := strings.Split(id, "-")
	if len(segments) != 3 {
		return id
	}
	return segments[1]
}

func count_areas(ids []string) []AreaCount {
	counts := map[string]int{}
	for _, id := range ids {
		counts[area_of(id)]++
	}
	areas := make([]AreaCount, 0, len(counts))
	for _, area := range sorted_keys_of_counts(counts) {
		areas = append(areas, AreaCount{Area: area, Count: counts[area]})
	}
	return areas
}

// least_complete finds the capability that sets the maturity of the library (REQ-CNF-008): the lowest level, then the
// lowest status, then the smallest share of requirements covered by tests, then the first in the file. An invalid
// level or status ranks below every valid one, so a broken entry is never hidden.
func least_complete(summaries []CapabilitySummary) Maturity {
	if len(summaries) == 0 {
		return Maturity{Level: LevelBeta}
	}
	least := summaries[0]
	for _, candidate := range summaries[1:] {
		if is_less_complete(candidate, least) {
			least = candidate
		}
	}
	level := least.Level
	if _, valid := level_ranks[level]; !valid {
		level = LevelBeta
	}
	return Maturity{Level: level, Capability: least.ID, Status: least.Status}
}

func is_less_complete(candidate CapabilitySummary, current CapabilitySummary) bool {
	candidate_level, current_level := rank(level_ranks, candidate.Level), rank(level_ranks, current.Level)
	if candidate_level != current_level {
		return candidate_level < current_level
	}
	candidate_status, current_status := rank(status_ranks, candidate.Status), rank(status_ranks, current.Status)
	if candidate_status != current_status {
		return candidate_status < current_status
	}
	return candidate.Covered*current.Requirements < current.Covered*candidate.Requirements
}

func rank(ranks map[string]int, value string) int {
	if position, valid := ranks[value]; valid {
		return position
	}
	return -1
}

func sorted_keys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sorted_keys_of_counts(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
