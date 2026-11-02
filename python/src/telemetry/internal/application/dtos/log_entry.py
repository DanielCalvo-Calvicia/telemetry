"""What a caller gives the logger for one log call."""

from dataclasses import dataclass
from typing import Any, Mapping, Optional


@dataclass(frozen=True, slots=True)
class LogEntry:
    """LogEntry is the input of one log call: a category, a message, the function and extra fields.

    ``function`` empty means it was not given, and then the event has no ``function`` in its data. ``fields`` are the
    extra key-value pairs of the call; they become the attributes of the event under the rules of
    ``fields_to_attributes``. Nothing else is recorded: no request body, no payload and no traceback (REQ-LOG-006).
    """

    category: str = ""
    message: str = ""
    function: str = ""
    fields: Optional[Mapping[str, Any]] = None
