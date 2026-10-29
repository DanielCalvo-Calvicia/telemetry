"""The result of handing a batch of events to a sink."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class SendResult:
    """SendResult says what became of the events of one call: accepted, dropped or failed.

    Each event of the call is counted once, so the three counts add up to the size of the batch. ``reason`` is empty
    when nothing was dropped or failed. A sink reports a failure here and never raises (ADR-005, REQ-DLV-005).
    """

    accepted: int
    dropped: int
    failed: int
    reason: str = ""
