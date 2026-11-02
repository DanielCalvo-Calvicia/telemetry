"""The HTTP destination: batches of events sent to the URL of the consumer (REQ-OUT-002, REQ-OUT-006).

It satisfies both the log sink port and the event sink port and adds only the semantics of the payload on top of the
shared HTTP client: the minimum severity and the mapping of the outcome to the port result. It does NOT queue and does
NOT retry on its own (the queue is T-05.1 and the retry is T-05.3); a failure is a value, never an exception for the
caller (ADR-005). It does no I/O itself: all of it is in the ``external/`` folder of the shared client.
"""

from dataclasses import dataclass
from typing import Optional, Sequence

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.domain.value_objects.severity import parse_severity, severity_at_least
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.http.error.http_errors import REASON_DESTINATION_FAILED
from telemetry.internal.infrastructure.outbound.logging.http.helpers.result_from_outcome import result_from_outcome
from telemetry.internal.infrastructure.outbound.logging.http.settings import Settings, validate_settings
from telemetry.internal.infrastructure.outbound.shared.http_client.http_client import HttpClient, Outcome


@dataclass(frozen=True)
class Delivery:
    """What ``deliver`` knows about one call: the port result, the outcome of the request and the counts.

    ``outcome`` is None when no request was made (every event was filtered, or there were none). ``sent`` is the
    number of events in the request and ``filtered`` the number that were below the minimum severity. When something
    unexpected happens the counts are zero and the result is ``destination_failed``.
    """

    result: SendResult
    outcome: Optional[Outcome]
    sent: int
    filtered: int


class HttpDestination:
    """HttpDestination sends the events of each call as ONE batch request and classifies what comes back.

    An event below the minimum severity is not sent and it counts as ACCEPTED. It holds no queue and no thread and it
    is safe for concurrent use: the settings are never changed after the construction.
    """

    def __init__(self, settings: Settings, limits: Limits = Limits()) -> None:
        """Validate the settings (a ConfigurationError holds every problem), then build the shared client."""
        problems = validate_settings(settings)
        if problems:
            raise ConfigurationError(problems)
        self._client = HttpClient(settings.client_settings(), limits)
        self._minimum = parse_severity(settings.minimum_severity)

    def send(self, events: Sequence[Envelope]) -> SendResult:
        """Send the events and say how many were accepted, dropped or failed; it never raises (ADR-005)."""
        return self.deliver(events).result

    def deliver(self, events: Sequence[Envelope]) -> Delivery:
        """Send the events in ONE request and return the result with the outcome and the counts; never raise.

        The events below the minimum severity are not sent and count as accepted. When none is left no request is
        made. A transient outcome FAILS the sent events (the worker may retry them) and a permanent one DROPS them. If
        something unexpected happens every event fails with ``destination_failed``. KeyboardInterrupt and the other
        exceptions that are not an ``Exception`` propagate.
        """
        try:
            batch = list(events)
            to_send = [event for event in batch if severity_at_least(event.severity, self._minimum)]
            filtered = len(batch) - len(to_send)
            if not to_send:
                return Delivery(SendResult(accepted=filtered, dropped=0, failed=0), None, 0, filtered)
            outcome = self._client.send_batch(to_send)
            return Delivery(result_from_outcome(outcome, len(to_send), filtered), outcome, len(to_send), filtered)
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
