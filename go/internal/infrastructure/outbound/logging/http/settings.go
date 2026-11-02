package http_destination

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
)

// =============================================================================
// SETTINGS
// =============================================================================

// Settings are the settings of the HTTP destination: the settings of the shared client (contracts/config/http_client.schema.json)
// followed by minimum_severity (contracts/config/http_destination.schema.json). The twelve fields are declared here, with the
// same tags as http_client.Settings, and not embedded: the canonical JSON writer refuses an embedded struct, and one flat
// list gives the same twelve keys to every writer. ClientSettings gives the part of the shared client. The library never
// reads the environment or a file (ADR-004): the consumer loads the values and passes them in. The env tags carry no
// default on purpose; the defaults live in DefaultSettings. The two secrets (BearerToken and APIKeyValue) have no default
// and never appear in an error, a log or a result (REQ-SEC-007).
type Settings struct {
	// URL is the destination, http or https. Required. Authentication over plain http is refused except to localhost.
	URL string `json:"url" env:"TELEMETRY_HTTP_URL"`
	// Method is POST, PUT or PATCH.
	Method string `json:"method" env:"TELEMETRY_HTTP_METHOD"`
	// TimeoutMS is the time in milliseconds before a request gives up.
	TimeoutMS int `json:"timeout_ms" env:"TELEMETRY_HTTP_TIMEOUT_MS"`
	// Compress sends the body gzipped with Content-Encoding: gzip.
	Compress bool `json:"compress" env:"TELEMETRY_HTTP_COMPRESS"`
	// SkipTLSVerification turns the verification of the TLS certificate off. It is on by default.
	SkipTLSVerification bool `json:"skip_tls_verification" env:"TELEMETRY_HTTP_SKIP_TLS_VERIFICATION"`
	// BearerToken is a bearer token for the Authorization header. A secret. Not together with an API key.
	BearerToken string `json:"bearer_token" env:"TELEMETRY_HTTP_BEARER_TOKEN"`
	// APIKeyHeader is the name of the header that carries an API key. Needs APIKeyValue.
	APIKeyHeader string `json:"api_key_header" env:"TELEMETRY_HTTP_API_KEY_HEADER"`
	// APIKeyValue is the API key. A secret. Needs APIKeyHeader.
	APIKeyValue string `json:"api_key_value" env:"TELEMETRY_HTTP_API_KEY_VALUE"`
	// MaxAttempts is the number of attempts at most for one batch, the first included.
	MaxAttempts int `json:"max_attempts" env:"TELEMETRY_HTTP_MAX_ATTEMPTS"`
	// RetryBaseDelayMS is the base of the exponential backoff in milliseconds.
	RetryBaseDelayMS int `json:"retry_base_delay_ms" env:"TELEMETRY_HTTP_RETRY_BASE_DELAY_MS"`
	// RetryMaxDelayMS is the ceiling of the backoff in milliseconds. Not below the base.
	RetryMaxDelayMS int `json:"retry_max_delay_ms" env:"TELEMETRY_HTTP_RETRY_MAX_DELAY_MS"`
	// MinimumSeverity is the lowest severity that is sent, in upper case: TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL.
	MinimumSeverity string `json:"minimum_severity" env:"TELEMETRY_HTTP_MINIMUM_SEVERITY"`
}

// DefaultSettings returns the settings with their defaults: the defaults of the shared client (POST, a timeout of
// 3000 ms, 5 attempts, a backoff from 200 ms to 30000 ms) and a minimum severity of INFO. The URL has no default and no
// secret has one.
func DefaultSettings() Settings {
	client_settings := http_client.DefaultSettings()
	return Settings{
		URL:                 client_settings.URL,
		Method:              client_settings.Method,
		TimeoutMS:           client_settings.TimeoutMS,
		Compress:            client_settings.Compress,
		SkipTLSVerification: client_settings.SkipTLSVerification,
		BearerToken:         client_settings.BearerToken,
		APIKeyHeader:        client_settings.APIKeyHeader,
		APIKeyValue:         client_settings.APIKeyValue,
		MaxAttempts:         client_settings.MaxAttempts,
		RetryBaseDelayMS:    client_settings.RetryBaseDelayMS,
		RetryMaxDelayMS:     client_settings.RetryMaxDelayMS,
		MinimumSeverity:     domain.DefaultConsoleMinimumSeverity,
	}
}

// ClientSettings returns the part of the settings that belongs to the shared HTTP client.
func (settings Settings) ClientSettings() http_client.Settings {
	return http_client.Settings{
		URL:                 settings.URL,
		Method:              settings.Method,
		TimeoutMS:           settings.TimeoutMS,
		Compress:            settings.Compress,
		SkipTLSVerification: settings.SkipTLSVerification,
		BearerToken:         settings.BearerToken,
		APIKeyHeader:        settings.APIKeyHeader,
		APIKeyValue:         settings.APIKeyValue,
		MaxAttempts:         settings.MaxAttempts,
		RetryBaseDelayMS:    settings.RetryBaseDelayMS,
		RetryMaxDelayMS:     settings.RetryMaxDelayMS,
	}
}

// ValidateSettings checks the settings. It returns nil when they are valid, or a *config.ValidationError with every
// problem together: first the problems of http_client.ValidateSettings (same order, same codes) and then the one of
// minimum_severity. Each problem names the field and gives a fixed reason; no reason contains a configured value,
// because a value may be a secret or a URL with a secret. This is the reference of the shared cases of
// integration-tests/fixtures/http_destination.
func ValidateSettings(settings Settings) error {
	var problems []config.Problem
	if client_error := http_client.ValidateSettings(settings.ClientSettings()); client_error != nil {
		if validation_error, is_validation_error := client_error.(*config.ValidationError); is_validation_error {
			problems = append(problems, validation_error.Problems...)
		}
	}
	if value_objects.ValidateSeverity(settings.MinimumSeverity) != nil {
		problems = append(problems, config.Problem{
			Field:  "minimum_severity",
			Code:   value_objects.ErrorCodeConfigFieldInvalid,
			Reason: "it must be TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL, in upper case",
		})
	}
	if len(problems) == 0 {
		return nil
	}
	return config.NewValidationError(problems)
}
