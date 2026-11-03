"""The dependencies of the memory queue adapter."""

from telemetry.internal.application.ports.outbound.event_queue_port import EventQueuePort
from telemetry.internal.infrastructure.outbound.shared.memory_queue.memory_queue import MemoryQueue


def new_memory_queue(capacity: int) -> EventQueuePort:
    """Create a bounded memory queue for one destination; a capacity of zero or less raises ConfigurationError."""
    return MemoryQueue(capacity)
