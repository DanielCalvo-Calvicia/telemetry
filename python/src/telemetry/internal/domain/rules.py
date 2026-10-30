"""The constants of the contract that the core writes into every event."""

# The version of the envelope contract (contracts/schemas/envelope.schema.json): major.minor.
SCHEMA_VERSION = "0.1"


# =============================================================================
# Masking (ADR-006). The six detector patterns are a CONTRACT shared with Go (RE2): the texts are the same, byte for
# byte, as the ones of the old library (internal/logging/domain/rules.go). Nothing was removed (Q06-5). Python reads
# them through operations/masking/shared_pattern.py, which keeps the two engines equivalent.
# =============================================================================

# MASKED_TEXT replaces every piece of sensitive data in an event.
MASKED_TEXT = "*****MASKED*****"

# The replacement kinds of a text detector (the Go twin uses "${1}" and "${1}...${3}" templates).
REPLACEMENT_WHOLE = "whole"  # the whole match is replaced by MASKED_TEXT
REPLACEMENT_KEEP_FIRST_GROUP = "keep_first_group"  # group 1 is kept and the rest of the match is hidden
REPLACEMENT_KEEP_OUTER_GROUPS = "keep_outer_groups"  # groups 1 and 3 are kept and group 2 is hidden

# Detector names, used in tests.
DETECTOR_AWS_ACCESS_KEY = "aws_access_key"
DETECTOR_BEARER_CREDENTIAL = "bearer_credential"
DETECTOR_JWT = "jwt"
DETECTOR_PRIVATE_KEY_BLOCK = "private_key_block"
DETECTOR_CONNECTION_STRING_PASSWORD = "connection_string_password"
DETECTOR_CREDENTIAL_PAIR = "credential_pair"

# AWS_ACCESS_KEY_PATTERN finds an AWS access key ID: a four-letter prefix that says what kind of key it is, then 16
# characters.
AWS_ACCESS_KEY_PATTERN = r"\b(?:AKIA|ASIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ABIA|ACCA)[0-9A-Z]{16}\b"

# BEARER_CREDENTIAL_PATTERN finds the credential that follows an authorization scheme: "Bearer" anywhere, and "Basic"
# or "Token" only right after "Authorization:". The first group is the scheme, with its header when there is one, and
# is kept.
BEARER_CREDENTIAL_PATTERN = r"(?i)\b((?:bearer|authorization\s*[:=]\s*(?:basic|token))\s+)[A-Za-z0-9._~+/=-]{8,}"

# JWT_PATTERN finds a JSON Web Token: three base64url parts, the first two starting like a JSON object.
JWT_PATTERN = r"\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*"

# PRIVATE_KEY_BLOCK_PATTERN finds a PEM private key block, from its BEGIN line to its END line, or to the end of the
# text when the block is cut.
PRIVATE_KEY_BLOCK_PATTERN = r"(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----(?:.*?-----END [A-Z ]*PRIVATE KEY-----|.*)"

# CONNECTION_STRING_PASSWORD_PATTERN finds the password of a URL with credentials (scheme://user:password@host); the
# first group is everything up to the password and the third is the "@", both kept.
CONNECTION_STRING_PASSWORD_PATTERN = r"([a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:)([^\s@/]+)(@)"

# CREDENTIAL_PAIR_KEYS are the endings of the names whose value is hidden when they appear as name=value, name: value
# or "name": "value" inside a text (a name may carry a prefix, as in db_password or X-Amz-Signature).
CREDENTIAL_PAIR_KEYS = (
    r"password|passwd|pwd|passphrase|secret|secret[_-]?key|secret[_-]?access[_-]?key|access[_-]?key|api[_-]?key"
    r"|token|client[_-]?secret|private[_-]?key|credentials?|signature"
)

# CREDENTIAL_PAIR_PATTERN finds a credential written as a pair; the first group (the name and its separator) is kept
# and the value, quoted or not, is hidden.
CREDENTIAL_PAIR_PATTERN = (
    r"""(?i)(["']?[a-z0-9_.-]*(?:""" + CREDENTIAL_PAIR_KEYS + r""")["']?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;&"'}]+)"""
)

# FIELD_NAME_CAMEL_BOUNDARY_PATTERN finds the place between a lower case letter or digit and an upper case letter, so
# that accessKey is read as access_key.
FIELD_NAME_CAMEL_BOUNDARY_PATTERN = r"([a-z0-9])([A-Z])"

# SENSITIVE_FIELD_NAMES are the field names (in snake case) whose value is always hidden, whatever it contains. A name
# matches when it is equal to one of them or ends with "_" and one of them (so db_password and request_body match,
# but secret_id, secret_name, token_count and key do not). They are kept from the old library, `body`, `content` and
# `payload` included; whether they fit the typed data of the new envelope is a question for the maintainer.
SENSITIVE_FIELD_NAMES = (
    "password",
    "passwd",
    "pwd",
    "passphrase",
    "secret",
    "secret_string",
    "secret_binary",
    "secret_value",
    "secret_key",
    "secret_access_key",
    "client_secret",
    "access_key",
    "api_key",
    "apikey",
    "private_key",
    "token",
    "authorization",
    "credential",
    "credentials",
    "cookie",
    "signature",
    "body",
    "message_body",
    "content",
    "payload",
)

# The rule of a class of fields (REQ-SEC-003).
MASK_RULE_DROP = "drop"  # the field is removed
MASK_RULE_REDACT = "redact"  # the value is replaced by MASKED_TEXT
MASK_RULE_ALLOW = "allow"  # the rule of the sensitive names is skipped (the text detectors still run)
MASK_RULES = (MASK_RULE_DROP, MASK_RULE_REDACT, MASK_RULE_ALLOW)
