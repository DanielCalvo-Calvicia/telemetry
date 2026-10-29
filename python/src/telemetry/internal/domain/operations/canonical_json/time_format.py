"""The fixed time format of the contract (ADR-012): YYYY-MM-DDTHH:MM:SS.ffffffZ."""

import datetime

from telemetry.internal.domain.value_objects.utc_instant import UtcInstant


def format_utc_time(instant: datetime.datetime) -> str:
    """Format an instant as YYYY-MM-DDTHH:MM:SS.ffffffZ: UTC, six decimals kept, always the Z.

    A naive datetime, or one whose offset is not zero, is rejected with the code event_schema_invalid; it is never
    converted silently. A datetime keeps microseconds, so nothing is rounded here (Go truncates its nanoseconds). The
    format itself lives in UtcInstant.
    """
    return UtcInstant(instant).format()
