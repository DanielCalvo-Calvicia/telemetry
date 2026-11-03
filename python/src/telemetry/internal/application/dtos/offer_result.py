"""The result of offering an event to a bounded queue."""

from dataclasses import dataclass
from typing import Optional

from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@dataclass(frozen=True, slots=True)
class OfferResult:
    """OfferResult says what the queue did with an offered event.

    ``admitted`` is True when the new event is now in the queue. ``displaced`` is the queued event that was removed to
    make room (None when none was). ``reason`` is the reason of the drop: of the NEW event when it is not admitted,
    of the displaced one when there is one, and empty when nothing was dropped. ``must_wait`` is True when the queue
    is full, nothing changed and the caller may wait for room (``reason`` is then the one the new event gets when the
    wait ends without room).
    """

    admitted: bool
    displaced: Optional[Envelope] = None
    reason: str = ""
    must_wait: bool = False
