"""One diagnostic: a fact about the delivery, never an event of telemetry."""

from dataclasses import dataclass

from telemetry.internal.domain.value_objects.enums_generated import DiagnosticCode


@dataclass(frozen=True, slots=True)
class DiagnosticEvent:
    """DiagnosticEvent says that something went wrong in the delivery of one destination (REQ-DGN-002).

    It carries counters and identifiers and NEVER the message or a field of an event, a token or a URL (ADR-005).
    ``kind`` is the kind of the destination (``log``, ``event``, ...) and ``index`` its position inside the kind;
    ``reason`` is a short code of ``[A-Za-z0-9_.:-]`` of at most 64 characters; ``count`` is how many events the
    diagnostic is about and ``suppressed`` how many diagnostics the rate limit held back before this one. ``at_ms`` is
    the wall clock in unix milliseconds. A diagnostic is not an event: it never goes through the pipeline, the queues,
    the masker or a destination.
    """

    code: DiagnosticCode
    kind: str
    index: int
    reason: str
    count: int
    suppressed: int = 0
    at_ms: int = 0
