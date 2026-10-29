"""An instant in UTC and the one fixed time format of the contract (ADR-012)."""

import datetime
import re
from dataclasses import dataclass

from telemetry.internal.domain.errors import InstantFormatInvalidError, NonUtcTimeError

INSTANT_PATTERN = re.compile(r"([0-9]{4})-([0-9]{2})-([0-9]{2})T([0-9]{2}):([0-9]{2}):([0-9]{2})\.([0-9]{6})Z")


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

    @classmethod
    def parse(cls, text: str) -> "UtcInstant":
        """Parse exactly YYYY-MM-DDTHH:MM:SS.ffffffZ (six decimals, Z); raise InstantFormatInvalidError otherwise.

        The message never holds the text. It does not rely on fromisoformat, which reads the Z only from Python 3.11.
        """
        match = INSTANT_PATTERN.fullmatch(text) if isinstance(text, str) else None
        if match is None:
            raise InstantFormatInvalidError("the text is not in the format YYYY-MM-DDTHH:MM:SS.ffffffZ")
        year, month, day, hour, minute, second, microsecond = (int(part) for part in match.groups())
        try:
            value = datetime.datetime(year, month, day, hour, minute, second, microsecond, tzinfo=datetime.timezone.utc)
        except ValueError as error:
            raise InstantFormatInvalidError("the text is not a real date and time") from error
        return cls(value)

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
