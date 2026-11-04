"""The exponential backoff with equal jitter and the decision to retry (pure: no clock, no random source)."""

import math
from typing import Optional

# The longest Retry-After that is honored: one day. A longer hint is cut, so a destination cannot make a worker wait
# for ever.
MAX_RETRY_AFTER_MILLISECONDS = 86_400_000


def backoff_with_jitter(attempt: int, base_ms: int, max_ms: int, random_value: float) -> int:
    """Return the wait in milliseconds before the retry that follows ``attempt`` (0 for the first retry).

    ``ceiling = min(max_ms, base_ms * 2^attempt)`` and ``delay = floor(ceiling / 2 + random_value * ceiling / 2)``: half
    fixed and half random (the equal jitter). ``random_value`` is cut to [0, 1]. The doubling stops at the maximum, so
    a large attempt never makes a huge number.
    """
    ceiling = base_ms
    for _ in range(attempt):
        if ceiling >= max_ms:
            break
        ceiling *= 2
    ceiling = min(ceiling, max_ms)
    if not random_value > 0:
        random_value = 0.0
    elif random_value > 1:
        random_value = 1.0
    half = ceiling / 2
    return math.floor(half + random_value * half)


def next_delay(attempt: int, base_ms: int, max_ms: int, retry_after_ms: Optional[int], random_value: float) -> int:
    """Return the wait in milliseconds before the next attempt: the larger of the backoff and the Retry-After.

    ``retry_after_ms`` is the hint of the destination, cut to ``MAX_RETRY_AFTER_MILLISECONDS``; ``None``, zero or a
    negative number is no hint.
    """
    delay = backoff_with_jitter(attempt, base_ms, max_ms, random_value)
    if retry_after_ms is not None:
        delay = max(delay, min(retry_after_ms, MAX_RETRY_AFTER_MILLISECONDS))
    return delay


def retry_delay(
    attempt: int,
    max_attempts: int,
    transient: bool,
    retry_after_ms: Optional[int],
    base_ms: int,
    max_ms: int,
    random_value: float,
) -> tuple[int, bool]:
    """Return ``(delay in milliseconds, retry)`` for the attempt that just ended (0 for the first).

    It retries only a transient failure and only while ``attempt + 1 < max_attempts``. The delay is ``next_delay``.
    When it does not retry the delay is 0.
    """
    if not transient or attempt + 1 >= max_attempts:
        return 0, False
    return next_delay(attempt, base_ms, max_ms, retry_after_ms, random_value), True
