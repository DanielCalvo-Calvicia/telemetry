"""The memory queue: the EventQueuePort over a deque, bounded and safe for many threads."""

import collections
import threading
from typing import Optional

from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.infrastructure.config.errors import ConfigProblem, ConfigurationError


class MemoryQueue:
    """MemoryQueue implements EventQueuePort: a FIFO of at most ``capacity`` events, each with its arrival millisecond.

    One lock guards every operation and no operation waits for anything, so the log call that pushes never waits for
    the worker that takes. A full queue refuses the new event and keeps what it holds. It does no I/O, so it has no
    ``external/`` folder, and an event is held by reference: the queue never copies or changes it.
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
        self._items: collections.deque[tuple[Envelope, int]] = collections.deque()

    def push(self, envelope: Envelope, arrival_ms: int) -> bool:
        """Add the event at the end; return False, and keep nothing, when the queue is full."""
        with self._lock:
            if len(self._items) >= self._capacity:
                return False
            self._items.append((envelope, arrival_ms))
            return True

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
            return self._items[0][1] if self._items else None
