"""The clock port."""

from typing import Protocol, runtime_checkable

from telemetry.internal.application.dtos.clock_info import ClockInfo
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant


@runtime_checkable
class ClockPort(Protocol):
    """ClockPort is what the application requires to know the time: the wall time and a monotonic reading."""

    def now(self) -> UtcInstant:
        """Return the wall time as an instant in UTC."""

    def monotonic_nanoseconds(self) -> int:
        """Return a reading from an arbitrary fixed point that never goes back; two readings give elapsed time."""

    def clock_info(self) -> ClockInfo:
        """Return the clock source and whether it is synchronised, when known."""
