"""The size limits of an event (REQ-CTR-010, REQ-SEC-004)."""

from dataclasses import dataclass

from telemetry.internal.domain.errors import BatchLimitBelowEventLimitError, LimitInvalidError
from telemetry.internal.domain.rules import (
    DEFAULT_MAX_ATTRIBUTE_KEY_CHARS,
    DEFAULT_MAX_ATTRIBUTE_KEYS,
    DEFAULT_MAX_ATTRIBUTE_VALUE_CHARS,
    DEFAULT_MAX_BATCH_BYTES,
    DEFAULT_MAX_EVENT_BYTES,
)


@dataclass(frozen=True, slots=True)
class Limits:
    """Limits holds the five size limits; ``Limits()`` is the default set.

    Keys and values are counted in characters (Unicode code points); the event and the batch in bytes of canonical
    JSON. T-03.6 maps the configuration to it.
    """

    max_attribute_keys: int = DEFAULT_MAX_ATTRIBUTE_KEYS
    max_attribute_key_chars: int = DEFAULT_MAX_ATTRIBUTE_KEY_CHARS
    max_attribute_value_chars: int = DEFAULT_MAX_ATTRIBUTE_VALUE_CHARS
    max_event_bytes: int = DEFAULT_MAX_EVENT_BYTES
    max_batch_bytes: int = DEFAULT_MAX_BATCH_BYTES

    def __post_init__(self) -> None:
        """Validate: every limit an int (not a bool) above zero, and a batch that holds at least one event."""
        for name in (
            "max_attribute_keys",
            "max_attribute_key_chars",
            "max_attribute_value_chars",
            "max_event_bytes",
            "max_batch_bytes",
        ):
            value = getattr(self, name)
            if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
                raise LimitInvalidError("a limit must be greater than zero")
        if self.max_batch_bytes < self.max_event_bytes:
            raise BatchLimitBelowEventLimitError("the batch limit must hold at least one event")
