"""The service that applies the size limits to an envelope, after masking and before any sink."""

from telemetry.internal.application.dtos.limits_result import LimitsResult
from telemetry.internal.application.errors import EnvelopeLimitsFailedError
from telemetry.internal.domain.operations.limits.apply_limits import apply_limits
from telemetry.internal.domain.operations.limits.report import LimitsReport
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.limits import Limits


class LimitsService:
    """LimitsService applies a Limits to an envelope.

    It runs AFTER masking (a secret is never cut in half and left half visible, ADR-006) and BEFORE the sinks; T-03.9
    wires the chain. The counters come back with the result and are exposed as diagnostics by T-05.5.
    """

    def __init__(self, limits: Limits) -> None:
        """Keep the limits; the composition root builds them from the configuration."""
        self._limits = limits

    def apply(self, envelope: Envelope) -> LimitsResult:
        """Return the limited COPY of the envelope and the counters, or the error; it never raises (ADR-005).

        A too-large event comes back in ``error`` (EventTooLargeError, code ``limit_exceeded``) with no envelope and
        the counters kept. Any other failure is an EnvelopeLimitsFailedError chained from the cause, with no envelope
        and a zero report. No message holds a value of the event.
        """
        try:
            outcome = apply_limits(envelope, self._limits)
            return LimitsResult(envelope=outcome.envelope, report=outcome.report, error=outcome.error)
        except Exception as error:  # the last resort of ADR-005: nothing escapes to the caller of a log call
            failure = EnvelopeLimitsFailedError("the limits could not be applied to the event")
            failure.__cause__ = error
            return LimitsResult(report=LimitsReport(), error=failure)
