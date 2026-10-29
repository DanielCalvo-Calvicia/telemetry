"""An instant in UTC and the one fixed time format of the contract (ADR-012)."""

import datetime
from dataclasses import dataclass

from telemetry.internal.domain.errors import NonUtcTimeError


@dataclass(frozen=True, slots=True)
class UtcInstant:
    """A timezone-aware datetime whose offset is zero.

    A naive datetime, or one with another offset, is rejected and never converted. A datetime cannot hold a year
    outside 0001 to 9999, so that bound of the contract holds by construction. The precision is the microsecond.
    """

    value: datetime.datetime

    def __post_init__(self) -> None:
        """Validate the datetime; raise NonUtcTimeError."""
        if not isinstance(self.value, datetime.datetime):
            raise NonUtcTimeError("the instant is not a datetime")
        offset = self.value.utcoffset()
        if offset is None:
            raise NonUtcTimeError("the datetime has no time zone")
        if offset != datetime.timedelta(0):
            raise NonUtcTimeError(f"the offset is {offset}")

    def format(self) -> str:
        """Format as YYYY-MM-DDTHH:MM:SS.ffffffZ: six decimals always kept, and the Z."""
        instant = self.value
        return (
            f"{instant.year:04d}-{instant.month:02d}-{instant.day:02d}"
            f"T{instant.hour:02d}:{instant.minute:02d}:{instant.second:02d}.{instant.microsecond:06d}Z"
        )

    def __str__(self) -> str:
        """Return the fixed format."""
        return self.format()
