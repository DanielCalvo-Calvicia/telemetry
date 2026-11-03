"""The milliseconds since the epoch of an ``occurred_at`` text, with integer arithmetic only (pure)."""

import datetime
import re

_OCCURRED_AT = re.compile(r"(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?Z", re.ASCII)
_EPOCH_ORDINAL = datetime.date(1970, 1, 1).toordinal()


def timestamp_milliseconds(occurred_at: str) -> int:
    """Return the instant of ``YYYY-MM-DDTHH:MM:SS[.fraction]Z`` in milliseconds since the epoch, CUT, never rounded.

    It raises ValueError when the text is not in that form or is not a real date and time.
    """
    match = _OCCURRED_AT.fullmatch(occurred_at)
    if match is None:
        raise ValueError("occurred_at is not an RFC 3339 UTC time")
    year, month, day, hour, minute, second = (int(match.group(index)) for index in range(1, 7))
    if hour > 23 or minute > 59 or second > 59:
        raise ValueError("occurred_at is not a real time")
    days = datetime.date(year, month, day).toordinal() - _EPOCH_ORDINAL
    milliseconds = int((match.group(7) or "").ljust(3, "0")[:3])
    return ((days * 24 + hour) * 60 + minute) * 60_000 + second * 1000 + milliseconds
