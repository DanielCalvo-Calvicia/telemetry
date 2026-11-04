"""The service that queues each event for every destination of its kind and sends it from a worker thread."""

import dataclasses
import threading
import time
from typing import Callable, Mapping, Optional, Sequence

from telemetry.internal.application.dtos.channel_diagnostics import ChannelDiagnostics
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.destination_diagnostics import DestinationDiagnostics
from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent
from telemetry.internal.application.dtos.diagnostics_snapshot import DiagnosticsSnapshot
from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.flush_outcome import FlushOutcome
from telemetry.internal.application.dtos.shutdown_result import ShutdownResult
from telemetry.internal.application.ports.outbound.ai_telemetry_sink_port import AITelemetrySinkPort
from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.application.ports.outbound.device_telemetry_sink_port import DeviceTelemetrySinkPort
from telemetry.internal.application.ports.outbound.diagnostics_channel_port import DiagnosticsChannelPort
from telemetry.internal.application.ports.outbound.event_queue_port import EventQueuePort
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.application.ports.outbound.metrics_sink_port import MetricsSinkPort
from telemetry.internal.application.ports.outbound.random_port import RandomPort
from telemetry.internal.application.ports.outbound.trace_sink_port import TraceSinkPort
from telemetry.internal.domain.operations.batching.plan_batch import PLAN_IDLE, PLAN_SEND, plan_batch
from telemetry.internal.domain.operations.classification.classify_event import classify_event
from telemetry.internal.domain.operations.diagnostics_reason.sanitize_reason import sanitize_reason
from telemetry.internal.domain.operations.full_queue.resolve_full_queue import (
    REASON_BLOCK_TIMEOUT,
    REASON_DISPLACED_BY_CRITICAL,
    REASON_QUEUE_FULL_NEWEST,
    REASON_QUEUE_FULL_OLDEST,
)
from telemetry.internal.domain.operations.retry_delay.retry_delay import retry_delay
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode, DiagnosticCode, EventClass
from telemetry.internal.domain.value_objects.envelope_generated import Envelope

REASON_NO_DESTINATION = "no_destination"
REASON_RETRY_LIMIT_REACHED = "retry_limit_reached"
REASON_SHUTDOWN = "shutdown"
REASON_SINK_PANICKED = "sink_panicked"
REASON_UNSPECIFIED = "unspecified"
REASON_SERIALIZATION_FAILED = "serialization_failed"
REASON_CONFIGURATION_ERROR = "configuration_error"

# The reasons of the drops of a full queue live in the domain rule; they are re-exported here with the others.
__all__ = [
    "REASON_BLOCK_TIMEOUT",
    "REASON_CONFIGURATION_ERROR",
    "REASON_DISPLACED_BY_CRITICAL",
    "REASON_NO_DESTINATION",
    "REASON_QUEUE_FULL_NEWEST",
    "REASON_QUEUE_FULL_OLDEST",
    "REASON_RETRY_LIMIT_REACHED",
    "REASON_SERIALIZATION_FAILED",
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
    ``queued`` is how many events wait now. ``batches`` is how many batches ENDED (a batch that is retried counts once,
    when its last attempt ends). ``attempts`` is how many times the sink was called and ``retries`` the attempts beyond
    the first of a batch, so ``attempts - retries`` is the number of batches that were tried. ``delivered`` is the
    events the sink ACCEPTED in the answer that ended their batch. ``accepted`` counts the events that entered the
    queue (accepted means QUEUED, not delivered). ``dropped`` counts the events a full queue
    refused or displaced plus the ones the sink reported as dropped, the ones that were still failing when the retry
    limit was reached and the ones abandoned at a shutdown, and ``failed`` the ones the sink reported as failed or
    raised on in a batch that was not retried (the ``best_effort`` mode). ``dropped_by_reason`` splits ``dropped`` by
    reason (``queue_full_newest``, ``queue_full_oldest``, ``displaced_by_critical``, ``block_timeout``,
    ``retry_limit_reached``, ``shutdown`` and the reasons of the sink; a sink that drops without a reason counts under
    ``unspecified``): it is a COPY, so it does not change afterwards. ``last_reason`` is the reason of the last drop or
    failure, and empty when there was none. These are the internal counters; the public diagnostics are T-05.5.
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
    attempts: int = 0
    retries: int = 0
    delivered: int = 0


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
        self.attempts = 0
        self.retries = 0
        self.accepted = 0
        self.dropped = 0
        self.failed = 0
        self.delivered = 0
        self.finished = 0
        self.displaced = 0
        self.last_reason = ""
        self.dropped_by_reason: dict[str, int] = {}
        self.export_failures = 0
        self.serialization_failures = 0
        self.configuration_errors = 0
        self.last_batch_size = 0
        self.max_batch_size = 0
        self.last_latency_ms = 0
        self.max_latency_ms = 0
        self.total_latency_ms = 0
        self.last_success_at_ms = 0
        self.last_flush = FlushOutcome()

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
    ``plan_batch``). With the mode ``best_effort`` each batch is sent ONCE whatever the answer. With ``retry`` a batch
    that failed in a transient way (the answer has failed events and none dropped) is sent AGAIN, whole, after a
    backoff with jitter (and never before the ``retry_after_ms`` of the answer), at most ``retry_max_attempts`` times
    in all; at the limit the failed events are dropped with the reason ``retry_limit_reached``. The wait between two
    attempts is a wait on the condition of the destination that ``stop`` and ``shutdown`` wake (the batch is then
    dropped with the reason ``shutdown``), it counts as in flight for ``flush``, and it delays only its own
    destination: every destination has its own worker. Neither mode survives a restart of the process. When a queue is
    full the policy and the class of the event decide (``resolve_full_queue``) and every drop is counted per
    destination and reason; the class is a priority, never a guarantee. A started service does not start again after
    it was stopped.
    """

    def __init__(
        self,
        destinations: Destinations,
        settings: DeliverySettings,
        clock: ClockPort,
        new_queue: Callable[[int], EventQueuePort],
        random_source: RandomPort,
        diagnostics: Optional[DiagnosticsChannelPort] = None,
    ) -> None:
        """Create a queue for every sink of every kind; nothing runs until ``start()`` is called.

        ``random_source`` gives the jitter of the wait between two attempts of the retry mode. ``diagnostics`` is the
        channel that receives the diagnostics (T-05.5); without one the service works the same and publishes nothing.
        """
        self._settings = settings
        self._clock = clock
        self._random = random_source
        self._diagnostics = diagnostics
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
        """Start one daemon worker thread per destination; calling it again, or after ``stop()``, does nothing.

        It also starts the diagnostics channel, whose own daemon thread calls the callbacks of the consumer.
        """
        with self._lifecycle:
            if self._started or self._stopped:
                return
            self._started = True
            if self._diagnostics is not None:
                self._diagnostics.start()
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
        self._stop_diagnostics()

    def _stop_diagnostics(self) -> None:
        """Stop the diagnostics channel after the workers: what they published last is still handed over."""
        if self._diagnostics is not None:
            self._diagnostics.stop()

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
            self._publish(destination, DiagnosticCode.EVENTS_DROPPED, REASON_SHUTDOWN, 1)
        return EmitResult(dropped=1, reason=REASON_SHUTDOWN)

    def flush(self, timeout_ms: int) -> ShutdownResult:
        """Send what is queued now and keep running; wait at most ``timeout_ms`` and say what became of it.

        Per destination the TARGET is the number of events queued plus the ones being sent when the call begins. The
        workers send partial batches at once while the flush waits (the flush interval does not apply), and the call
        waits on a condition the workers notify, never by polling. ``sent`` is what the sinks ACCEPTED of the targets;
        ``dropped`` is every other target: dropped or failed by the sink (after the retries of the retry mode),
        displaced from a full queue, or still queued or in flight when the time ends (a batch that is waiting to be
        retried is in flight). ``spooled`` is 0 until the spool (T-06).
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
            self._stop_diagnostics()
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
        finished_at_ms = self._wall_milliseconds()
        for destination, target, delivered_before, _, _ in targets:
            with destination.condition:
                accepted = min(destination.delivered - delivered_before, target)
                destination.last_flush = FlushOutcome(
                    done=True, sent=accepted, spooled=0, dropped=target - accepted, at_ms=finished_at_ms
                )
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
        """Offer the event to one destination; return "" when it was queued, or the reason it was dropped.

        Every drop (the new event or a displaced one) is published as a diagnostic AFTER the lock is released.
        """
        drop_reason, displaced_reason = self._offer_locked(destination, envelope, event_class, arrival_ms)
        for reason in (displaced_reason, drop_reason):
            if reason:
                self._publish(destination, DiagnosticCode.EVENTS_DROPPED, reason, 1)
        return drop_reason

    def _offer_locked(
        self, destination: _Destination, envelope: Envelope, event_class: EventClass, arrival_ms: int
    ) -> tuple[str, str]:
        """Offer under the lock; return ``(reason the new event was dropped, reason a queued one was displaced)``."""
        settings = self._settings
        deadline: Optional[float] = None
        with destination.condition:
            while True:
                if destination.closed:  # a shutdown began while this call was on its way
                    destination.count_drop(REASON_SHUTDOWN)
                    return REASON_SHUTDOWN, ""
                offer = destination.queue.offer(envelope, event_class, arrival_ms, settings.queue_policy)
                if offer.admitted:
                    destination.accepted += 1
                    displaced_reason = ""
                    if offer.displaced is not None:
                        destination.count_drop(offer.reason)
                        destination.displaced += 1
                        destination.settled.notify_all()  # a flush that counted the displaced event stops waiting
                        displaced_reason = offer.reason
                    destination.condition.notify()
                    return "", displaced_reason
                if not offer.must_wait:
                    destination.count_drop(offer.reason)
                    return offer.reason, ""
                if deadline is None:
                    deadline = time.monotonic() + settings.block_timeout_milliseconds / _MILLISECONDS_PER_SECOND
                remaining = deadline - time.monotonic()
                if remaining <= 0 or destination.stopping:
                    destination.count_drop(REASON_BLOCK_TIMEOUT)
                    return REASON_BLOCK_TIMEOUT, ""
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
                        attempts=destination.attempts,
                        retries=destination.retries,
                        delivered=destination.delivered,
                    )
                )
        return snapshot

    def diagnostics(self) -> DiagnosticsSnapshot:
        """Return a COPY of the diagnostics of every destination and of the channel (REQ-DGN-001, REQ-DGN-002).

        It works before ``start`` and after ``shutdown``, never raises and shares nothing with the service.
        """
        destinations = []
        for destination in self._all:
            with destination.condition:
                destinations.append(
                    DestinationDiagnostics(
                        kind=destination.kind,
                        index=destination.index,
                        queue_depth=destination.queue.length(),
                        queue_capacity=self._settings.queue_size,
                        accepted=destination.accepted,
                        delivered=destination.delivered,
                        dropped=destination.dropped,
                        dropped_by_reason=self._safe_reasons(destination.dropped_by_reason),
                        failed_events=destination.failed,
                        export_failures=destination.export_failures,
                        attempts=destination.attempts,
                        retries=destination.retries,
                        batches=destination.batches,
                        last_batch_size=destination.last_batch_size,
                        max_batch_size=destination.max_batch_size,
                        last_export_latency_ms=destination.last_latency_ms,
                        max_export_latency_ms=destination.max_latency_ms,
                        total_export_latency_ms=destination.total_latency_ms,
                        last_success_at_ms=destination.last_success_at_ms,
                        configuration_errors=destination.configuration_errors,
                        serialization_failures=destination.serialization_failures,
                        last_flush=destination.last_flush,
                        last_reason=sanitize_reason(destination.last_reason) if destination.last_reason else "",
                    )
                )
        channel = self._diagnostics.counters() if self._diagnostics is not None else ChannelDiagnostics()
        return DiagnosticsSnapshot(destinations=tuple(destinations), channel=channel)

    @staticmethod
    def _safe_reasons(by_reason: Mapping[str, int]) -> dict[str, int]:
        """Copy the drops by reason with each reason sanitized (reasons that become equal add up)."""
        safe: dict[str, int] = {}
        for reason, count in by_reason.items():
            key = sanitize_reason(reason)
            safe[key] = safe.get(key, 0) + count
        return safe

    def _wall_milliseconds(self) -> int:
        """The wall clock in unix milliseconds."""
        return int(self._clock.now().value.timestamp() * _MILLISECONDS_PER_SECOND)

    def _publish(self, destination: _Destination, code: DiagnosticCode, reason: str, count: int) -> None:
        """Hand one diagnostic to the channel; call it with NO lock of the service held. It never raises.

        The diagnostic goes to the channel and never to the pipeline, a queue or a destination, so a failure of the
        delivery cannot create an event that fails again (REQ-DGN-003). It holds a code, the position of the
        destination, a reason and a count, and nothing of an event.
        """
        if self._diagnostics is None:
            return
        try:
            self._diagnostics.publish(
                DiagnosticEvent(
                    code=code,
                    kind=destination.kind,
                    index=destination.index,
                    reason=reason,
                    count=count,
                    at_ms=self._wall_milliseconds(),
                )
            )
        except Exception:  # a diagnostic is never worth a failure of the delivery (ADR-005)
            return

    def _work(self, destination: _Destination) -> None:
        """The loop of a worker: wait for a batch, deliver it with no lock held while the sink works, repeat."""
        while True:
            batch = self._next_batch(destination)
            if batch is None:
                return
            self._deliver(destination, batch)

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
                    destination.last_batch_size = len(batch)
                    destination.max_batch_size = max(destination.max_batch_size, len(batch))
                    destination.room.notify_all()  # there is room now: a caller that waits for it offers again
                    return batch
                if plan.action == PLAN_IDLE:
                    destination.condition.wait()
                else:
                    destination.condition.wait(plan.wait_ms / _MILLISECONDS_PER_SECOND)
        return None

    def _deliver(self, destination: _Destination, batch: list[Envelope]) -> None:
        """Deliver one batch by the delivery mode; it ends when the batch has an outcome that is counted.

        ``best_effort`` calls the sink ONCE whatever the answer. ``retry`` calls it again, with the WHOLE batch, while
        the answer is a transient failure (``failed > 0`` and ``dropped == 0``) and the attempts are fewer than
        ``retry_max_attempts``; between two attempts it waits the backoff with jitter or the ``retry_after_ms`` of the
        answer, whichever is longer. Only the answer that ENDS the batch counts its accepted events as delivered. When
        the limit is reached the failed events are dropped with ``retry_limit_reached``. A stop during the wait
        abandons the batch: its events are dropped with the reason ``shutdown``.
        """
        settings = self._settings
        retries_enabled = settings.mode is DeliveryMode.RETRY
        attempt = 0
        while True:
            accepted, dropped, failed, reason, retry_after_ms, latency_ms = self._call_sink(destination, batch)
            transient = failed > 0 and dropped == 0
            retry = False
            wait_ms = 0
            if retries_enabled and transient:
                wait_ms, retry = retry_delay(
                    attempt,
                    settings.retry_max_attempts,
                    True,
                    retry_after_ms,
                    settings.retry_base_delay_milliseconds,
                    settings.retry_max_delay_milliseconds,
                    self._random_value(),
                )
            self._count_attempt(destination, attempt, accepted, dropped, failed, reason, latency_ms)
            if failed > 0:
                self._publish(destination, DiagnosticCode.EXPORT_FAILED, reason, failed)
            if not retry:
                limit_reached = retries_enabled and transient
                self._finish(destination, batch, accepted, dropped, failed, reason, limit_reached)
                return
            if not self._wait_before_retry(destination, wait_ms):
                self._abandon(destination, batch)
                return
            attempt += 1

    @staticmethod
    def _call_sink(destination: _Destination, batch: list[Envelope]) -> tuple[int, int, int, str, int, int]:
        """Call the sink once, contained: whatever it raises is a failure of the batch and is never kept or printed.

        The last number of the answer is the time of the call in whole milliseconds (``time.monotonic``).
        """
        started = time.monotonic()
        try:
            result = destination.sink.send(batch)  # type: ignore[attr-defined]
            answer = result.accepted, result.dropped, result.failed, result.reason, result.retry_after_ms
        except Exception:  # a sink never stops its worker or the others (ADR-005); the text could hold event data
            answer = 0, 0, len(batch), REASON_SINK_PANICKED, 0
        return (*answer, int((time.monotonic() - started) * _MILLISECONDS_PER_SECOND))

    def _count_attempt(
        self,
        destination: _Destination,
        attempt: int,
        accepted: int,
        dropped: int,
        failed: int,
        reason: str,
        latency_ms: int,
    ) -> None:
        """Count one call to the sink: the attempt, its latency, the export failure and the kind of problem."""
        with destination.condition:
            destination.attempts += 1
            if attempt > 0:
                destination.retries += 1
            destination.last_latency_ms = latency_ms
            destination.max_latency_ms = max(destination.max_latency_ms, latency_ms)
            destination.total_latency_ms += latency_ms
            if failed > 0 or dropped > 0:
                destination.export_failures += 1
            if reason == REASON_SERIALIZATION_FAILED:
                destination.serialization_failures += failed + dropped
            elif reason == REASON_CONFIGURATION_ERROR:
                destination.configuration_errors += failed + dropped

    def _random_value(self) -> float:
        """The next number of the random source; a source that raises gives the middle, so the worker never dies."""
        try:
            return float(self._random.random())
        except Exception:  # a jitter is never worth a failure (ADR-005)
            return 0.5

    @staticmethod
    def _wait_before_retry(destination: _Destination, wait_ms: int) -> bool:
        """Wait ``wait_ms`` on the condition of the destination; False when a stop ended the wait first.

        A real wait that ``stop`` wakes (never a loop of sleeps): the batch stays in flight for the whole time, and
        the notifications of an offer or of a flush only make it look at the clock again.
        """
        deadline = time.monotonic() + wait_ms / _MILLISECONDS_PER_SECOND
        with destination.condition:
            while not destination.stopping:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    return True
                destination.condition.wait(remaining)
        return False

    def _finish(
        self,
        destination: _Destination,
        batch: list[Envelope],
        accepted: int,
        dropped: int,
        failed: int,
        reason: str,
        limit_reached: bool,
    ) -> None:
        """Count the answer that ENDS a batch: what was accepted, dropped or failed, and let a flush look again.

        The drops are published as diagnostics after the lock is released: the code of the reason when the sink
        dropped events (``serialization_failed``, ``configuration_error`` or else ``events_dropped``) and
        ``events_dropped`` with ``retry_limit_reached`` when the batch ran out of attempts.
        """
        succeeded_at_ms = self._wall_milliseconds() if accepted > 0 else 0
        with destination.condition:
            destination.batches += 1
            destination.delivered += accepted
            if accepted > 0:
                destination.last_success_at_ms = succeeded_at_ms  # the events the answer that ENDS a batch accepted
            destination.finished += len(batch)
            destination.in_flight = 0
            destination.settled.notify_all()
            if limit_reached:
                destination.count_drop(REASON_RETRY_LIMIT_REACHED, failed)
            else:
                destination.failed += failed
                if dropped:
                    destination.count_drop(reason or REASON_UNSPECIFIED, dropped)
                if reason and failed:
                    destination.last_reason = reason
        if limit_reached:
            self._publish(destination, DiagnosticCode.EVENTS_DROPPED, REASON_RETRY_LIMIT_REACHED, failed)
        elif dropped:
            code = DiagnosticCode.EVENTS_DROPPED
            if reason == REASON_SERIALIZATION_FAILED:
                code = DiagnosticCode.SERIALIZATION_FAILED
            elif reason == REASON_CONFIGURATION_ERROR:
                code = DiagnosticCode.CONFIGURATION_ERROR
            self._publish(destination, code, reason, dropped)

    def _abandon(self, destination: _Destination, batch: list[Envelope]) -> None:
        """Count a batch that a stop ended during the wait of its retry: all its events dropped as ``shutdown``."""
        with destination.condition:
            destination.batches += 1
            destination.finished += len(batch)
            destination.in_flight = 0
            destination.count_drop(REASON_SHUTDOWN, len(batch))
            destination.settled.notify_all()
        self._publish(destination, DiagnosticCode.EVENTS_DROPPED, REASON_SHUTDOWN, len(batch))
