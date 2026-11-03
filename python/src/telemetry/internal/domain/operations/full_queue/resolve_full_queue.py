"""The rule that decides what a FULL queue does with a new event (REQ-DLV-007, REQ-DLV-009)."""

from dataclasses import dataclass
from typing import Sequence

from telemetry.internal.domain.value_objects.enums_generated import EventClass, QueuePolicy

RESOLUTION_DROP_NEW = "drop_new"
RESOLUTION_DISPLACE = "displace"
RESOLUTION_WAIT = "wait"

REASON_QUEUE_FULL_NEWEST = "queue_full_newest"
REASON_QUEUE_FULL_OLDEST = "queue_full_oldest"
REASON_DISPLACED_BY_CRITICAL = "displaced_by_critical"
REASON_BLOCK_TIMEOUT = "block_timeout"

NO_INDEX = -1

_CLASS_ORDER = (EventClass.BULK, EventClass.NORMAL, EventClass.CRITICAL)  # lowest first


@dataclass(frozen=True, slots=True)
class Resolution:
    """Resolution is what the full queue does: ``drop_new``, ``displace`` the event at ``index`` or ``wait``.

    ``index`` is the position (oldest first) of the queued event that leaves, and -1 unless the outcome is
    ``displace``. ``reason`` is the reason of the drop: for ``wait`` it is the reason the new event gets if no room
    appears in time (``block_timeout``).
    """

    outcome: str
    index: int = NO_INDEX
    reason: str = ""


def _oldest_of_class(queued_classes: Sequence[EventClass], wanted: EventClass) -> int:
    for index, event_class in enumerate(queued_classes):
        if event_class == wanted:
            return index
    return NO_INDEX


def _oldest_not_higher(queued_classes: Sequence[EventClass], new_class: EventClass) -> int:
    for index, event_class in enumerate(queued_classes):
        if _CLASS_ORDER.index(event_class) <= _CLASS_ORDER.index(new_class):
            return index
    return NO_INDEX


def resolve_full_queue(policy: QueuePolicy, new_class: EventClass, queued_classes: Sequence[EventClass]) -> Resolution:
    """Return what a full queue does with a new event of ``new_class``; ``queued_classes`` is oldest first.

    1. A ``critical`` event displaces the OLDEST queued event of class ``bulk`` or, when there is none, the OLDEST one
       of class ``normal``, with the reason ``displaced_by_critical``; the new event takes the last place.
    2. When nothing is displaced (a non-critical event, or a critical one with no lower class queued) the policy
       decides. ``drop_newest`` drops the new event (``queue_full_newest``). ``drop_oldest`` displaces the oldest
       queued event whose class is not higher than the class of the new one (``queue_full_oldest``) and, when there
       is none, drops the new event (``queue_full_newest``). ``block_with_timeout`` answers ``wait`` and changes
       nothing now: if no room appears within the timeout the new event is dropped with the reason ``block_timeout``.

    The class order is bulk < normal < critical. The order of the events that stay is kept. It is pure: the queue is
    only described by its classes, so the caller applies the answer while it holds the lock of the queue.
    """
    if new_class == EventClass.CRITICAL:
        index = _oldest_of_class(queued_classes, EventClass.BULK)
        if index == NO_INDEX:
            index = _oldest_of_class(queued_classes, EventClass.NORMAL)
        if index != NO_INDEX:
            return Resolution(RESOLUTION_DISPLACE, index, REASON_DISPLACED_BY_CRITICAL)
    if policy == QueuePolicy.DROP_OLDEST:
        index = _oldest_not_higher(queued_classes, new_class)
        if index != NO_INDEX:
            return Resolution(RESOLUTION_DISPLACE, index, REASON_QUEUE_FULL_OLDEST)
    if policy == QueuePolicy.BLOCK_WITH_TIMEOUT:
        return Resolution(RESOLUTION_WAIT, NO_INDEX, REASON_BLOCK_TIMEOUT)
    return Resolution(RESOLUTION_DROP_NEW, NO_INDEX, REASON_QUEUE_FULL_NEWEST)
