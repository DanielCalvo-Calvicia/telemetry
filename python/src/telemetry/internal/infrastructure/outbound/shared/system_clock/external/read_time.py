"""The calls to the clock of the machine."""

import datetime
import time


def read_time() -> datetime.datetime:
    """Return the wall time of the machine as an aware datetime in UTC."""
    return datetime.datetime.now(datetime.timezone.utc)


def read_monotonic() -> int:
    """Return the monotonic clock of the machine in nanoseconds; it never goes back."""
    return time.monotonic_ns()
