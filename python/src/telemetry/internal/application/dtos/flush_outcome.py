"""The result of the last flush or shutdown of one destination."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class FlushOutcome:
    """FlushOutcome is what the last ``flush`` or ``shutdown`` did for one destination.

    ``done`` is false until one has finished. ``sent`` were accepted by the destination, ``dropped`` were lost and
    ``spooled`` were kept in the local spool (0 until T-06). ``at_ms`` is the wall clock in unix milliseconds.
    """

    done: bool = False
    sent: int = 0
    spooled: int = 0
    dropped: int = 0
    at_ms: int = 0
