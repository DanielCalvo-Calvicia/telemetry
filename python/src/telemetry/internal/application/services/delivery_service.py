"""The service that queues each event for every destination of its kind and sends it from a worker thread."""

import dataclasses
import threading
from typing import Callable, Optional, Sequence

from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.ports.outbound.ai_telemetry_sink_port import AITelemetrySinkPort
from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.application.ports.outbound.device_telemetry_sink_port import DeviceTelemetrySinkPort
from telemetry.internal.application.ports.outbound.event_queue_port import EventQueuePort
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.application.ports.outbound.metrics_sink_port import MetricsSinkPort
from telemetry.internal.application.ports.outbound.trace_sink_port import TraceSinkPort
from telemetry.internal.domain.operations.batching.plan_batch import PLAN_IDLE, PLAN_SEND, plan_batch
from telemetry.internal.domain.value_objects.envelope_generated import Envelope

REASON_NO_DESTINATION = "no_destination"
REASON_QUEUE_FULL = "queue_full"
REASON_SINK_PANICKED = "sink_panicked"

_NANOSECONDS_PER_MILLISECOND = 1_000_000
_MILLISECONDS_PER_SECOND = 1000


@dataclasses.dataclass(frozen=True)
class Destinations:
    """Destinations holds the sinks of each kind, in the order they were given; any kind may have several."""

    log: tuple[LogSinkPort, ...] = ()
    event: tuple[EventSinkPort, ...] = ()
    metrics: tuple[MetricsSinkPort, ...] = ()
    trace: tuple[TraceSinkPort, ...] = ()
    device_telemetry: tuple[DeviceTelemetrySinkPort, ...] = ()
    ai_telemetry: tuple[AITelemetrySinkPort, ...] = ()


@dataclasses.dataclass(frozen=True, slots=True)
class DestinationStats:
    """DestinationStats is a snapshot of the counters of one destination, taken at one moment.

    ``kind`` is the name of the field of Destinations and ``index`` the position of the sink inside that kind.
    ``queued`` is how many events wait now. ``batches`` is how many times the sink was called. ``accepted`` counts the
    events that entered the queue (accepted means QUEUED, not delivered). ``dropped`` counts the events refused by a
    full queue plus the ones the sink reported as dropped, and ``failed`` the ones the sink reported as failed or
    raised on. ``last_reason`` is the reason of the last drop or failure, and empty when there was none. These are the
    internal counters; the public diagnostics are T-05.5.
    """

    kind: str
    index: int
    queued: int
    batches: int
    accepted: int
    dropped: int
    failed: int
    last_reason: str


class _Destination:
    """One destination: its sink, its queue, its counters and the condition its worker sleeps on.

    The condition guards the counters and the wake-up of the worker. A push and the decision of the worker happen
    under it, so a wake-up is never lost; the worker releases it while it calls the sink, so a log call never waits
    for a send.
    """

    def __init__(self, kind: str, index: int, sink: object, queue: EventQueuePort) -> None:
        self.kind = kind
        self.index = index
        self.sink = sink
        self.queue = queue
        self.condition = threading.Condition()
        self.stopping = False
        self.thread: Optional[threading.Thread] = None
        self.batches = 0
        self.accepted = 0
        self.dropped = 0
        self.failed = 0
        self.last_reason = ""


class DeliveryService:
    """DeliveryService gives every destination a bounded queue and a worker thread that sends in batches.

    A log call only ENQUEUES and returns: it never waits for the network, the disk or a slow destination. There is one
    queue and one daemon ``threading.Thread`` per destination, so a slow destination blocks only its own worker, and
    the order of the events inside one destination is kept. The worker touches no event loop, so it also works inside
    a program that runs ``asyncio``.

    The workers start only on ``start()`` and never on construction or import. ``stop()`` signals the workers and joins
    them without draining the queues: draining, ``flush`` and ``shutdown`` with a result are T-05.4. A batch leaves
    when it holds ``batch_size`` events or when the oldest one has waited ``flush_interval_milliseconds`` (the rule is
    ``plan_batch``). Each batch is sent ONCE: the retry is T-05.3, and the drop policies of a full queue are T-05.2.
    A started service does not start again after it was stopped.
    """

    def __init__(
        self,
        destinations: Destinations,
        settings: DeliverySettings,
        clock: ClockPort,
        new_queue: Callable[[int], EventQueuePort],
    ) -> None:
        """Create a queue for every sink of every kind; nothing runs until ``start()`` is called."""
        self._settings = settings
        self._clock = clock
        self._lifecycle = threading.Lock()
        self._started = False
        self._stopped = False
        self._all: list[_Destination] = []
        self._by_kind: dict[str, list[_Destination]] = {}
        for field in dataclasses.fields(destinations):
            sinks: Sequence[object] = getattr(destinations, field.name)
            entries = [
                _Destination(field.name, index, sink, new_queue(settings.queue_size))
                for index, sink in enumerate(sinks)
            ]
            self._by_kind[field.name] = entries
            self._all.extend(entries)

    def start(self) -> None:
        """Start one daemon worker thread per destination; calling it again, or after ``stop()``, does nothing."""
        with self._lifecycle:
            if self._started or self._stopped:
                return
            self._started = True
            for destination in self._all:
                thread = threading.Thread(
                    target=self._work,
                    args=(destination,),
                    name=f"telemetry-delivery-{destination.kind}-{destination.index}",
                    daemon=True,
                )
                destination.thread = thread
                thread.start()

    def stop(self) -> None:
        """Signal every worker to end and join them; calling it again does nothing. The queues are NOT drained.

        A worker that is inside a send ends when the sink returns, so a sink that never returns holds this call: the
        sinks have their own timeouts. The events still queued stay there (T-05.4 drains them).
        """
        with self._lifecycle:
            if self._stopped:
                return
            self._stopped = True
            threads = []
            for destination in self._all:
                with destination.condition:
                    destination.stopping = True
                    destination.condition.notify_all()
                if destination.thread is not None:
                    threads.append(destination.thread)
        for thread in threads:
            thread.join()

    def enqueue(self, kind: str, envelope: Envelope) -> EmitResult:
        """Queue the event for every destination of the kind and say what became of it; it never waits or raises.

        No destination of the kind gives ``dropped=1`` with the reason ``no_destination``. Otherwise each destination
        counts ``accepted=1`` when its queue took the event and ``dropped=1`` with the reason ``queue_full`` when it
        was full (the NEW event is the one dropped; T-05.2 adds the policies). The result adds the destinations and its
        reason is the first that is not empty.
        """
        destinations = self._by_kind.get(kind, [])
        if not destinations:
            return EmitResult(dropped=1, reason=REASON_NO_DESTINATION)
        arrival_ms = self._clock.monotonic_nanoseconds() // _NANOSECONDS_PER_MILLISECOND
        accepted = dropped = 0
        reason = ""
        for destination in destinations:
            with destination.condition:
                if destination.queue.push(envelope, arrival_ms):
                    destination.accepted += 1
                    destination.condition.notify()
                    accepted += 1
                else:
                    destination.dropped += 1
                    destination.last_reason = REASON_QUEUE_FULL
                    dropped += 1
                    reason = reason or REASON_QUEUE_FULL
        return EmitResult(accepted=accepted, dropped=dropped, reason=reason)

    def stats(self) -> list[DestinationStats]:
        """Return a snapshot of the counters of every destination, in the order of the kinds and of the sinks."""
        snapshot = []
        for destination in self._all:
            with destination.condition:
                snapshot.append(
                    DestinationStats(
                        kind=destination.kind,
                        index=destination.index,
                        queued=destination.queue.length(),
                        batches=destination.batches,
                        accepted=destination.accepted,
                        dropped=destination.dropped,
                        failed=destination.failed,
                        last_reason=destination.last_reason,
                    )
                )
        return snapshot

    def _work(self, destination: _Destination) -> None:
        """The loop of a worker: wait for a batch, send it once with no lock held, count the result, repeat."""
        while True:
            batch = self._next_batch(destination)
            if batch is None:
                return
            self._send(destination, batch)

    def _next_batch(self, destination: _Destination) -> Optional[list[Envelope]]:
        """Wait on the condition (never in a loop of sleeps) until a batch is due; return None when stopping."""
        settings = self._settings
        with destination.condition:
            while not destination.stopping:
                oldest_ms = destination.queue.oldest_arrival_ms()
                plan = plan_batch(
                    self._clock.monotonic_nanoseconds() // _NANOSECONDS_PER_MILLISECOND,
                    destination.queue.length(),
                    0 if oldest_ms is None else oldest_ms,
                    settings.batch_size,
                    settings.flush_interval_milliseconds,
                )
                if plan.action == PLAN_SEND:
                    return destination.queue.take(plan.take)
                if plan.action == PLAN_IDLE:
                    destination.condition.wait()
                else:
                    destination.condition.wait(plan.wait_ms / _MILLISECONDS_PER_SECOND)
        return None

    def _send(self, destination: _Destination, batch: list[Envelope]) -> None:
        """Call the sink once, contained: whatever it raises is a failure of the batch and is never kept or printed."""
        try:
            result = destination.sink.send(batch)  # type: ignore[attr-defined]
            dropped, failed, reason = result.dropped, result.failed, result.reason
        except Exception:  # a sink never stops its worker or the others (ADR-005); the text could hold event data
            dropped, failed, reason = 0, len(batch), REASON_SINK_PANICKED
        with destination.condition:
            destination.batches += 1
            destination.dropped += dropped
            destination.failed += failed
            if reason and (dropped or failed):
                destination.last_reason = reason
