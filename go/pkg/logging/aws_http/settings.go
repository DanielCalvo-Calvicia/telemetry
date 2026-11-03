package aws_http

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/http_client"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http/helpers"
)

// =============================================================================
// SETTINGS
// =============================================================================

// Settings are the settings of the AWS HTTP adapter, the thirteen properties of contracts/config/aws_http.schema.json in
// the order of the schema: the five that locate the add_log endpoint and the log group and stream, then the ones of the
// shared client (timeout, TLS, authentication, retry). The method is always POST, the body is never compressed and no
// redirect is followed, so those three settings of the shared client are not here. The library never reads the
// environment or a file (ADR-004): the consumer loads the values and passes them in. The env tags carry no default on
// purpose; the defaults live in DefaultSettings. The two secrets (BearerToken and APIKeyValue) have no default and never
// appear in an error, a log or a result (REQ-SEC-007).
type Settings struct {
	// Host is the host WITH its scheme, for example http://localhost. Required, http or https.
	Host string `json:"host" env:"TELEMETRY_AWS_HTTP_HOST"`
	// Port is the port, 1 to 65535. Required.
	Port string `json:"port" env:"TELEMETRY_AWS_HTTP_PORT"`
	// Path is the path of the add_log endpoint, starting with /. Required.
	Path string `json:"path" env:"TELEMETRY_AWS_HTTP_PATH"`
	// LogGroupName is the CloudWatch log group the entries go to. Required.
	LogGroupName string `json:"log_group_name" env:"TELEMETRY_AWS_HTTP_LOG_GROUP_NAME"`
	// LogStreamName is the CloudWatch log stream the entries go to. Required.
	LogStreamName string `json:"log_stream_name" env:"TELEMETRY_AWS_HTTP_LOG_STREAM_NAME"`
	// TimeoutMS is the time in milliseconds before a request gives up.
	TimeoutMS int `json:"timeout_ms" env:"TELEMETRY_AWS_HTTP_TIMEOUT_MS"`
	// SkipTLSVerification turns the verification of the TLS certificate off. It is on by default.
	SkipTLSVerification bool `json:"skip_tls_verification" env:"TELEMETRY_AWS_HTTP_SKIP_TLS_VERIFICATION"`
	// BearerToken is a bearer token for the Authorization header. A secret. Not together with an API key.
	BearerToken string `json:"bearer_token" env:"TELEMETRY_AWS_HTTP_BEARER_TOKEN"`
	// APIKeyHeader is the name of the header that carries an API key. Needs APIKeyValue.
	APIKeyHeader string `json:"api_key_header" env:"TELEMETRY_AWS_HTTP_API_KEY_HEADER"`
	// APIKeyValue is the API key. A secret. Needs APIKeyHeader.
	APIKeyValue string `json:"api_key_value" env:"TELEMETRY_AWS_HTTP_API_KEY_VALUE"`
	// MaxAttempts is the number of attempts at most for one request, the first included.
	MaxAttempts int `json:"max_attempts" env:"TELEMETRY_AWS_HTTP_MAX_ATTEMPTS"`
	// RetryBaseDelayMS is the base of the exponential backoff in milliseconds.
	RetryBaseDelayMS int `json:"retry_base_delay_ms" env:"TELEMETRY_AWS_HTTP_RETRY_BASE_DELAY_MS"`
	// RetryMaxDelayMS is the ceiling of the backoff in milliseconds. Not below the base.
	RetryMaxDelayMS int `json:"retry_max_delay_ms" env:"TELEMETRY_AWS_HTTP_RETRY_MAX_DELAY_MS"`
}

// DefaultSettings returns the settings with their defaults: the defaults of the shared client (a timeout of 3000 ms,
// 5 attempts, a backoff from 200 ms to 30000 ms) and nothing else: the five required settings are empty (set them before
// NewDestination), TLS is verified and no secret has a default.
func DefaultSettings() Settings {
	client_settings := http_client.DefaultSettings()
	return Settings{
		TimeoutMS:        client_settings.TimeoutMS,
		MaxAttempts:      client_settings.MaxAttempts,
		RetryBaseDelayMS: client_settings.RetryBaseDelayMS,
		RetryMaxDelayMS:  client_settings.RetryMaxDelayMS,
	}
}

// ClientSettings returns the settings of the shared HTTP client of the core: the URL is the endpoint (host, port and
// path), the method is POST, the body is not compressed, and the rest are the settings of the same name.
func (settings Settings) ClientSettings() http_client.Settings {
	return http_client.Settings{
		URL:                 helpers.BuildEndpoint(settings.Host, settings.Port, settings.Path),
		Method:              "POST",
		TimeoutMS:           settings.TimeoutMS,
		Compress:            false,
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
// problem together, in the order of aws_http.schema.json: host, port, path, log_group_name and log_stream_name first and
// then the problems of http_client.ValidateSettings for the rest (timeout, authentication, retry). A problem of the URL
// of the shared client (authentication over plain http to a host that is not local) is reported on host. Each problem
// names the field and gives a fixed reason with the code config_field_invalid; no reason contains a configured value,
// because a value may be a secret. This is the reference of the shared cases of integration-tests/fixtures/aws_http.
func ValidateSettings(settings Settings) error {
	invalid := func(field string, reason string) config.Problem {
		return config.Problem{Field: field, Code: value_objects.ErrorCodeConfigFieldInvalid, Reason: reason}
	}

	var host_problems, port_problems, path_problems, group_problems, stream_problems []config.Problem
	if !is_valid_host(settings.Host) {
		host_problems = append(host_problems, invalid("host", "it must be an http or https scheme followed by a host, with no path, query or space"))
	}
	if !is_valid_port(settings.Port) {
		port_problems = append(port_problems, invalid("port", "it must be a number from 1 to 65535"))
	}
	if !strings.HasPrefix(settings.Path, "/") {
		path_problems = append(path_problems, invalid("path", "it must start with /"))
	}
	if settings.LogGroupName == "" {
		group_problems = append(group_problems, invalid("log_group_name", "it is required"))
	}
	if settings.LogStreamName == "" {
		stream_problems = append(stream_problems, invalid("log_stream_name", "it is required"))
	}

	// The shared client checks the rest. While the endpoint is not valid its URL cannot be judged, so the client is given
	// a URL that is always valid and only its other settings are checked.
	client_settings := settings.ClientSettings()
	if len(host_problems)+len(port_problems)+len(path_problems) > 0 {
		client_settings.URL = "https://placeholder.invalid"
	}
	var client_problems []config.Problem
	if client_error := http_client.ValidateSettings(client_settings); client_error != nil {
		if validation_error, is_validation_error := client_error.(*config.ValidationError); is_validation_error {
			for _, problem := range validation_error.Problems {
				if problem.Field == "url" {
					host_problems = append(host_problems, invalid("host", problem.Reason))
					continue
				}
				client_problems = append(client_problems, problem)
			}
		}
	}

	var problems []config.Problem
	for _, group := range [][]config.Problem{host_problems, port_problems, path_problems, group_problems, stream_problems, client_problems} {
		problems = append(problems, group...)
	}
	if len(problems) == 0 {
		return nil
	}
	return config.NewValidationError(problems)
}

// ----- HELPERS --------------------------------------------------------------

// is_valid_host tells whether the host is an http or https scheme followed by a non-empty authority that has no path,
// no query, no fragment and no space.
func is_valid_host(host string) bool {
	authority, has_scheme := strings.CutPrefix(host, "http://")
	if !has_scheme {
		authority, has_scheme = strings.CutPrefix(host, "https://")
	}
	if !has_scheme || authority == "" {
		return false
	}
	return !strings.ContainsFunc(authority, func(character rune) bool {
		return character == '/' || character == '?' || character == '#' || unicode.IsSpace(character)
	})
}

// is_valid_port tells whether the port is only digits and a number from 1 to 65535.
func is_valid_port(port string) bool {
	if port == "" || strings.ContainsFunc(port, func(character rune) bool { return character < '0' || character > '9' }) {
		return false
	}
	port_number, parse_error := strconv.Atoi(port)
	return parse_error == nil && port_number >= 1 && port_number <= 65535
}
