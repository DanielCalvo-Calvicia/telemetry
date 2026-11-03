"""The result of a shutdown."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class ShutdownResult:
    """ShutdownResult says what became of the events still waiting when the core was shut down.

    ``sent`` were delivered, ``spooled`` were kept in the local spool and ``dropped`` were lost. The counts are the
    events that were queued or being sent when ``flush`` or ``shutdown`` was called; ``spooled`` is 0 until T-06.
    """

    sent: int = 0
    spooled: int = 0
    dropped: int = 0
