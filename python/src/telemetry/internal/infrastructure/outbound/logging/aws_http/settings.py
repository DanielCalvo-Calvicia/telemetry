"""The settings of the AWS HTTP adapter: the Python form of contracts/config/aws_http.schema.json.

The consumer loads these values with its own tool, using the names of ``AWS_HTTP_ENVIRONMENT_VARIABLES``, and passes
them in. THIS PACKAGE NEVER READS THE ENVIRONMENT OR A FILE (ADR-004). The attribute names are exactly the property
names of the schema, in its order, so ``dataclasses.asdict(settings)`` is the flat object of thirteen keys. The five
first have no default (the consumer must set them); ``bearer_token`` and ``api_key_value`` are secrets (REQ-SEC-007).
"""

import dataclasses
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
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.build_endpoint import build_endpoint
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import HttpSettings
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import (
    validate_settings as validate_client_settings,
)

_HOST = re.compile(r"https?://[^/?#\s]+", re.ASCII)
_PORT = re.compile(r"[0-9]+", re.ASCII)
_PLACEHOLDER_PORT = "80"
_PLACEHOLDER_PATH = "/"
_PLACEHOLDER_URL = "http://localhost"


@dataclass(frozen=True)
class Settings:
    """The settings of ONE AWS HTTP adapter: the five of the endpoint and the eight of the shared client.

    An empty text is 'not set'. ``host`` holds its scheme (``http://localhost``). The adapter always POSTs and never
    compresses, so those two settings of the shared client are not here.
    """

    host: str = ""
    port: str = ""
    path: str = ""
    log_group_name: str = ""
    log_stream_name: str = ""
    timeout_ms: int = DEFAULT_TIMEOUT_MILLISECONDS
    skip_tls_verification: bool = False
    bearer_token: str = ""
    api_key_header: str = ""
    api_key_value: str = ""
    max_attempts: int = DEFAULT_HTTP_MAX_ATTEMPTS
    retry_base_delay_ms: int = DEFAULT_RETRY_BASE_DELAY_MILLISECONDS
    retry_max_delay_ms: int = DEFAULT_RETRY_MAX_DELAY_MILLISECONDS

    def endpoint(self) -> str:
        """Return the URL of the add_log endpoint: host, ``:``, port and path."""
        return build_endpoint(self.host, self.port, self.path)

    def client_settings(self) -> HttpSettings:
        """Return the settings of the shared client: the endpoint, POST, no compression and the same eight fields."""
        return HttpSettings(
            url=self.endpoint(),
            method="POST",
            timeout_ms=self.timeout_ms,
            compress=False,
            skip_tls_verification=self.skip_tls_verification,
            bearer_token=self.bearer_token,
            api_key_header=self.api_key_header,
            api_key_value=self.api_key_value,
            max_attempts=self.max_attempts,
            retry_base_delay_ms=self.retry_base_delay_ms,
            retry_max_delay_ms=self.retry_max_delay_ms,
        )


def _valid_host(value: object) -> bool:
    return isinstance(value, str) and _HOST.fullmatch(value) is not None


def _valid_port(value: object) -> bool:
    if not isinstance(value, str) or _PORT.fullmatch(value) is None:
        return False
    digits = value.lstrip("0")
    return 0 < len(digits) <= 5 and int(digits) <= 65535


def _valid_path(value: object) -> bool:
    return isinstance(value, str) and value.startswith("/")


def _set(value: object) -> bool:
    return isinstance(value, str) and value != ""


def validate_settings(settings: Settings) -> tuple[ConfigProblem, ...]:
    """Return every problem of the settings, in the order of the schema; an empty tuple means they are valid.

    The host, the port, the path, the log group and the log stream come first; then the problems of the shared client
    (the one about authentication over plain http is reported on ``host``). A reason is a fixed text: it never holds a
    configured value (a host or a secret). A value of a wrong type is a problem, not an exception.
    """
    problems: list[ConfigProblem] = []

    def invalid(field_name: str, reason: str) -> None:
        problems.append(ConfigProblem(field_name, ERROR_CODE_CONFIG_FIELD_INVALID, reason))

    host_valid = _valid_host(settings.host)
    port_valid = _valid_port(settings.port)
    path_valid = _valid_path(settings.path)
    # The shared client judges the endpoint; a part that is wrong is replaced so that it judges the host only.
    url = _PLACEHOLDER_URL
    if host_valid:
        url = build_endpoint(
            settings.host,
            settings.port if port_valid else _PLACEHOLDER_PORT,
            settings.path if path_valid else _PLACEHOLDER_PATH,
        )
    client_problems = validate_client_settings(dataclasses.replace(settings.client_settings(), url=url))
    url_problems = [problem for problem in client_problems if problem.field == "url"]

    if not host_valid:
        invalid("host", "the host is required and must be http or https with a host name, such as http://localhost")
    elif url_problems:
        invalid("host", url_problems[0].reason)
    if not port_valid:
        invalid("port", "the port is required and must be a number from 1 to 65535")
    if not path_valid:
        invalid("path", "the path is required and must start with /")
    if not _set(settings.log_group_name):
        invalid("log_group_name", "the log group name is required")
    if not _set(settings.log_stream_name):
        invalid("log_stream_name", "the log stream name is required")
    problems.extend(problem for problem in client_problems if problem.field != "url")
    return tuple(problems)
