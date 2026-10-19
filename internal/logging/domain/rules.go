package domain

// #############################################################################
// VALUE OBJECT RULES
// #############################################################################

// The constants below are untyped on purpose: domain cannot import
// value_objects (value_objects already imports domain for its error aliases).
// Each value object wraps them into its own typed constants.

// =============================================================================
// Level
// =============================================================================
const (
	LevelTrace = "TRACE"
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
	LevelFatal = "FATAL"
)

// ValidLevels lists every accepted log level, from the least to the most severe.
var ValidLevels = []string{LevelTrace, LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal}

// =============================================================================
// Environment (APP_ENV) and its minimum level
// =============================================================================
const (
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
)

// MinimumLevelByEnvironment gives the lowest level that is logged in each environment: development lets
// DEBUG and above through, production lets INFO and above through (Q18-14, Q19-2). TRACE is never logged.
var MinimumLevelByEnvironment = map[string]string{
	EnvironmentDevelopment: LevelDebug,
	EnvironmentProduction:  LevelInfo,
}

// #############################################################################
// OPERATION RULES
// #############################################################################

// =============================================================================
// SensitiveDataMasker
// Everything sensitive is hidden behind MaskedText, wherever it appears: as the
// value of a field with a sensitive name, as a whole value, or as a segment of
// a longer text (Q17-1, Q17-2, Q17-3). The detectors below are data: one
// pattern per detector, each in its own file of domain/operations.
// =============================================================================
const (
	// MaskedText replaces every piece of sensitive data in a log entry.
	MaskedText = "*****MASKED*****"

	// MaskKeepFirstGroup is the replacement that keeps the first captured group (the key or the scheme in front
	// of the secret) and hides the rest of the match.
	MaskKeepFirstGroup = "${1}" + MaskedText

	// MaskKeepOuterGroups is the replacement that keeps the first and the third captured group and hides the second.
	MaskKeepOuterGroups = "${1}" + MaskedText + "${3}"
)

// Detector names, used in tests and in the message of a failing extra pattern.
const (
	DetectorAWSAccessKey             = "aws_access_key"
	DetectorBearerCredential         = "bearer_credential"
	DetectorJWT                      = "jwt"
	DetectorPrivateKeyBlock          = "private_key_block"
	DetectorConnectionStringPassword = "connection_string_password"
	DetectorCredentialPair           = "credential_pair"
)

// AWSAccessKeyPattern finds an AWS access key ID: a four-letter prefix that says what kind of key it is, then 16 characters.
const AWSAccessKeyPattern = `\b(?:AKIA|ASIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ABIA|ACCA)[0-9A-Z]{16}\b`

// BearerCredentialPattern finds the credential that follows an authorization scheme: "Bearer" anywhere, and
// "Basic" or "Token" only right after "Authorization:" (as bare words they appear in normal sentences). The first
// group is the scheme, with its header when there is one, and is kept.
const BearerCredentialPattern = `(?i)\b((?:bearer|authorization\s*[:=]\s*(?:basic|token))\s+)[A-Za-z0-9._~+/=-]{8,}`

// JWTPattern finds a JSON Web Token: three base64url parts, the first two starting like a JSON object.
const JWTPattern = `\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*`

// PrivateKeyBlockPattern finds a PEM private key block, from its BEGIN line to its END line, or to the end of the
// text when the block is cut.
const PrivateKeyBlockPattern = `(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----(?:.*?-----END [A-Z ]*PRIVATE KEY-----|.*)`

// ConnectionStringPasswordPattern finds the password of a URL with credentials (scheme://user:password@host);
// the first group is everything up to the password and the third is the "@", both kept.
const ConnectionStringPasswordPattern = `([a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:)([^\s@/]+)(@)`

// CredentialPairKeys are the endings of the names whose value is hidden when they appear as name=value,
// name: value or "name": "value" inside a text (a name may carry a prefix, as in db_password or
// X-Amz-Signature).
const CredentialPairKeys = `password|passwd|pwd|passphrase|secret|secret[_-]?key|secret[_-]?access[_-]?key|access[_-]?key|api[_-]?key|token|client[_-]?secret|private[_-]?key|credentials?|signature`

// CredentialPairPattern finds a credential written as a pair; the first group (the name and its separator) is kept
// and the value, quoted or not, is hidden.
const CredentialPairPattern = `(?i)(["']?[a-z0-9_.-]*(?:` + CredentialPairKeys + `)["']?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;&"'}]+)`

// FieldNameCamelBoundaryPattern finds the place between a lower case letter or digit and an upper case letter,
// so that accessKey is read as access_key.
const FieldNameCamelBoundaryPattern = `([a-z0-9])([A-Z])`

// SensitiveFieldNames are the field names (in snake case) whose value is always hidden, whatever it contains.
// A name matches when it is equal to one of them or ends with "_" and one of them (so db_password and
// request_body match, but secret_id, secret_name, token_count and key do not).
var SensitiveFieldNames = []string{
	"password", "passwd", "pwd", "passphrase",
	"secret", "secret_string", "secret_binary", "secret_value", "secret_key", "secret_access_key", "client_secret",
	"access_key", "api_key", "apikey", "private_key",
	"token", "authorization", "credential", "credentials", "cookie", "signature",
	"body", "message_body", "content", "payload",
}
