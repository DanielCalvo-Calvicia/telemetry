"""The counters of what the limits cut and the outcome of applying them."""

from dataclasses import dataclass
from typing import Optional

from telemetry.internal.domain.errors import EventTooLargeError
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@dataclass(frozen=True, slots=True)
class LimitsReport:
    """LimitsReport counts what the limits cut from one event.

    It holds ONLY integers: never the cut text, a key or a value, because it may be sensitive.
    """

    attribute_keys_dropped: int = 0
    attribute_keys_truncated: int = 0
    attribute_values_truncated: int = 0
    events_rejected: int = 0


@dataclass(frozen=True, slots=True)
class LimitsOutcome:
    """LimitsOutcome is what apply_limits returns: the limited envelope, or the error and no envelope.

    The report is always there, also when the event was rejected.
    """

    envelope: Optional[Envelope]
    report: LimitsReport
    error: Optional[EventTooLargeError] = None
