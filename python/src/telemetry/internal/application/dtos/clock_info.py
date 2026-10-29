"""What a clock knows about itself."""

from dataclasses import dataclass
from typing import Optional


@dataclass(frozen=True, slots=True)
class ClockInfo:
    """ClockInfo carries the clock_source and clock_synced of an event, when the clock knows them.

    ``None`` means not known; it matches the SourceKnown and SyncedKnown flags of the Go half.
    """

    source: Optional[str] = None
    synced: Optional[bool] = None
