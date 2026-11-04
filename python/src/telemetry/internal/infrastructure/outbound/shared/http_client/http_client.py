"""The shared HTTP client: it sends ONE body and returns a classified outcome (REQ-OUT-005, REQ-OUT-006).

Rate limiting is NOT implemented here: a ``Retry-After`` of the server is honored through ``next_delay``, the delay the
caller waits before the next attempt. The client has no global state and is safe for concurrent use.
"""

import datetime
from dataclasses import dataclass
from typing import Optional, Sequence

from telemetry.internal.domain.operations.retry_delay.retry_delay import retry_delay
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.shared.http_client.external.post_request import post_request
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.auth_header import auth_header
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.build_batch_body import build_batch_body
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.classify_response import (
    CLASS_PERMANENT,
    CLASS_SUCCESS,
    CLASS_TRANSIENT,
    classify_status,
)
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.compress_gzip import compress_gzip
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.parse_retry_after import parse_retry_after
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import HttpSettings, validate_settings

REASON_BATCH_TOO_LARGE = "batch_too_large"
REASON_SERIALIZATION_FAILED = "serialization_failed"
REASON_CLIENT_FAILED = "client_failed"


@dataclass(frozen=True)
class Outcome:
    """The result of one send: its class, the status (0 when none arrived), the Retry-After and a fixed reason.

    ``reason`` is empty on success; otherwise it is ``status_<code>``, a failure of the transport (``timeout``,
    ``tls_verification_failed``, ``network_error``, ``response_unreadable``), ``batch_too_large``,
    ``serialization_failed`` or ``client_failed``. It never holds a configured value or the text of an exception.
    """

    klass: str
    status_code: int = 0
    retry_after_milliseconds: Optional[int] = None
    reason: str = ""


class HttpClient:
    """HttpClient sends one body to ONE destination and classifies what comes back; ``send`` never raises."""

    def __init__(self, settings: HttpSettings, limits: Limits) -> None:
        """Validate the settings first: invalid ones raise a ConfigurationError that holds every problem."""
        problems = validate_settings(settings)
        if problems:
            raise ConfigurationError(problems)
        self._settings = settings
        self._limits = limits

    @property
    def max_attempts(self) -> int:
        """The attempts at most for one batch, the first included."""
        return self._settings.max_attempts

    def send(self, body: bytes) -> Outcome:
        """Send one body: gzip it when the settings say so and return the outcome; it never raises.

        A plain body larger than the batch limit is a permanent ``batch_too_large`` and no network call is made.
        """
        try:
            if len(body) > self._limits.max_batch_bytes:
                return Outcome(CLASS_PERMANENT, reason=REASON_BATCH_TOO_LARGE)
            return self._post(body)
        except Exception:  # send never fails the caller (ADR-005)
            return Outcome(CLASS_TRANSIENT, reason=REASON_CLIENT_FAILED)

    def send_batch(self, events: Sequence[Envelope]) -> Outcome:
        """Build the batch body of the events and send it; a body that cannot be built is permanent."""
        try:
            body = build_batch_body(events)
        except Exception:  # an event that cannot be serialized never gets better by retrying
            return Outcome(CLASS_PERMANENT, reason=REASON_SERIALIZATION_FAILED)
        return self.send(body)

    def next_delay(self, attempt: int, outcome: Outcome, random_value: float) -> tuple[int, bool]:
        """Return ``(delay in milliseconds, retry)`` after the attempt number ``attempt`` (0 for the first)."""
        settings = self._settings
        return retry_delay(
            attempt,
            settings.max_attempts,
            outcome.klass == CLASS_TRANSIENT,
            outcome.retry_after_milliseconds,
            settings.retry_base_delay_ms,
            settings.retry_max_delay_ms,
            random_value,
        )

    def _post(self, body: bytes) -> Outcome:
        settings = self._settings
        headers = {"Content-Type": "application/json"}
        authentication = auth_header(settings.bearer_token, settings.api_key_header, settings.api_key_value)
        if authentication is not None:
            headers[authentication[0]] = authentication[1]
        payload = body
        if settings.compress:
            payload = compress_gzip(body)
            headers["Content-Encoding"] = "gzip"
        result = post_request(
            settings.url,
            settings.method,
            payload,
            headers,
            settings.timeout_ms / 1000,
            settings.skip_tls_verification,
        )
        if result.failure != "":
            return Outcome(CLASS_TRANSIENT, result.status_code, reason=result.failure)
        klass = classify_status(result.status_code)
        if klass == CLASS_SUCCESS:
            return Outcome(klass, result.status_code)
        retry_after = None
        if result.retry_after != "":
            retry_after = parse_retry_after(result.retry_after, datetime.datetime.now(datetime.timezone.utc))
        return Outcome(klass, result.status_code, retry_after, f"status_{result.status_code}")
