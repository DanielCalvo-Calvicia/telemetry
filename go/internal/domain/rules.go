package domain

// #### VALUE OBJECT RULES

// The raw values of the rules of the value objects. They are untyped; the value object that uses one wraps it.

// === UTCInstant ===

const (
	// UTCInstantLayout is the Go layout of the text of an instant: UTC, exactly six decimals (trailing zeros kept,
	// the nanoseconds are cut) and the letter Z (ADR-012).
	UTCInstantLayout = "2006-01-02T15:04:05.000000Z"
	// UTCInstantMinimumYear is the first year that an instant may have.
	UTCInstantMinimumYear = 1
	// UTCInstantMaximumYear is the last year that an instant may have.
	UTCInstantMaximumYear = 9999
)

// === EventID ===

const (
	// EventIDGroupSeparator separates the five groups of an event id.
	EventIDGroupSeparator = "-"
	// EventIDVersionDigit is the first digit of the third group of a UUID of version 7.
	EventIDVersionDigit = "7"
	// EventIDVariantDigits are the digits that the fourth group of an event id may start with (variant of RFC 4122).
	EventIDVariantDigits = "89ab"
)

// === ResourceIdentity ===

const (
	// TimeZonePattern is the shape of the time_zone of a resource: names made of letters, digits, underscore, plus and
	// minus, joined by slashes. It is a shape, not a check against the IANA database, because the field is for display.
	TimeZonePattern = `^[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+)*$`
)

// === CorrelationContext ===

const (
	// TraceIDWidth is the number of hexadecimal digits of a W3C trace id.
	TraceIDWidth = 32
	// SpanIDWidth is the number of hexadecimal digits of a W3C span id.
	SpanIDWidth = 16
)

// #### ENVELOPE RULES

// The raw values of the rules of the envelope. They are untyped.

// === SchemaVersion ===

const (
	// SchemaVersion is the version of the contract that every envelope carries in schema_version: major.minor.
	SchemaVersion = "0.1"
)

// #### LIMIT RULES

// The default size limits (REQ-CTR-010, REQ-SEC-004). They are untyped; the Limits value object wraps them and the
// consumer may change each one (T-03.6 maps the configuration to NewLimits). Texts are counted in characters (Unicode
// code points); the event and the batch are counted in bytes of the canonical JSON.

// === Attributes ===

const (
	// DefaultMaxAttributeKeys is the default number of attributes that an event may keep.
	DefaultMaxAttributeKeys = 32
	// DefaultMaxAttributeKeyChars is the default length of an attribute key, in characters.
	DefaultMaxAttributeKeyChars = 64
	// DefaultMaxAttributeValueChars is the default length of a text value of an attribute, in characters.
	DefaultMaxAttributeValueChars = 256
)

// === Event and batch ===

const (
	// DefaultMaxEventBytes is the default size of one event in canonical JSON: 64 KiB.
	DefaultMaxEventBytes = 65536
	// DefaultMaxBatchBytes is the default size of one HTTP batch: 1 MiB.
	DefaultMaxBatchBytes = 1048576
)

// #### MASKING RULES

// Everything sensitive is hidden behind MaskedText, wherever it appears: as the value of a field with a sensitive name,
// as a whole value, or as a segment of a longer text (ADR-006). The patterns below are the contract shared with the
// Python implementation: they are copied byte for byte from the first library and are written in the subset that both
// regular expression engines understand. Nothing is removed from them (Q06-5).

// === Replacement text ===

const (
	// MaskedText replaces every piece of sensitive data in an event.
	MaskedText = "*****MASKED*****"

	// MaskKeepFirstGroup is the replacement that keeps the first captured group (the key or the scheme in front
	// of the secret) and hides the rest of the match.
	MaskKeepFirstGroup = "${1}" + MaskedText

	// MaskKeepOuterGroups is the replacement that keeps the first and the third captured group and hides the second.
	MaskKeepOuterGroups = "${1}" + MaskedText + "${3}"
)

// === Detector names ===

const (
	// DetectorAWSAccessKey is the name of the detector of AWS access key IDs.
	DetectorAWSAccessKey = "aws_access_key"
	// DetectorBearerCredential is the name of the detector of the credential that follows an authorization scheme.
	DetectorBearerCredential = "bearer_credential"
	// DetectorJWT is the name of the detector of JSON Web Tokens.
	DetectorJWT = "jwt"
	// DetectorPrivateKeyBlock is the name of the detector of PEM private key blocks.
	DetectorPrivateKeyBlock = "private_key_block"
	// DetectorConnectionStringPassword is the name of the detector of the password of a URL with credentials.
	DetectorConnectionStringPassword = "connection_string_password"
	// DetectorCredentialPair is the name of the detector of credentials written as a pair.
	DetectorCredentialPair = "credential_pair"
)

// === AWS access key ===

const (
	// AWSAccessKeyPattern finds an AWS access key ID: a four-letter prefix that says what kind of key it is, then 16 characters.
	AWSAccessKeyPattern = `\b(?:AKIA|ASIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ABIA|ACCA)[0-9A-Z]{16}\b`
)

// === Bearer credential ===

const (
	// BearerCredentialPattern finds the credential that follows an authorization scheme: "Bearer" anywhere, and
	// "Basic" or "Token" only right after "Authorization:" (as bare words they appear in normal sentences). The first
	// group is the scheme, with its header when there is one, and is kept.
	BearerCredentialPattern = `(?i)\b((?:bearer|authorization\s*[:=]\s*(?:basic|token))\s+)[A-Za-z0-9._~+/=-]{8,}`
)

// === JSON Web Token ===

const (
	// JWTPattern finds a JSON Web Token: three base64url parts, the first two starting like a JSON object.
	JWTPattern = `\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*`
)

// === Private key block ===

const (
	// PrivateKeyBlockPattern finds a PEM private key block, from its BEGIN line to its END line, or to the end of the
	// text when the block is cut.
	PrivateKeyBlockPattern = `(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----(?:.*?-----END [A-Z ]*PRIVATE KEY-----|.*)`
)

// === Connection string password ===

const (
	// ConnectionStringPasswordPattern finds the password of a URL with credentials (scheme://user:password@host);
	// the first group is everything up to the password and the third is the "@", both kept.
	ConnectionStringPasswordPattern = `([a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:)([^\s@/]+)(@)`
)

// === Credential pair ===

const (
	// CredentialPairKeys are the endings of the names whose value is hidden when they appear as name=value,
	// name: value or "name": "value" inside a text (a name may carry a prefix, as in db_password or
	// X-Amz-Signature).
	CredentialPairKeys = `password|passwd|pwd|passphrase|secret|secret[_-]?key|secret[_-]?access[_-]?key|access[_-]?key|api[_-]?key|token|client[_-]?secret|private[_-]?key|credentials?|signature`

	// CredentialPairPattern finds a credential written as a pair; the first group (the name and its separator) is kept
	// and the value, quoted or not, is hidden.
	CredentialPairPattern = `(?i)(["']?[a-z0-9_.-]*(?:` + CredentialPairKeys + `)["']?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;&"'}]+)`
)

// === Sensitive field names ===

// FieldNameCamelBoundaryPattern finds the place between a lower case letter or digit and an upper case letter,
// so that accessKey is read as access_key.
const FieldNameCamelBoundaryPattern = `([a-z0-9])([A-Z])`

// SensitiveFieldNames is the default list of field names (in snake case) whose value is always hidden, whatever it
// contains (REQ-SEC-002). A name matches when it is equal to one of them or ends with "_" and one of them (so
// db_password and request_body match, but secret_id, secret_name, token_count and key do not). A consumer may
// replace the list or add to it (T-03.6 maps the configuration to the options of the masker).
//
// Open question for the maintainer: body, content and payload are always hidden, kept from the old library; whether
// that still fits the typed data of the new envelope is a question for the maintainer.
var SensitiveFieldNames = []string{
	"password", "passwd", "pwd", "passphrase",
	"secret", "secret_string", "secret_binary", "secret_value", "secret_key", "secret_access_key", "client_secret",
	"access_key", "api_key", "apikey", "private_key",
	"token", "authorization", "credential", "credentials", "cookie", "signature",
	"body", "message_body", "content", "payload",
}

// === Classification rules ===

const (
	// MaskRuleDrop removes the field that belongs to the class.
	MaskRuleDrop = "drop"
	// MaskRuleRedact replaces the value of the field that belongs to the class by MaskedText.
	MaskRuleRedact = "redact"
	// MaskRuleAllow skips the rule of the sensitive field names for the field of the class (the detectors of text still
	// run on its text, and maps and lists are still walked).
	MaskRuleAllow = "allow"
)
