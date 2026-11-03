"""The AWS HTTP destination: each ``log.*`` event is ONE add_log request to aws_microservice (T-04.6, ADR-008).

It satisfies both the log sink port and the event sink port. The add_log contract has no place for the other event
types, so they are skipped and counted. It does NOT queue and does NOT retry on its own; a failure is a value, never an
exception for the caller (ADR-005). It does no I/O itself: all of it is in the ``external/`` folder of the shared
client.
"""

from dataclasses import dataclass
from typing import Optional, Sequence

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.aws_http.error.aws_http_errors import (
    REASON_DESTINATION_FAILED,
    REASON_SERIALIZATION_FAILED,
)
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.build_add_log_request import (
    build_add_log_request,
)
from telemetry.internal.infrastructure.outbound.logging.aws_http.settings import Settings, validate_settings
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.classify_response import (
    CLASS_PERMANENT,
    CLASS_SUCCESS,
)
from telemetry.internal.infrastructure.outbound.shared.http_client.http_client import HttpClient, Outcome

LOG_EVENT_PREFIX = "log."


@dataclass(frozen=True)
class Delivery:
    """What ``deliver`` knows about one call: the port result, the last outcome and the counts.

    ``outcome`` is the outcome of the LAST request that was made, and None when no request was made (every event was
    skipped, none could be built, or there were none). ``sent`` is the number of requests made, one per translated
    event, and ``skipped`` the number of events that are not ``log.*`` events. When something unexpected happens the
    counts are zero and the result is ``destination_failed``.
    """

    result: SendResult
    outcome: Optional[Outcome]
    sent: int
    skipped: int


class AwsHttpDestination:
    """AwsHttpDestination sends each ``log.*`` event as ONE add_log request and classifies what comes back.

    An event that is not a ``log.*`` event is not sent and it counts as ACCEPTED. It holds no queue and no thread and
    it is safe for concurrent use: the settings are never changed after the construction.
    """

    def __init__(self, settings: Settings, limits: Limits = Limits()) -> None:
        """Validate the settings (a ConfigurationError holds every problem), then build the shared client."""
        problems = validate_settings(settings)
        if problems:
            raise ConfigurationError(problems)
        self._client = HttpClient(settings.client_settings(), limits)
        self._log_group_name = settings.log_group_name
        self._log_stream_name = settings.log_stream_name

    def send(self, events: Sequence[Envelope]) -> SendResult:
        """Send the events and say how many were accepted, dropped or failed; it never raises (ADR-005)."""
        return self.deliver(events).result

    def deliver(self, events: Sequence[Envelope]) -> Delivery:
        """Send one request per ``log.*`` event and return the result with the last outcome and the counts; never raise.

        The other events are skipped and count as accepted. A 2xx accepts the event, a transient outcome FAILS it (the
        worker may retry it) and a permanent one DROPS it; an event that cannot be built is dropped as
        ``serialization_failed``. The reason of the result is the one of the first event that was not accepted. If
        something unexpected happens every event fails with ``destination_failed``. KeyboardInterrupt and the other
        exceptions that are not an ``Exception`` propagate.
        """
        try:
            accepted = dropped = failed = sent = skipped = 0
            reason = ""
            last_outcome: Optional[Outcome] = None
            for event in list(events):
                if not event.event_type.startswith(LOG_EVENT_PREFIX):
                    skipped += 1
                    accepted += 1
                    continue
                try:
                    body = build_add_log_request(event, self._log_group_name, self._log_stream_name)
                except Exception:  # an event that cannot be built never gets better by retrying
                    dropped += 1
                    reason = reason or REASON_SERIALIZATION_FAILED
                    continue
                last_outcome = self._client.send(body)
                sent += 1
                if last_outcome.klass == CLASS_SUCCESS:
                    accepted += 1
                    continue
                if last_outcome.klass == CLASS_PERMANENT:
                    dropped += 1
                else:
                    failed += 1
                reason = reason or last_outcome.reason
            return Delivery(SendResult(accepted, dropped, failed, reason), last_outcome, sent, skipped)
        except Exception:
            return Delivery(self._failed(events), None, 0, 0)

    def next_delay(self, attempt: int, outcome: Outcome, random_value: float) -> tuple[int, bool]:
        """Return ``(delay in milliseconds, retry)`` after the attempt number ``attempt`` (0 for the first).

        It delegates to the shared client: the worker of T-05.3 decides with it, this destination never retries.
        """
        return self._client.next_delay(attempt, outcome, random_value)

    @staticmethod
    def _failed(events: Sequence[Envelope]) -> SendResult:
        try:
            count = len(events)
        except Exception:
            count = 0
        return SendResult(accepted=0, dropped=0, failed=count, reason=REASON_DESTINATION_FAILED)
