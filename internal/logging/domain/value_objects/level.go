package value_objects

import (
	"slices"

	domain "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
)

// =============================================================================
// Level
// =============================================================================

// Level is the severity of a log entry.
type Level string

// -----------------------------------------------------------------------------
// ERRORS
// -----------------------------------------------------------------------------
var (
	ErrInvalidLevel       = domain.ErrInvalidLevel
	ErrInvalidEnvironment = domain.ErrInvalidEnvironment
)

// -----------------------------------------------------------------------------
// CONSTANTS
// -----------------------------------------------------------------------------
const (
	LevelTrace = Level(domain.LevelTrace)
	LevelDebug = Level(domain.LevelDebug)
	LevelInfo  = Level(domain.LevelInfo)
	LevelWarn  = Level(domain.LevelWarn)
	LevelError = Level(domain.LevelError)
	LevelFatal = Level(domain.LevelFatal)
)

// -----------------------------------------------------------------------------
// CONSTRUCTORS
// -----------------------------------------------------------------------------

// NewLevel creates a Level from its upper-case name and rejects an unknown level.
func NewLevel(raw_level string) (Level, error) {
	if err := ValidateLevel(raw_level); err != nil {
		return "", err
	}
	return Level(raw_level), nil
}

// MinimumLevelFor returns the lowest level that is logged in an environment (the value of APP_ENV) and
// rejects an environment other than development or production.
func MinimumLevelFor(environment string) (Level, error) {
	minimum_level, found := domain.MinimumLevelByEnvironment[environment]
	if !found {
		return "", ErrInvalidEnvironment
	}
	return Level(minimum_level), nil
}

// -----------------------------------------------------------------------------
// METHODS
// -----------------------------------------------------------------------------

// String returns the level name.
func (level Level) String() string {
	return string(level)
}

// IsValid reports whether the level is one of the known levels.
func (level Level) IsValid() bool {
	return ValidateLevel(string(level)) == nil
}

// IsAtLeast reports whether the level is as severe as the minimum or more; an unknown level is never at least anything.
func (level Level) IsAtLeast(minimum_level Level) bool {
	level_position := slices.Index(domain.ValidLevels, string(level))
	minimum_position := slices.Index(domain.ValidLevels, string(minimum_level))
	return level_position >= 0 && minimum_position >= 0 && level_position >= minimum_position
}

// -----------------------------------------------------------------------------
// VALIDATORS
// -----------------------------------------------------------------------------

// ValidateLevel rejects any text that is not one of TRACE, DEBUG, INFO, WARN, ERROR or FATAL.
func ValidateLevel(raw_level string) error {
	if !slices.Contains(domain.ValidLevels, raw_level) {
		return ErrInvalidLevel
	}
	return nil
}
