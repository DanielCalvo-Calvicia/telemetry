package http_client

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client/helpers"
)

// =============================================================================
// SETTINGS
// =============================================================================

// Settings are the settings that every HTTP adapter shares, the same model as contracts/config/http_client.schema.json
// (the json tags are the names of the schema, the env tags are its x-env names). The library never reads the
// environment or a file (ADR-004): the consumer loads the values and passes them in. The env tags carry no default on
// purpose; the defaults live in DefaultSettings. An empty text means not set. The two secrets (BearerToken and
// APIKeyValue) have no default and never appear in an error, a log or an outcome (REQ-SEC-007).
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
}

// DefaultSettings returns the settings with their defaults: POST, a timeout of 3000 ms, 5 attempts, a backoff from
// 200 ms to 30000 ms. The URL has no default (the library fixes none) and no secret has one.
func DefaultSettings() Settings {
	return Settings{
		Method:           "POST",
		TimeoutMS:        domain.DefaultTimeoutMilliseconds,
		MaxAttempts:      domain.DefaultHTTPMaxAttempts,
		RetryBaseDelayMS: domain.DefaultRetryBaseDelayMilliseconds,
		RetryMaxDelayMS:  domain.DefaultRetryMaxDelayMilliseconds,
	}
}

// ValidateSettings checks the settings. It returns nil when they are valid, or a *config.ValidationError with every
// problem together, in the order of the properties of the schema. Each problem names the field and gives a fixed
// reason; no reason contains a configured value, because a value may be a secret or a URL with a secret. This is the
// reference of the shared cases of integration-tests/fixtures/http.
func ValidateSettings(settings Settings) error {
	var problems []config.Problem
	invalid := func(field string, reason string) {
		problems = append(problems, config.Problem{Field: field, Code: value_objects.ErrorCodeConfigFieldInvalid, Reason: reason})
	}

	// ----- url -----
	has_authentication := settings.BearerToken != "" || settings.APIKeyHeader != "" || settings.APIKeyValue != ""
	parsed_url, parse_error := url.Parse(settings.URL)
	switch {
	case parse_error != nil || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") || parsed_url.Hostname() == "":
		invalid("url", "it must be an http or https URL with a host")
	case !is_valid_port(parsed_url.Port()):
		invalid("url", "the port of the URL must be a number from 1 to 65535")
	case parsed_url.Scheme == "http" && has_authentication && !is_local_host(parsed_url.Hostname()):
		invalid("url", "authentication over plain http is only allowed to localhost: use https")
	}

	// ----- method, timeout_ms -----
	if settings.Method != "POST" && settings.Method != "PUT" && settings.Method != "PATCH" {
		invalid("method", "it must be POST, PUT or PATCH")
	}
	if settings.TimeoutMS <= 0 {
		invalid("timeout_ms", "it must be greater than zero")
	}

	// ----- bearer_token -----
	if !helpers.IsVisibleASCII(settings.BearerToken) {
		invalid("bearer_token", "when set it must be visible ASCII characters only, with no space and no control character")
	}

	// ----- api_key_header -----
	switch {
	case settings.APIKeyHeader != "" && !helpers.IsHTTPToken(settings.APIKeyHeader):
		invalid("api_key_header", "when set it must be a valid HTTP header name")
	case settings.APIKeyValue != "" && settings.APIKeyHeader == "":
		invalid("api_key_header", "it is required when the API key value is set")
	case settings.BearerToken != "" && (settings.APIKeyHeader != "" || settings.APIKeyValue != ""):
		invalid("api_key_header", "only one authentication is allowed: a bearer token or an API key")
	}

	// ----- api_key_value -----
	switch {
	case settings.APIKeyHeader != "" && settings.APIKeyValue == "":
		invalid("api_key_value", "it is required when the API key header is set")
	case !helpers.IsVisibleASCII(settings.APIKeyValue):
		invalid("api_key_value", "when set it must be visible ASCII characters only, with no space and no control character")
	}

	// ----- max_attempts, retry_base_delay_ms, retry_max_delay_ms -----
	if settings.MaxAttempts <= 0 {
		invalid("max_attempts", "it must be greater than zero")
	}
	if settings.RetryBaseDelayMS <= 0 {
		invalid("retry_base_delay_ms", "it must be greater than zero")
	}
	if settings.RetryMaxDelayMS <= 0 {
		invalid("retry_max_delay_ms", "it must be greater than zero")
	} else if settings.RetryBaseDelayMS > 0 && settings.RetryMaxDelayMS < settings.RetryBaseDelayMS {
		invalid("retry_max_delay_ms", "it must not be less than the base delay")
	}

	if len(problems) == 0 {
		return nil
	}
	return config.NewValidationError(problems)
}

// ----- HELPERS --------------------------------------------------------------

// is_valid_port tells whether the port text of a URL is empty (no port) or a whole number from 1 to 65535.
func is_valid_port(port string) bool {
	if port == "" {
		return true
	}
	port_number, parse_error := strconv.Atoi(port)
	return parse_error == nil && port_number >= 1 && port_number <= 65535 && port[0] != '+' && port[0] != '-'
}

// is_local_host tells whether a host name is localhost, 127.0.0.1 or ::1.
func is_local_host(host string) bool {
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1"
}
