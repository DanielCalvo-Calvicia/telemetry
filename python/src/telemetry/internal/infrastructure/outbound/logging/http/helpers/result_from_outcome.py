"""The mapping from the outcome of one request to the result of the port (pure)."""

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.classify_response import (
    CLASS_PERMANENT,
    CLASS_SUCCESS,
)
from telemetry.internal.infrastructure.outbound.shared.http_client.http_client import Outcome


def result_from_outcome(outcome: Outcome, sent: int, filtered: int) -> SendResult:
    """Return the result of a call that sent ``sent`` events in one request and filtered ``filtered`` events.

    The filtered events are ACCEPTED (the destination took them and its own policy decided not to send them). A
    success accepts the sent events too. A permanent rejection DROPS them and keeps the reason (they are not retried).
    Any other class is transient: they FAIL with the reason and the Retry-After of the outcome (0 when the server
    gave none) and the worker may retry them.
    """
    if outcome.klass == CLASS_SUCCESS:
        return SendResult(accepted=filtered + sent, dropped=0, failed=0)
    if outcome.klass == CLASS_PERMANENT:
        return SendResult(accepted=filtered, dropped=sent, failed=0, reason=outcome.reason)
    return SendResult(
        accepted=filtered,
        dropped=0,
        failed=sent,
        reason=outcome.reason,
        retry_after_ms=outcome.retry_after_milliseconds or 0,
    )
