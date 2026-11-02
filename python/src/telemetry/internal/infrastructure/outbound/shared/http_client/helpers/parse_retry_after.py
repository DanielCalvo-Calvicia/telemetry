"""The delay asked by a Retry-After header."""

import datetime
import re
from typing import Optional

MAX_RETRY_AFTER_SECONDS = 86400

_SECONDS = re.compile(r"[0-9]+", re.ASCII)
_DAYS = "Mon|Tue|Wed|Thu|Fri|Sat|Sun"
_MONTHS = ("Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec")
_TIME = r"(\d\d):(\d\d):(\d\d)"
# IMF-fixdate: Www, DD Mon YYYY HH:MM:SS GMT
_IMF_FIXDATE = re.compile(rf"(?:{_DAYS}), (\d\d) ({'|'.join(_MONTHS)}) (\d{{4}}) {_TIME} GMT", re.ASCII)
# asctime: Www Mon  D HH:MM:SS YYYY (the day is padded with a space or has two digits)
_ASCTIME = re.compile(rf"(?:{_DAYS}) ({'|'.join(_MONTHS)}) ( \d|\d\d) {_TIME} (\d{{4}})", re.ASCII)


def _http_date(text: str) -> Optional[datetime.datetime]:
    """Read the two accepted date forms, in English and always UTC; anything else, or a bad date, gives None."""
    match = _IMF_FIXDATE.fullmatch(text)
    if match:
        day, month, year, hour, minute, second = match.groups()
    else:
        match = _ASCTIME.fullmatch(text)
        if not match:
            return None
        month, day, hour, minute, second, year = match.groups()
    try:
        return datetime.datetime(
            int(year),
            _MONTHS.index(month) + 1,
            int(day),
            int(hour),
            int(minute),
            int(second),
            tzinfo=datetime.timezone.utc,
        )
    except ValueError:
        return None


def parse_retry_after(header: str, now: datetime.datetime) -> Optional[int]:
    """Return the milliseconds asked by a Retry-After value, or None when it asks nothing valid.

    The value is a number of seconds or an HTTP date: ONLY the IMF-fixdate (``Www, DD Mon YYYY HH:MM:SS GMT``) or the
    asctime form (``Www Mon  D HH:MM:SS YYYY``), in English and read as UTC. The weekday need not match the date.
    A date in the past gives 0.
    The result is cut to one day so a server cannot make the library wait for ever.
    """
    text = header.strip()
    if _SECONDS.fullmatch(text):
        digits = text.lstrip("0") or "0"
        seconds = MAX_RETRY_AFTER_SECONDS if len(digits) > 9 else min(int(digits), MAX_RETRY_AFTER_SECONDS)
        return seconds * 1000
    moment = _http_date(text)
    if moment is None:
        return None
    if now.tzinfo is None:
        now = now.replace(tzinfo=datetime.timezone.utc)
    milliseconds = int((moment - now).total_seconds() * 1000)
    return min(max(milliseconds, 0), MAX_RETRY_AFTER_SECONDS * 1000)
