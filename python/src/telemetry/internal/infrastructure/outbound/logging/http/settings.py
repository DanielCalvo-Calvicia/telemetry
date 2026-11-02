"""The settings of the HTTP destination: the Python form of the two schemas that describe them.

They are the settings of the shared client (contracts/config/http_client.schema.json) followed by ``minimum_severity``
(contracts/config/http_destination.schema.json). The consumer loads these values with its own tool, using the names of
``HTTP_DESTINATION_ENVIRONMENT_VARIABLES``, and passes them in. THIS PACKAGE NEVER READS THE ENVIRONMENT OR A FILE
(ADR-004). The attribute names are exactly the property names of the schemas, so ``dataclasses.asdict(settings)`` is
the flat object of twelve keys. ``bearer_token`` and ``api_key_value`` are secrets (REQ-SEC-007).
"""

from dataclasses import dataclass

from telemetry.internal.domain.rules import (
    DEFAULT_CONSOLE_MINIMUM_SEVERITY,
    DEFAULT_HTTP_MAX_ATTEMPTS,
    DEFAULT_RETRY_BASE_DELAY_MILLISECONDS,
    DEFAULT_RETRY_MAX_DELAY_MILLISECONDS,
    DEFAULT_TIMEOUT_MILLISECONDS,
)
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.infrastructure.config.errors import ConfigProblem
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import HttpSettings
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import (
    validate_settings as validate_client_settings,
)


@dataclass(frozen=True)
class Settings:
    """The settings of ONE HTTP destination: the eleven of the shared client and the ``minimum_severity``.

    An empty text is 'not set'; ``url`` is required (none is fixed).
    """

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
    minimum_severity: str = DEFAULT_CONSOLE_MINIMUM_SEVERITY

    def client_settings(self) -> HttpSettings:
        """Return the settings of the shared client: the same eleven fields, without the minimum severity."""
        return HttpSettings(
            url=self.url,
            method=self.method,
            timeout_ms=self.timeout_ms,
            compress=self.compress,
            skip_tls_verification=self.skip_tls_verification,
            bearer_token=self.bearer_token,
            api_key_header=self.api_key_header,
            api_key_value=self.api_key_value,
            max_attempts=self.max_attempts,
            retry_base_delay_ms=self.retry_base_delay_ms,
            retry_max_delay_ms=self.retry_max_delay_ms,
        )


def validate_settings(settings: Settings) -> tuple[ConfigProblem, ...]:
    """Return every problem of the settings: the ones of the shared client first, then the minimum severity.

    An empty tuple means they are valid. A reason is a fixed text: it never holds a configured value. A value of a
    wrong type is a problem, not an exception.
    """
    problems = list(validate_client_settings(settings.client_settings()))
    severities = tuple(member.value for member in Severity)
    minimum = settings.minimum_severity
    if not (type(minimum) is str and minimum in severities):
        problems.append(
            ConfigProblem(
                "minimum_severity",
                ERROR_CODE_CONFIG_FIELD_INVALID,
                "the minimum severity must be TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL",
            )
        )
    return tuple(problems)
