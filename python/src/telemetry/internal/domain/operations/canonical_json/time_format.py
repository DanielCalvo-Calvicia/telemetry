"""The fixed time format of the contract (ADR-012): YYYY-MM-DDTHH:MM:SS.ffffffZ."""

import datetime

from telemetry.internal.domain.errors import NonUtcTimeError


def format_utc_time(instant: datetime.datetime) -> str:
    """Format an instant as YYYY-MM-DDTHH:MM:SS.ffffffZ: UTC, six decimals kept, always the Z.

    A naive datetime, or one whose offset is not zero, is rejected with the code event_schema_invalid; it is never
    converted silently. A datetime keeps microseconds, so nothing is rounded here (Go truncates its nanoseconds).
    """
    offset = instant.utcoffset()
    if offset is None:
        raise NonUtcTimeError("the datetime has no time zone")
    if offset != datetime.timedelta(0):
        raise NonUtcTimeError(f"the offset is {offset}")
    return (
        f"{instant.year:04d}-{instant.month:02d}-{instant.day:02d}"
        f"T{instant.hour:02d}:{instant.minute:02d}:{instant.second:02d}.{instant.microsecond:06d}Z"
    )
