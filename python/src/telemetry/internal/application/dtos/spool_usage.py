"""How full the local spool is."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class SpoolUsage:
    """SpoolUsage is the number of events and bytes held by the spool and its quota in bytes."""

    events: int
    bytes: int
    quota_bytes: int
