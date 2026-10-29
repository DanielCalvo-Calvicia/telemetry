"""What a clean-up of the local spool removed."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class SpoolCleanup:
    """SpoolCleanup is the number of events removed and the bytes freed by one clean-up."""

    removed_events: int
    freed_bytes: int
