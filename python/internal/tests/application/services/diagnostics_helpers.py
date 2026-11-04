"""Helpers of the tests of the diagnostics (T-05.5): a collector of diagnostics and a service built over a channel."""

import threading
import unittest
from typing import Any, Callable, Optional, Sequence

from internal.tests.application.services.test_delivery_service import ItemDeliveryService
from internal.tests.eventually import eventually
from internal.tests.fakes import FakeRandom
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent
from telemetry.internal.application.services.delivery_service import Destinations
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode, EventClass, QueuePolicy
from telemetry.internal.infrastructure.outbound.shared.diagnostics_channel.diagnostics_channel import DiagnosticsChannel
from telemetry.internal.infrastructure.outbound.shared.system_clock.system_clock import SystemClock

LONG_FLUSH_INTERVAL_MS = 60_000
HIGH_LIMIT = 100_000


class Collector:
    """A diagnostics callback that keeps what it receives, safe for the thread of the channel and the test."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._events: list[DiagnosticEvent] = []

    def __call__(self, event: DiagnosticEvent) -> None:
        with self._lock:
            self._events.append(event)

    @property
    def events(self) -> list[DiagnosticEvent]:
        """A copy of the diagnostics received, in order."""
        with self._lock:
            return list(self._events)

    def wait_for(self, count: int, timeout: float = 5.0) -> list[DiagnosticEvent]:
        """Wait until ``count`` diagnostics arrived and return them."""
        eventually(timeout, lambda: len(self.events) >= count, f"{count} diagnostics")
        return self.events


def triples(events: Sequence[DiagnosticEvent]) -> list[tuple[str, str, int]]:
    """The ``(code, reason, count)`` of each diagnostic: what the shared fixture compares."""
    return [(event.code.value, event.reason, event.count) for event in events]


def real_monotonic_milliseconds() -> int:
    """The monotonic milliseconds of the system clock."""
    return SystemClock().monotonic_nanoseconds() // 1_000_000


def new_channel(
    test: unittest.TestCase,
    callbacks: Sequence[Callable[[DiagnosticEvent], object]] = (),
    writer: Any = None,
    read_monotonic_milliseconds: Optional[Callable[[], int]] = None,
    limit: int = HIGH_LIMIT,
    queue_size: int = 64,
) -> DiagnosticsChannel:
    """A channel with a limit that suppresses nothing by default; it is stopped when the test ends."""
    started = SystemClock()
    channel = DiagnosticsChannel(
        callbacks,
        writer,
        read_monotonic_milliseconds
        if read_monotonic_milliseconds is not None
        else (lambda: started.monotonic_nanoseconds() // 1_000_000),
        limit=limit,
        queue_size=queue_size,
    )
    test.addCleanup(channel.stop)
    return channel


def new_service(
    test: unittest.TestCase,
    sinks: Sequence[Any],
    channel: Optional[DiagnosticsChannel] = None,
    mode: DeliveryMode = DeliveryMode.BEST_EFFORT,
    retry_max_attempts: int = 5,
    batch_size: int = 10,
    queue_size: int = 1000,
    flush_interval_milliseconds: int = LONG_FLUSH_INTERVAL_MS,
    queue_policy: QueuePolicy = QueuePolicy.DROP_NEWEST,
    block_timeout_milliseconds: int = 0,
    retry_base_delay_milliseconds: int = 1,
    retry_max_delay_milliseconds: int = 5,
    event_classes: Optional[dict[str, EventClass]] = None,
    start: bool = True,
) -> ItemDeliveryService:
    """A service over the sinks and the channel (the harness of the shared scenarios: backoff 1/5 ms, random 0.5).

    With ``start=False`` neither the workers nor the channel run, so the queue only fills; the channel of a started
    service is started by the service. The service is stopped when the test ends.
    """
    settings = DeliverySettings(
        mode=mode,
        queue_size=queue_size,
        batch_size=batch_size,
        flush_interval_milliseconds=flush_interval_milliseconds,
        queue_policy=queue_policy,
        block_timeout_milliseconds=block_timeout_milliseconds,
        event_classes={} if event_classes is None else event_classes,
        retry_max_attempts=retry_max_attempts,
        retry_base_delay_milliseconds=retry_base_delay_milliseconds,
        retry_max_delay_milliseconds=retry_max_delay_milliseconds,
    )
    service = ItemDeliveryService(
        Destinations(log=tuple(sinks)), settings, SystemClock(), new_memory_queue, FakeRandom((0.5,)), channel
    )
    test.addCleanup(service.stop)
    if start:
        service.start()
    return service
