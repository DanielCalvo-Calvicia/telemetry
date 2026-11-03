"""The memory queue: the EventQueuePort over a deque, bounded and safe for many threads."""

import collections
import threading
from typing import Optional

from telemetry.internal.application.dtos.offer_result import OfferResult
from telemetry.internal.domain.operations.full_queue.resolve_full_queue import (
    RESOLUTION_DISPLACE,
    RESOLUTION_DROP_NEW,
    resolve_full_queue,
)
from telemetry.internal.domain.value_objects.enums_generated import EventClass, QueuePolicy
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.infrastructure.config.errors import ConfigProblem, ConfigurationError


class MemoryQueue:
    """MemoryQueue implements EventQueuePort: a FIFO of at most ``capacity`` events, each with its class and arrival.

    One lock guards every operation and no operation waits for anything, so the log call that offers never waits for
    the worker that takes. When the queue is full ``offer`` asks ``resolve_full_queue`` and applies the answer under
    the same lock, so the decision and the change are one step and the queue never holds more than ``capacity``. It
    does no I/O, so it has no ``external/`` folder, and an event is held by reference: the queue never copies or
    changes it.
    """

    def __init__(self, capacity: int) -> None:
        """Create an empty queue; a capacity that is not an integer greater than zero raises ConfigurationError."""
        if not isinstance(capacity, int) or isinstance(capacity, bool) or capacity <= 0:
            problem = ConfigProblem(
                "queue_size", ERROR_CODE_CONFIG_FIELD_INVALID, "it must be an integer greater than zero"
            )
            raise ConfigurationError((problem,))
        self._capacity = capacity
        self._lock = threading.Lock()
        self._items: collections.deque[tuple[Envelope, EventClass, int]] = collections.deque()

    def offer(self, envelope: Envelope, event_class: EventClass, arrival_ms: int, policy: QueuePolicy) -> OfferResult:
        """Add the event at the end; when the queue is full apply the policy and the class (see EventQueuePort)."""
        with self._lock:
            if len(self._items) < self._capacity:
                self._items.append((envelope, event_class, arrival_ms))
                return OfferResult(admitted=True)
            resolution = resolve_full_queue(policy, event_class, [item[1] for item in self._items])
            if resolution.outcome == RESOLUTION_DROP_NEW:
                return OfferResult(admitted=False, reason=resolution.reason)
            if resolution.outcome == RESOLUTION_DISPLACE:
                displaced = self._items[resolution.index][0]
                del self._items[resolution.index]
                self._items.append((envelope, event_class, arrival_ms))
                return OfferResult(admitted=True, displaced=displaced, reason=resolution.reason)
            return OfferResult(admitted=False, reason=resolution.reason, must_wait=True)

    def take(self, n: int) -> list[Envelope]:
        """Remove and return the oldest ``n`` events in order, or all of them when fewer are held; none for n <= 0."""
        with self._lock:
            count = min(max(n, 0), len(self._items))
            return [self._items.popleft()[0] for _ in range(count)]

    def length(self) -> int:
        """Return how many events are waiting."""
        with self._lock:
            return len(self._items)

    def oldest_arrival_ms(self) -> Optional[int]:
        """Return the arrival millisecond of the oldest event, or None when the queue is empty."""
        with self._lock:
            return self._items[0][2] if self._items else None
