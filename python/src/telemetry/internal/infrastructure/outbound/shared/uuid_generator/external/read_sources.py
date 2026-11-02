"""The calls to the clock and to the random source of the machine."""

import os
import time


def read_milliseconds() -> int:
    """Return the unix time in milliseconds."""
    return time.time_ns() // 1_000_000


def read_random(byte_count: int) -> bytes:
    """Return bytes from the random source of the operating system; there is no weaker fallback."""
    return os.urandom(byte_count)
