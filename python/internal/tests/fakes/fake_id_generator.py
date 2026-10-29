"""A fake of the id generator port."""

import threading
from collections import deque
from typing import Iterable, Optional

from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.value_objects.event_id import EventId

EVENT_ID_PREFIX = "01900000-0000-7000-8000-"


class FakeIdGenerator:
    """FakeIdGenerator returns the queued ids first and then deterministic ones built from a counter."""

    def __init__(self, event_ids: Optional[Iterable[EventId]] = None, instance_id: str = "instance-fake") -> None:
        self._lock = threading.Lock()
        self._queue = deque(event_ids or ())
        self._counter = 0
        self._instance_id = instance_id
        self.event_id_calls = 0
        self.instance_id_calls = 0

    def queue_event_id(self, event_id: EventId) -> None:
        """Add an id to return before the generated ones."""
        with self._lock:
            self._queue.append(event_id)

    def new_event_id(self) -> EventId:
        """Return the next queued id or a valid UUIDv7 from the counter."""
        with self._lock:
            self.event_id_calls += 1
            if self._queue:
                return self._queue.popleft()
            self._counter += 1
            return EventId(f"{EVENT_ID_PREFIX}{self._counter:012x}")

    def new_instance_id(self) -> Identifier:
        """Return the fixed instance id."""
        with self._lock:
            self.instance_id_calls += 1
            return Identifier(self._instance_id)
