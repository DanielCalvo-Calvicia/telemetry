"""The service that queues each event for every destination of its kind and sends it from a worker thread."""

import dataclasses
import threading
import time
from typing import Callable, Mapping, Optional, Sequence

from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.shutdown_result import ShutdownResult
from telemetry.internal.application.ports.outbound.ai_telemetry_sink_port import AITelemetrySinkPort
from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.application.ports.outbound.device_telemetry_sink_port import DeviceTelemetrySinkPort
from telemetry.internal.application.ports.outbound.event_queue_port import EventQueuePort
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.application.ports.outbound.metrics_sink_port import MetricsSinkPort
from telemetry.internal.application.ports.outbound.trace_sink_port import TraceSinkPort
from telemetry.internal.domain.operations.batching.plan_batch import PLAN_IDLE, PLAN_SEND, plan_batch
from telemetry.internal.domain.operations.classification.classify_event import classify_event
from telemetry.internal.domain.operations.full_queue.resolve_full_queue import (
    REASON_BLOCK_TIMEOUT,
    REASON_DISPLACED_BY_CRITICAL,
    REASON_QUEUE_FULL_NEWEST,
    REASON_QUEUE_FULL_OLDEST,
)
from telemetry.internal.domain.value_objects.enums_generated import EventClass
from telemetry.internal.domain.value_objects.envelope_generated import Envelope

REASON_NO_DESTINATION = "no_destination"
REASON_SHUTDOWN = "shutdown"
REASON_SINK_PANICKED = "sink_panicked"
REASON_UNSPECIFIED = "unspecified"

# The reasons of the drops of a full queue live in the domain rule; they are re-exported here with the others.
__all__ = [
    "REASON_BLOCK_TIMEOUT",
    "REASON_DISPLACED_BY_CRITICAL",
    "REASON_NO_DESTINATION",
    "REASON_QUEUE_FULL_NEWEST",
    "REASON_QUEUE_FULL_OLDEST",
    "REASON_SHUTDOWN",
    "REASON_SINK_PANICKED",
    "REASON_UNSPECIFIED",
    "DeliveryService",
    "Destinations",
    "DestinationStats",
]

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
    events that entered the queue (accepted means QUEUED, not delivered). ``dropped`` counts the events a full queue
    refused or displaced plus the ones the sink reported as dropped, and ``failed`` the ones the sink reported as
    failed or raised on. ``dropped_by_reason`` splits ``dropped`` by reason (``queue_full_newest``,
    ``queue_full_oldest``, ``displaced_by_critical``, ``block_timeout`` and the reasons of the sink; a sink that drops
    without a reason counts under ``unspecified``): it is a COPY, so it does not change afterwards. ``last_reason`` is
    the reason of the last drop or failure, and empty when there was none. These are the internal counters; the
    public diagnostics are T-05.5.
    """

    kind: str
    index: int
    queued: int
    batches: int
    accepted: int
    dropped: int
    failed: int
    last_reason: str
    dropped_by_reason: Mapping[str, int] = dataclasses.field(default_factory=dict)


class _Destination:
    """One destination: its sink, its queue, its counters and the two conditions of one lock.

    The lock guards the counters and the wake-ups. ``condition`` is the one the worker sleeps on, ``room`` the one a
    caller waits on for room in a full queue (``block_with_timeout``) and ``settled`` the one a ``flush`` waits on
    until the events it counted have left; they share the lock, so a notification of the one never wakes a waiter of
    the other and none is lost. ``in_flight`` is the size of the batch the sink holds now, ``delivered`` the events
    the sink ACCEPTED, ``finished`` the events whose send ended (whatever the outcome) and ``displaced`` the ones a
    full queue removed: an event leaves the destination by one of the last two. ``closed`` is set by ``shutdown`` and
    ``flush_pending`` counts the flushes that wait, so the worker sends partial batches at once. An offer and the
    decision of the worker happen under the lock; the worker releases it while it calls the sink, so a log call never
    waits for a send.
    """

    def __init__(self, kind: str, index: int, sink: object, queue: EventQueuePort) -> None:
        self.kind = kind
        self.index = index
        self.sink = sink
        self.queue = queue
        lock = threading.Lock()
        self.condition = threading.Condition(lock)
        self.room = threading.Condition(lock)
        self.settled = threading.Condition(lock)
        self.stopping = False
        self.closed = False
        self.flush_pending = 0
        self.in_flight = 0
        self.thread: Optional[threading.Thread] = None
        self.batches = 0
        self.accepted = 0
        self.dropped = 0
        self.failed = 0
        self.delivered = 0
        self.finished = 0
        self.displaced = 0
        self.last_reason = ""
        self.dropped_by_reason: dict[str, int] = {}

    def count_drop(self, reason: str, count: int = 1) -> None:
        """Count dropped events under their reason; call it with the lock held."""
        self.dropped += count
        self.dropped_by_reason[reason] = self.dropped_by_reason.get(reason, 0) + count
        self.last_reason = reason


class DeliveryService:
    """DeliveryService gives every destination a bounded queue and a worker thread that sends in batches.

    A log call only ENQUEUES and returns: it never waits for the network, the disk or a slow destination. The ONE
    exception is the policy ``block_with_timeout``: with a full queue the call waits for room, at most the configured
    timeout (see ``enqueue``). There is one queue and one daemon ``threading.Thread`` per destination, so a slow
    destination blocks only its own worker, and the order of the events inside one destination is kept. The worker
    touches no event loop, so it also works inside a program that runs ``asyncio``.

    The workers start only on ``start()`` and never on construction or import. ``stop()`` signals the workers and joins
    them without draining the queues; ``flush`` and ``shutdown`` drain them first and return a result (T-05.4). There is
    NO synchronous send and no hook at interpreter exit: a program MUST call ``shutdown`` before it ends or the events
    still queued are lost. A batch leaves
    when it holds ``batch_size`` events or when the oldest one has waited ``flush_interval_milliseconds`` (the rule is
    ``plan_batch``). Each batch is sent ONCE: the retry is T-05.3. When a queue is full the policy and the class of
    the event decide (``resolve_full_queue``) and every drop is counted per destination and reason; the class is a
    priority, never a guarantee. A started service does not start again after it was stopped.
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
        self._closed = False
        self._shutdown_lock = threading.Lock()
        self._shutdown_result: Optional[ShutdownResult] = None
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
        sinks have their own timeouts. The events still queued stay there (``shutdown`` drains them first).
        """
        for thread in self._signal_stop():
            thread.join()

    def _signal_stop(self) -> list[threading.Thread]:
        """Mark the service stopped, wake every worker and return the threads to join (none when already stopped)."""
        with self._lifecycle:
            if self._stopped:
                return []
            self._stopped = True
            threads = []
            for destination in self._all:
                with destination.condition:
                    destination.stopping = True
                    destination.condition.notify_all()
                    destination.room.notify_all()  # a caller that waits for room ends its wait
                if destination.thread is not None:
                    threads.append(destination.thread)
            return threads

    @property
    def closed(self) -> bool:
        """True once ``shutdown`` has begun: from then on every event is dropped with the reason ``shutdown``."""
        return self._closed

    def reject_closed(self, kind: str) -> EmitResult:
        """Drop one event because the service is closed: count it under ``shutdown`` in each destination of the kind."""
        for destination in self._by_kind.get(kind, []):
            with destination.condition:
                destination.count_drop(REASON_SHUTDOWN)
        return EmitResult(dropped=1, reason=REASON_SHUTDOWN)

    def flush(self, timeout_ms: int) -> ShutdownResult:
        """Send what is queued now and keep running; wait at most ``timeout_ms`` and say what became of it.

        Per destination the TARGET is the number of events queued plus the ones being sent when the call begins. The
        workers send partial batches at once while the flush waits (the flush interval does not apply), and the call
        waits on a condition the workers notify, never by polling. ``sent`` is what the sinks ACCEPTED of the targets;
        ``dropped`` is every other target: dropped or failed by the sink (there is no retry yet, T-05.3), displaced
        from a full queue, or still queued or in flight when the time ends. ``spooled`` is 0 until the spool (T-06).
        A timeout of 0 or less returns at once with everything pending counted as dropped. It never raises, and
        after ``shutdown`` it returns the zero result.
        """
        try:
            if self._closed:
                return ShutdownResult()
            return self._drain(timeout_ms, close=False)
        except Exception:  # a flush never fails the caller (ADR-005)
            return ShutdownResult()

    def shutdown(self, timeout_ms: int) -> ShutdownResult:
        """Flush with the budget ``timeout_ms``, then stop the workers; a second call returns the FIRST result.

        The service is closed from the first moment: an event that arrives afterwards is dropped with the reason
        ``shutdown``. The workers are joined only for the time that is left of the budget: a sink that never returns
        does not hold the call past the timeout, because its worker is a DAEMON thread that is abandoned (it ends
        with the process). A call that arrives while another shutdown runs waits for it and returns its result. It
        never raises.
        """
        with self._shutdown_lock:
            if self._shutdown_result is not None:
                return self._shutdown_result
            self._closed = True
            deadline = time.monotonic() + self._budget_seconds(timeout_ms)
            try:
                result = self._drain(timeout_ms, close=True)
            except Exception:  # a shutdown never fails the caller (ADR-005)
                result = ShutdownResult()
            for thread in self._signal_stop():
                thread.join(max(0.0, deadline - time.monotonic()))
            self._shutdown_result = result
            return result

    @staticmethod
    def _budget_seconds(timeout_ms: int) -> float:
        """The timeout in seconds; a value that is not a number or is not above zero is a budget of zero."""
        try:
            return max(0.0, timeout_ms / _MILLISECONDS_PER_SECOND)
        except TypeError:
            return 0.0

    def _drain(self, timeout_ms: int, close: bool) -> ShutdownResult:
        """Count the targets of every destination, wake the workers and wait until they are done or time is up."""
        budget = self._budget_seconds(timeout_ms)
        deadline = time.monotonic() + budget
        targets: list[tuple[_Destination, int, int, int, bool]] = []
        for destination in self._all:
            with destination.condition:
                if close:
                    destination.closed = True  # nothing enters after the count, so the target is final
                target = destination.queue.length() + destination.in_flight
                thread = destination.thread
                waits = target > 0 and budget > 0 and thread is not None and thread.is_alive()
                if waits:
                    destination.flush_pending += 1
                    destination.condition.notify_all()  # the worker plans again, now with no flush interval
                targets.append(
                    (destination, target, destination.delivered, destination.finished + destination.displaced, waits)
                )
        try:
            for destination, target, _, left_before, waits in targets:
                if not waits:
                    continue
                with destination.condition:
                    while destination.finished + destination.displaced - left_before < target:
                        remaining = deadline - time.monotonic()
                        if remaining <= 0:
                            break
                        destination.settled.wait(remaining)
        finally:
            for destination, _, _, _, waits in targets:
                if waits:
                    with destination.condition:
                        destination.flush_pending -= 1
        sent = dropped = 0
        for destination, target, delivered_before, _, _ in targets:
            with destination.condition:
                accepted = min(destination.delivered - delivered_before, target)
            sent += accepted
            dropped += target - accepted
        return ShutdownResult(sent=sent, spooled=0, dropped=dropped)

    def enqueue(self, kind: str, envelope: Envelope) -> EmitResult:
        """Queue the event for every destination of the kind and say what became of it.

        No destination of the kind gives ``dropped=1`` with the reason ``no_destination``. Otherwise the event is
        classified once (``classify_event``: its event type, its severity and the assignments of the configuration)
        and offered to the queue of each destination, which counts ``accepted=1`` when it took the event. A full queue
        applies the policy (``resolve_full_queue``): ``drop_newest`` drops the NEW event (``queue_full_newest``);
        ``drop_oldest`` removes the oldest event of a class not higher than the new one (``queue_full_oldest``);
        a ``critical`` event removes the oldest ``bulk`` or else ``normal`` one (``displaced_by_critical``). A removed
        event counts as dropped in the STATS of its destination under its reason and does not change the result of
        this call, which is about the new event. The result adds the destinations and its reason is the first that is
        not empty. A drop never raises.

        With ``block_with_timeout`` a full queue makes the caller WAIT for room, at most ``block_timeout_milliseconds``,
        on a condition the worker notifies each time it takes events; if there is still no room the new event is
        dropped with the reason ``block_timeout``, and ``stop()`` ends the wait the same way. This is the ONE case in
        which a log call waits: it blocks the calling thread, and also the event loop when the call comes from asyncio.
        In every other case the call returns at once. The class is a priority and never a guarantee of delivery.
        """
        if self._closed:
            return self.reject_closed(kind)
        destinations = self._by_kind.get(kind, [])
        if not destinations:
            return EmitResult(dropped=1, reason=REASON_NO_DESTINATION)
        arrival_ms = self._clock.monotonic_nanoseconds() // _NANOSECONDS_PER_MILLISECOND
        event_class = classify_event(envelope.event_type, envelope.severity, self._settings.event_classes)
        accepted = dropped = 0
        reason = ""
        for destination in destinations:
            drop_reason = self._offer(destination, envelope, event_class, arrival_ms)
            if drop_reason == "":
                accepted += 1
            else:
                dropped += 1
                reason = reason or drop_reason
        return EmitResult(accepted=accepted, dropped=dropped, reason=reason)

    def _offer(self, destination: _Destination, envelope: Envelope, event_class: EventClass, arrival_ms: int) -> str:
        """Offer the event to one destination; return "" when it was queued, or the reason it was dropped."""
        settings = self._settings
        deadline: Optional[float] = None
        with destination.condition:
            while True:
                if destination.closed:  # a shutdown began while this call was on its way
                    destination.count_drop(REASON_SHUTDOWN)
                    return REASON_SHUTDOWN
                offer = destination.queue.offer(envelope, event_class, arrival_ms, settings.queue_policy)
                if offer.admitted:
                    destination.accepted += 1
                    if offer.displaced is not None:
                        destination.count_drop(offer.reason)
                        destination.displaced += 1
                        destination.settled.notify_all()  # a flush that counted the displaced event stops waiting
                    destination.condition.notify()
                    return ""
                if not offer.must_wait:
                    destination.count_drop(offer.reason)
                    return offer.reason
                if deadline is None:
                    deadline = time.monotonic() + settings.block_timeout_milliseconds / _MILLISECONDS_PER_SECOND
                remaining = deadline - time.monotonic()
                if remaining <= 0 or destination.stopping:
                    destination.count_drop(REASON_BLOCK_TIMEOUT)
                    return REASON_BLOCK_TIMEOUT
                destination.room.wait(remaining)

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
                        dropped_by_reason=dict(destination.dropped_by_reason),
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
                flush_interval_ms = 0 if destination.flush_pending > 0 else settings.flush_interval_milliseconds
                oldest_ms = destination.queue.oldest_arrival_ms()
                plan = plan_batch(
                    self._clock.monotonic_nanoseconds() // _NANOSECONDS_PER_MILLISECOND,
                    destination.queue.length(),
                    0 if oldest_ms is None else oldest_ms,
                    settings.batch_size,
                    flush_interval_ms,
                )
                if plan.action == PLAN_SEND:
                    batch = destination.queue.take(plan.take)
                    destination.in_flight = len(batch)
                    destination.room.notify_all()  # there is room now: a caller that waits for it offers again
                    return batch
                if plan.action == PLAN_IDLE:
                    destination.condition.wait()
                else:
                    destination.condition.wait(plan.wait_ms / _MILLISECONDS_PER_SECOND)
        return None

    def _send(self, destination: _Destination, batch: list[Envelope]) -> None:
        """Call the sink once, contained: whatever it raises is a failure of the batch and is never kept or printed."""
        try:
            result = destination.sink.send(batch)  # type: ignore[attr-defined]
            accepted, dropped, failed, reason = result.accepted, result.dropped, result.failed, result.reason
        except Exception:  # a sink never stops its worker or the others (ADR-005); the text could hold event data
            accepted, dropped, failed, reason = 0, 0, len(batch), REASON_SINK_PANICKED
        with destination.condition:
            destination.batches += 1
            destination.delivered += accepted
            destination.finished += len(batch)
            destination.in_flight = 0
            destination.settled.notify_all()
            destination.failed += failed
            if dropped:
                destination.count_drop(reason or REASON_UNSPECIFIED, dropped)
            if reason and failed:
                destination.last_reason = reason
