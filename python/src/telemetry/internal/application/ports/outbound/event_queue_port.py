"""The event queue port."""

from typing import Optional, Protocol, runtime_checkable

from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@runtime_checkable
class EventQueuePort(Protocol):
    """EventQueuePort is what the application requires of the bounded queue of one destination.

    The log call pushes and the worker of the destination takes; both may run at the same time, so an implementation
    is safe for many threads. Events leave in the order they entered.
    """

    def push(self, envelope: Envelope, arrival_ms: int) -> bool:
        """Add the event at the end with the millisecond it arrived; return False, and keep nothing, when it is full."""

    def take(self, n: int) -> list[Envelope]:
        """Remove and return the oldest ``n`` events in order, or all of them when fewer are held."""

    def length(self) -> int:
        """Return how many events are waiting."""

    def oldest_arrival_ms(self) -> Optional[int]:
        """Return the arrival millisecond of the oldest event, or None when the queue is empty."""
