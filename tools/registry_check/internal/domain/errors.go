package domain

import "errors"

// The rules of the registry. Each error is wrapped with the registry file and the entry it is about.
var (
	// ErrUnsupportedVersion is returned for a registry_version the check does not understand.
	ErrUnsupportedVersion = errors.New("unsupported registry_version")
	// ErrMissingField is returned for a required text that is empty.
	ErrMissingField = errors.New("required field is empty")
	// ErrInvalidCapabilityID is returned for a capability identifier that is not snake_case.
	ErrInvalidCapabilityID = errors.New("invalid capability id")
	// ErrDuplicateCapability is returned for a capability identifier that appears twice.
	ErrDuplicateCapability = errors.New("duplicated capability id")
	// ErrInvalidLevel is returned for a level that is not stable or beta.
	ErrInvalidLevel = errors.New("invalid level")
	// ErrInvalidStatus is returned for a status that is not supported, in_progress or planned.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrInvalidState is returned for a language state that is not todo, built, passing or not_applicable.
	ErrInvalidState = errors.New("invalid language state")
	// ErrMissingReason is returned for a not_applicable language without a written reason.
	ErrMissingReason = errors.New("not_applicable needs a reason")
	// ErrNoRequirements is returned for a capability that lists no requirement.
	ErrNoRequirements = errors.New("capability without requirements")
	// ErrUnknownRequirement is returned for a listed identifier that the specification does not define.
	ErrUnknownRequirement = errors.New("requirement not defined in the specification")
	// ErrDuplicateRequirement is returned for a requirement listed twice, in one capability or in two.
	ErrDuplicateRequirement = errors.New("requirement listed more than once")
	// ErrUnlistedRequirement is returned for a requirement of the specification that no capability lists.
	ErrUnlistedRequirement = errors.New("requirement of the specification listed by no capability")
	// ErrNoEvidence is returned for a requirement with neither tests nor pending.
	ErrNoEvidence = errors.New("requirement without tests and without pending")
	// ErrTwoEvidences is returned for a requirement that has both tests and pending.
	ErrTwoEvidences = errors.New("requirement has both tests and pending: choose one")
	// ErrInvalidTest is returned for a test entry with a language or a path that is not valid.
	ErrInvalidTest = errors.New("invalid test entry")
	// ErrTestFileMissing is returned for a listed test file that does not exist.
	ErrTestFileMissing = errors.New("test file does not exist")
	// ErrTestForInapplicableLanguage is returned for a test of a language that the capability marks not_applicable.
	ErrTestForInapplicableLanguage = errors.New("test of a language that is not applicable to the capability")
	// ErrInvalidPending is returned for a pending value that is neither a task identifier nor "unassigned".
	ErrInvalidPending = errors.New("invalid pending value")
	// ErrTaskMissing is returned for a pending task that does not exist in the tasks folder.
	ErrTaskMissing = errors.New("pending task does not exist")
	// ErrNotSupportable is returned for a capability marked supported while a language does not pass or a requirement
	// has no test (REQ-CNF-007).
	ErrNotSupportable = errors.New("capability marked supported without the evidence")
)
