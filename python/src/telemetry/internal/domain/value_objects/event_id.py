"""The id of an event: a UUID of version 7. It is validated here and generated behind a port (T-03.2)."""

import re
from dataclasses import dataclass

from telemetry.internal.domain.errors import EventIdInvalidError

# Lowercase, hyphenated 8-4-4-4-12, version nibble 7, variant nibble 8, 9, a or b (RFC 4122).
UUID_V7_PATTERN = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}")


@dataclass(frozen=True, slots=True)
class EventId:
    """A validated UUIDv7 in its canonical lowercase text form."""

    value: str

    def __post_init__(self) -> None:
        """Validate the text; raise EventIdInvalidError."""
        if not isinstance(self.value, str) or UUID_V7_PATTERN.fullmatch(self.value) is None:
            raise EventIdInvalidError("an event id must be a lowercase hyphenated UUID of version 7")

    def __str__(self) -> str:
        """Return the canonical text."""
        return self.value
