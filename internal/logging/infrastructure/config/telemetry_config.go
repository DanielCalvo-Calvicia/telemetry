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
//
// The AWS HTTP sink adapts every entry to the add_log endpoint of aws_microservice (CloudWatch Logs): AWSHTTPHost
// includes the scheme (for example "http://aws-microservice") and AWSHTTPLogGroup and AWSHTTPLogStream name the
// destination, which the endpoint creates when it does not exist. They are required when AWSHTTPEnabled is true.
type TelemetryConfig struct {
	Host              string   `env:"LOGGER_HOST,default=http://localhost"`
	Port              string   `env:"LOGGER_PORT,default=3000"`
	Path              string   `env:"LOGGER_PATH,default=/logs"`
	ConsoleEnabled    bool     `env:"LOGGER_CONSOLE_ENABLED,default=true"`
	HTTPEnabled       bool     `env:"LOGGER_HTTP_ENABLED,default=false"`
	MaskExtraPatterns []string `env:"LOGGER_MASK_EXTRA_PATTERNS, delimiter=;"`

	AWSHTTPEnabled   bool   `env:"LOGGER_AWS_HTTP_ENABLED,default=false"`
	AWSHTTPHost      string `env:"LOGGER_AWS_HTTP_HOST,default=http://localhost"`
	AWSHTTPPort      string `env:"LOGGER_AWS_HTTP_PORT,default=8080"`
	AWSHTTPPath      string `env:"LOGGER_AWS_HTTP_PATH,default=/api/v1/cloudwatch/logs/add"`
	AWSHTTPLogGroup  string `env:"LOGGER_AWS_HTTP_LOG_GROUP"`
	AWSHTTPLogStream string `env:"LOGGER_AWS_HTTP_LOG_STREAM"`
}
