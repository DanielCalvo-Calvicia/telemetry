"""The event queue port."""

from typing import Optional, Protocol, runtime_checkable

from telemetry.internal.application.dtos.offer_result import OfferResult
from telemetry.internal.domain.value_objects.enums_generated import EventClass, QueuePolicy
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@runtime_checkable
class EventQueuePort(Protocol):
    """EventQueuePort is what the application requires of the bounded queue of one destination.

    The log call offers and the worker of the destination takes; both may run at the same time, so an implementation
    is safe for many threads. Events leave in the order they entered.
    """

    def offer(self, envelope: Envelope, event_class: EventClass, arrival_ms: int, policy: QueuePolicy) -> OfferResult:
        """Add the event at the end, with its class and the millisecond it arrived, or apply the policy when full.

        With room the event is admitted. When the queue is full the rule of ``resolve_full_queue`` decides, and the
        decision and its effect are ONE atomic step: the new event is dropped (not admitted, with the reason), or a
        queued event is removed keeping the order of the rest and the new one takes the last place (admitted, the
        removed event in ``displaced``), or the caller must wait (not admitted, ``must_wait``, nothing changed). It
        never waits itself and never raises.
        """

    def take(self, n: int) -> list[Envelope]:
        """Remove and return the oldest ``n`` events in order, or all of them when fewer are held."""

    def length(self) -> int:
        """Return how many events are waiting."""

    def oldest_arrival_ms(self) -> Optional[int]:
        """Return the arrival millisecond of the oldest event, or None when the queue is empty."""
