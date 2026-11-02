package file

import (
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
)

// =============================================================================
// SETTINGS
// =============================================================================

// Settings are the settings of the file destination, the same model as contracts/config/file.schema.json (the json
// tags are the names of the schema, the env tags are its x-env names). The library never reads the environment or a
// file of settings (ADR-004): the consumer loads the values and passes them in. The env tags carry no default on
// purpose; the defaults live in DefaultSettings.
type Settings struct {
	// Directory is the directory of the files. It is required: the library picks no path. It is created, readable by
	// the owner only, when it does not exist.
	Directory string `json:"directory" env:"TELEMETRY_FILE_DIRECTORY"`
	// FileName is the name of the current file; it holds no path separator.
	FileName string `json:"file_name" env:"TELEMETRY_FILE_NAME"`
	// MinimumSeverity is the lowest severity that is written, in upper case: TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL.
	MinimumSeverity string `json:"minimum_severity" env:"TELEMETRY_FILE_MINIMUM_SEVERITY"`
	// MaxFileBytes is the size at which the file rotates: it rotates when the next line would pass it.
	MaxFileBytes int64 `json:"max_file_bytes" env:"TELEMETRY_FILE_MAX_FILE_BYTES"`
	// MaxFiles is how many files are kept, the current one included.
	MaxFiles int `json:"max_files" env:"TELEMETRY_FILE_MAX_FILES"`
	// QuotaBytes is the most that all the files together may use; it is not below MaxFileBytes.
	QuotaBytes int64 `json:"quota_bytes" env:"TELEMETRY_FILE_QUOTA_BYTES"`
}

// DefaultSettings returns the settings with their defaults. The Directory is empty on purpose: the consumer must give
// one, and ValidateSettings refuses the settings until it does.
func DefaultSettings() Settings {
	return Settings{
		FileName:        domain.DefaultFileName,
		MinimumSeverity: domain.DefaultFileMinimumSeverity,
		MaxFileBytes:    domain.DefaultFileMaxFileBytes,
		MaxFiles:        domain.DefaultFileMaxFiles,
		QuotaBytes:      domain.DefaultFileQuotaBytes,
	}
}

// ValidateSettings checks the settings. It returns nil when they are valid, or a *config.ValidationError with every
// problem together, in the order of the properties of the schema. Each problem names the field and gives a fixed
// reason; no reason contains a configured value. Whether the directory can be written is checked when the destination
// is built, not here. This is the reference of the shared cases of integration-tests/fixtures/file.
func ValidateSettings(settings Settings) error {
	var problems []config.Problem
	invalid := func(field string, reason string) {
		problems = append(problems, config.Problem{Field: field, Code: value_objects.ErrorCodeConfigFieldInvalid, Reason: reason})
	}

	// ----- directory -----
	if settings.Directory == "" || has_control_character(settings.Directory) {
		invalid("directory", "it must be a directory, with no control character")
	}

	// ----- file_name -----
	if settings.FileName == "" || strings.ContainsAny(settings.FileName, `/\`) || settings.FileName == "." || settings.FileName == ".." ||
		has_control_character(settings.FileName) {
		invalid("file_name", "it must be a file name, with no path separator and no control character")
	}

	// ----- minimum_severity -----
	if value_objects.ValidateSeverity(settings.MinimumSeverity) != nil {
		invalid("minimum_severity", "it must be TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL, in upper case")
	}

	// ----- max_file_bytes, max_files, quota_bytes -----
	if settings.MaxFileBytes <= 0 {
		invalid("max_file_bytes", "it must be greater than zero")
	}
	if settings.MaxFiles <= 0 {
		invalid("max_files", "it must be greater than zero")
	}
	if settings.QuotaBytes <= 0 {
		invalid("quota_bytes", "it must be greater than zero")
	} else if settings.MaxFileBytes > 0 && settings.QuotaBytes < settings.MaxFileBytes {
		invalid("quota_bytes", "it must not be below max_file_bytes")
	}

	if len(problems) == 0 {
		return nil
	}
	return config.NewValidationError(problems)
}

// ----- HELPERS --------------------------------------------------------------

// has_control_character tells whether the text holds a character below U+0020 or U+007F.
func has_control_character(text string) bool {
	for _, character := range text {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}
