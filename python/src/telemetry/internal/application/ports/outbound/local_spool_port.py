"""The local spool port."""

from typing import Protocol, Sequence, runtime_checkable

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.dtos.spool_cleanup import SpoolCleanup
from telemetry.internal.application.dtos.spool_record import SpoolRecord
from telemetry.internal.application.dtos.spool_usage import SpoolUsage
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@runtime_checkable
class LocalSpoolPort(Protocol):
    """LocalSpoolPort is what the application requires to keep events on the device until they are delivered.

    It is the contract only; the adapter comes in T-06. The methods that return no result may raise.
    """

    def append(self, events: Sequence[Envelope]) -> SendResult:
        """Store the events; say how many were accepted, dropped or failed."""

    def read_batch(self, maximum_events: int) -> list[SpoolRecord]:
        """Return up to maximum_events records not yet acknowledged, oldest first."""

    def acknowledge(self, record_ids: Sequence[str]) -> None:
        """Mark the records as delivered so that they are no longer read."""

    def usage(self) -> SpoolUsage:
        """Return the events and bytes held and the quota."""

    def cleanup(self) -> SpoolCleanup:
        """Remove what is no longer needed and say what was removed."""
