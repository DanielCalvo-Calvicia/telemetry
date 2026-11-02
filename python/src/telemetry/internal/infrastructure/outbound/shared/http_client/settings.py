"""The settings of the shared HTTP client: the Python form of contracts/config/http_client.schema.json.

The consumer loads these values with its own tool, using the names of ``HTTP_ENVIRONMENT_VARIABLES``, and passes them
in. THIS PACKAGE NEVER READS THE ENVIRONMENT OR A FILE (ADR-004). The attribute names are exactly the property names
of the schema, so ``dataclasses.asdict(settings)`` has the shape of the schema. An empty text means 'not set'.
``bearer_token`` and ``api_key_value`` are secrets: they have no default and they never appear in an error, a log or a
diagnostic (REQ-SEC-007).
"""

import re
from dataclasses import dataclass

from telemetry.internal.domain.rules import (
    DEFAULT_HTTP_MAX_ATTEMPTS,
    DEFAULT_RETRY_BASE_DELAY_MILLISECONDS,
    DEFAULT_RETRY_MAX_DELAY_MILLISECONDS,
    DEFAULT_TIMEOUT_MILLISECONDS,
)
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.infrastructure.config.errors import ConfigProblem
from telemetry.internal.infrastructure.outbound.shared.http_client.external.parse_url import parse_url

METHODS = ("POST", "PUT", "PATCH")
LOCAL_HOSTS = ("localhost", "127.0.0.1", "::1")

_HEADER_TOKEN = re.compile(r"[!#$%&'*+.^_`|~0-9A-Za-z-]+", re.ASCII)
_VISIBLE_ASCII = re.compile(r"[\x21-\x7e]+", re.ASCII)


@dataclass(frozen=True)
class HttpSettings:
    """The settings of ONE HTTP destination; an empty text is 'not set' and ``url`` is required (none is fixed)."""

    url: str = ""
    method: str = "POST"
    timeout_ms: int = DEFAULT_TIMEOUT_MILLISECONDS
    compress: bool = False
    skip_tls_verification: bool = False
    bearer_token: str = ""
    api_key_header: str = ""
    api_key_value: str = ""
    max_attempts: int = DEFAULT_HTTP_MAX_ATTEMPTS
    retry_base_delay_ms: int = DEFAULT_RETRY_BASE_DELAY_MILLISECONDS
    retry_max_delay_ms: int = DEFAULT_RETRY_MAX_DELAY_MILLISECONDS


def _positive(value: object) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value > 0


def _text(value: object) -> bool:
    return isinstance(value, str)


def _is_set(value: object) -> bool:
    return value != ""


def _visible(value: object) -> bool:
    return isinstance(value, str) and _VISIBLE_ASCII.fullmatch(value) is not None


def _url_problem(settings: HttpSettings) -> str:
    """Return the fixed reason of the problem of the url, or an empty text when it is valid."""
    parts = parse_url(settings.url) if _text(settings.url) else None
    if parts is None:
        return "the url is required and must be a valid http or https url"
    scheme, host = parts
    if scheme not in ("http", "https") or host == "":
        return "the url is required and must be a valid http or https url"
    authenticated = any(
        _is_set(value) for value in (settings.bearer_token, settings.api_key_header, settings.api_key_value)
    )
    if scheme == "http" and authenticated and host not in LOCAL_HOSTS:
        return "authentication over plain http is refused except to localhost"
    return ""


def validate_settings(settings: HttpSettings) -> tuple[ConfigProblem, ...]:
    """Return every problem of the settings, in the order of the schema; an empty tuple means they are valid.

    A reason is a fixed text: it never holds a configured value (a secret or a URL).
    """
    problems: list[ConfigProblem] = []

    def invalid(field_name: str, reason: str) -> None:
        problems.append(ConfigProblem(field_name, ERROR_CODE_CONFIG_FIELD_INVALID, reason))

    reason = _url_problem(settings)
    if reason:
        invalid("url", reason)
    if settings.method not in METHODS:
        invalid("method", "the method must be POST, PUT or PATCH")
    if not _positive(settings.timeout_ms):
        invalid("timeout_ms", "it must be an integer greater than zero")

    bearer_set = _is_set(settings.bearer_token)
    if not _text(settings.bearer_token) or (bearer_set and not _visible(settings.bearer_token)):
        invalid("bearer_token", "it must be visible ASCII characters only, with no space or control character")

    header, key_value = settings.api_key_header, settings.api_key_value
    if not _text(header) or (_is_set(header) and _HEADER_TOKEN.fullmatch(header) is None):
        invalid("api_key_header", "it must be a valid HTTP header name")
    elif _is_set(key_value) and not _is_set(header):
        invalid("api_key_header", "it is required when an API key value is set")
    elif bearer_set and (_is_set(header) or _is_set(key_value)):
        invalid("api_key_header", "only one authentication is allowed: a bearer token or an API key")
    if not _text(key_value) or (_is_set(key_value) and not _visible(key_value)):
        invalid("api_key_value", "it must be visible ASCII characters only, with no space or control character")
    elif _is_set(header) and not _is_set(key_value):
        invalid("api_key_value", "it is required when an API key header is set")

    if not _positive(settings.max_attempts):
        invalid("max_attempts", "it must be an integer greater than zero")
    base_valid = _positive(settings.retry_base_delay_ms)
    if not base_valid:
        invalid("retry_base_delay_ms", "it must be an integer greater than zero")
    if not _positive(settings.retry_max_delay_ms):
        invalid("retry_max_delay_ms", "it must be an integer greater than zero")
    elif base_valid and settings.retry_max_delay_ms < settings.retry_base_delay_ms:
        invalid("retry_max_delay_ms", "it must not be less than the base delay")

    return tuple(problems)
