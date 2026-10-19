package config

// =============================================================================
// TELEMETRY CONFIG
// =============================================================================

// TelemetryConfig holds the settings of the logger, populated by the single environment load of
// composition_root/config — this struct itself does no loading. The minimum level is not configured here: it
// follows APP_ENV (development logs DEBUG and above, production INFO and above).
//
// Host includes the scheme (for example "http://localhost"). MaskExtraPatterns are regular expressions, added
// to the built-in detectors of sensitive data, separated by ";" (a comma would split patterns such as \d{2,5}).
type TelemetryConfig struct {
	Host              string   `env:"LOGGER_HOST,default=http://localhost"`
	Port              string   `env:"LOGGER_PORT,default=3000"`
	Path              string   `env:"LOGGER_PATH,default=/logs"`
	ConsoleEnabled    bool     `env:"LOGGER_CONSOLE_ENABLED,default=true"`
	HTTPEnabled       bool     `env:"LOGGER_HTTP_ENABLED,default=false"`
	MaskExtraPatterns []string `env:"LOGGER_MASK_EXTRA_PATTERNS, delimiter=;"`
}
