package console

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
)

// =============================================================================
// SETTINGS
// =============================================================================

// Settings are the settings of the console destination, the same model as contracts/config/console.schema.json (the
// json tags are the names of the schema, the env tags are its x-env names). The library never reads the environment or a
// file (ADR-004): the consumer loads the values and passes them in. The env tags carry no default on purpose; the
// defaults live in DefaultSettings.
type Settings struct {
	// Format is json (the canonical JSON of the envelope, one per line) or text (a readable line).
	Format string `json:"format" env:"TELEMETRY_CONSOLE_FORMAT"`
	// MinimumSeverity is the lowest severity that is written, in upper case: TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL.
	MinimumSeverity string `json:"minimum_severity" env:"TELEMETRY_CONSOLE_MINIMUM_SEVERITY"`
}

// DefaultSettings returns the settings with their defaults: the json format and a minimum severity of INFO.
func DefaultSettings() Settings {
	return Settings{
		Format:          domain.DefaultConsoleFormat,
		MinimumSeverity: domain.DefaultConsoleMinimumSeverity,
	}
}

// ValidateSettings checks the settings. It returns nil when they are valid, or a *config.ValidationError with every
// problem together, in the order of the properties of the schema. Each problem names the field and gives a fixed
// reason; no reason contains a configured value. This is the reference of the shared cases of
// integration-tests/fixtures/console.
func ValidateSettings(settings Settings) error {
	var problems []config.Problem
	invalid := func(field string, reason string) {
		problems = append(problems, config.Problem{Field: field, Code: value_objects.ErrorCodeConfigFieldInvalid, Reason: reason})
	}

	// ----- format -----
	if settings.Format != domain.ConsoleFormatJSON && settings.Format != domain.ConsoleFormatText {
		invalid("format", "it must be json or text")
	}

	// ----- minimum_severity -----
	if value_objects.ValidateSeverity(settings.MinimumSeverity) != nil {
		invalid("minimum_severity", "it must be TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL, in upper case")
	}

	if len(problems) == 0 {
		return nil
	}
	return config.NewValidationError(problems)
}
