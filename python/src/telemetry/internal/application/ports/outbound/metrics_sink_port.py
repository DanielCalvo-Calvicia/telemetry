"""The metrics sink port."""

from typing import Protocol, Sequence, runtime_checkable

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@runtime_checkable
class MetricsSinkPort(Protocol):
    """MetricsSinkPort is what the application requires of a sink that serves the metrics capability.

    It receives events already validated and already masked. It never raises to the caller: a failure is in the
    result (ADR-005, REQ-DLV-005). Its shape is shared with the other sink ports (REQ-OUT-001).
    """

    def send(self, events: Sequence[Envelope]) -> SendResult:
        """Send the batch and say how many events were accepted, dropped or failed; never raise."""
