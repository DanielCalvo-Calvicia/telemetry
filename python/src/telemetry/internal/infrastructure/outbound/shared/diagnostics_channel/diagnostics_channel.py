"""The diagnostics channel: a rate limit, a bounded queue and ONE daemon thread that calls the consumer."""

import collections
import dataclasses
import threading
from typing import Callable, Iterable, Optional

from telemetry.internal.application.dtos.channel_diagnostics import ChannelDiagnostics
from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent
from telemetry.internal.application.ports.outbound.diagnostics_channel_port import (
    DiagnosticsCallback,
    DiagnosticsWriter,
)
from telemetry.internal.domain.operations.diagnostics_rate.plan_diagnostic import NO_WINDOW, plan_diagnostic
from telemetry.internal.domain.operations.diagnostics_reason.sanitize_reason import sanitize_reason
from telemetry.internal.domain.rules import (
    DEFAULT_DIAGNOSTICS_QUEUE_SIZE,
    DEFAULT_DIAGNOSTICS_RATE_LIMIT,
    DEFAULT_DIAGNOSTICS_RATE_WINDOW_MILLISECONDS,
)
from telemetry.internal.infrastructure.outbound.shared.diagnostics_channel.helpers.format_line import format_line

THREAD_NAME = "telemetry-diagnostics"


class DiagnosticsChannel:
    """DiagnosticsChannel implements DiagnosticsChannelPort; it is NOT a destination and never takes a telemetry event.

    ``publish`` takes the monotonic millisecond, asks ``plan_diagnostic`` (limit and window of the defaults) under a
    lock and either counts the diagnostic as suppressed or puts it, with the number suppressed before it, in a bounded
    queue; a full queue counts a drop. It never waits and never raises. ONE daemon thread, started by ``start`` and
    never by the constructor, takes the diagnostics in order and calls every callback and then the writer with one
    line. A callback or a writer that raises is contained (what it raised is never kept or printed) and counted in
    ``callback_errors``: it creates no diagnostic. A callback that never returns blocks only this thread, so the queue
    fills and then drops. Without a callback and a writer no thread is started (the diagnostics are still counted).
    ``stop`` does not wait for the thread: it ends once the diagnostics that were queued are handed over, or never when
    a callback hangs (it is a daemon thread). After ``stop`` a ``publish`` does nothing.
    """

    def __init__(
        self,
        callbacks: Iterable[DiagnosticsCallback],
        writer: Optional[DiagnosticsWriter],
        read_monotonic_milliseconds: Callable[[], int],
        limit: int = DEFAULT_DIAGNOSTICS_RATE_LIMIT,
        window_milliseconds: int = DEFAULT_DIAGNOSTICS_RATE_WINDOW_MILLISECONDS,
        queue_size: int = DEFAULT_DIAGNOSTICS_QUEUE_SIZE,
    ) -> None:
        """Create the channel; nothing runs until ``start()``. ``read_monotonic_milliseconds`` times the limit."""
        self._callbacks = tuple(callbacks)
        self._writer = writer
        self._read_monotonic_milliseconds = read_monotonic_milliseconds
        self._limit = limit
        self._window_milliseconds = window_milliseconds
        self._queue_size = queue_size
        self._condition = threading.Condition()
        self._queue: collections.deque[DiagnosticEvent] = collections.deque()
        self._window_start_ms = NO_WINDOW
        self._count = 0
        self._pending_suppressed = 0
        self._started = False
        self._stopped = False
        self._published = 0
        self._suppressed_total = 0
        self._dropped = 0
        self._callback_errors = 0

    def publish(self, event: DiagnosticEvent) -> None:
        """Offer one diagnostic; it never blocks and never raises, and the rate limit and the queue may drop it."""
        try:
            now_ms = self._read_monotonic_milliseconds()
            with self._condition:
                if self._stopped:
                    return
                plan = plan_diagnostic(
                    now_ms,
                    self._window_start_ms,
                    self._count,
                    self._pending_suppressed,
                    self._limit,
                    self._window_milliseconds,
                )
                self._window_start_ms = plan.window_start_ms
                self._count = plan.count
                self._pending_suppressed = plan.suppressed
                if not plan.allow:
                    self._suppressed_total += 1
                    return
                if not self._callbacks and self._writer is None:
                    self._published += 1  # nobody listens: it is counted and goes nowhere
                    return
                if len(self._queue) >= self._queue_size:
                    self._dropped += 1
                    return
                self._queue.append(
                    dataclasses.replace(event, reason=sanitize_reason(event.reason), suppressed=plan.attach_suppressed)
                )
                self._published += 1
                self._condition.notify()
        except Exception:  # a diagnostic is never worth a failure of the caller (ADR-005)
            return

    def start(self) -> None:
        """Start the thread of the callbacks; it does nothing twice, after ``stop`` or with nobody to call."""
        with self._condition:
            if self._started or self._stopped:
                return
            self._started = True
            if not self._callbacks and self._writer is None:
                return
            threading.Thread(target=self._dispatch_loop, name=THREAD_NAME, daemon=True).start()

    def stop(self) -> None:
        """Tell the thread to end once the queued diagnostics are handed over; it never waits for a callback."""
        with self._condition:
            self._stopped = True
            self._condition.notify_all()

    def counters(self) -> ChannelDiagnostics:
        """Return a copy of the counters of the channel."""
        with self._condition:
            return ChannelDiagnostics(
                published=self._published,
                suppressed=self._suppressed_total,
                dropped=self._dropped,
                callback_errors=self._callback_errors,
            )

    def _dispatch_loop(self) -> None:
        """Take the diagnostics in order and hand each one over, with no lock held while the consumer works."""
        while True:
            with self._condition:
                while not self._queue and not self._stopped:
                    self._condition.wait()
                if not self._queue:
                    return
                event = self._queue.popleft()
            self._hand_over(event)

    def _hand_over(self, event: DiagnosticEvent) -> None:
        """Call every callback in order and then the writer; whatever one raises is counted and nothing else."""
        for callback in self._callbacks:
            self._contained(callback, event)
        if self._writer is not None:
            self._contained(self._write_line, event)

    def _write_line(self, event: DiagnosticEvent) -> None:
        """Write the line of the diagnostic to the writer."""
        self._writer.write(format_line(event))  # type: ignore[union-attr]

    def _contained(self, call: DiagnosticsCallback, event: DiagnosticEvent) -> None:
        """Call the consumer; anything it raises (even SystemExit) is counted and never kept, so the thread lives."""
        try:
            call(event)
        except BaseException:  # the value could hold anything, so it is neither kept nor printed
            with self._condition:
                self._callback_errors += 1
