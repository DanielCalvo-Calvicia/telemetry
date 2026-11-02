package helpers

// =============================================================================
// AUTH HEADER
// =============================================================================

// AuthorizationHeaderName is the header that carries a bearer token.
const AuthorizationHeaderName = "Authorization"

// AuthHeader returns the header that carries the authentication: a bearer token goes in Authorization as
// "Bearer <token>"; an API key goes in the header with the configured name; ok is false when nothing is configured.
// The validation of the settings already refused a token and an API key together.
func AuthHeader(bearer_token string, api_key_header string, api_key_value string) (name string, value string, ok bool) {
	if bearer_token != "" {
		return AuthorizationHeaderName, "Bearer " + bearer_token, true
	}
	if api_key_header != "" && api_key_value != "" {
		return api_key_header, api_key_value, true
	}
	return "", "", false
}
