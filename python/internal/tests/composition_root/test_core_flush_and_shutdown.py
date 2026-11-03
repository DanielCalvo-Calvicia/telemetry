"""Black-box tests of ``flush`` and ``shutdown`` of the core built by ``telemetry.new`` (T-05.4, REQ-DLV-010).

There is no synchronous send: a program MUST call ``shutdown()`` before it exits. The time is real and the bounds are
generous; no test waits for the 5 s default.
"""

import asyncio
import inspect
import threading
import time
import unittest
from typing import Any
from unittest import mock

import telemetry
from internal.tests.composition_root.test_core import build, config_for, log_event, received
from internal.tests.eventually import with_short_flush
from internal.tests.fakes import BlockingLogSink, FakeLogSink
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.domain.rules import DEFAULT_SHUTDOWN_TIMEOUT_MILLISECONDS

LONG_FLUSH_MS = 60_000


def long_config() -> telemetry.Config:
    """A configuration whose flush interval is 60 s: only a flush or a shutdown makes a partial batch leave."""
    return with_short_flush(config_for(), LONG_FLUSH_MS)


class FailingSink:
    def send(self, events: Any) -> SendResult:
        return SendResult(accepted=0, dropped=0, failed=len(events), reason="boom")


class FlushOfTheCoreTest(unittest.TestCase):
    def test_flush_sends_the_queued_events_and_the_core_keeps_running(self):
        sink = FakeLogSink()
        core = build(self, long_config(), telemetry.with_log_sink(sink))
        core.emit(log_event("one"))
        core.emit(log_event("two"))
        self.assertEqual(core.flush(1000), telemetry.ShutdownResult(sent=2, spooled=0, dropped=0))
        self.assertEqual(sum(len(batch) for batch in sink.batches), 2)
        core.logger.info(telemetry.LogEntry(category="http", message="after the flush"))  # still accepted
        result = core.shutdown(1000)
        self.assertEqual(result, telemetry.ShutdownResult(sent=1, spooled=0, dropped=0))
        received(sink, 3)

    def test_flush_before_any_event_is_zero_and_returns_at_once(self):
        core = build(self, long_config(), telemetry.with_log_sink(FakeLogSink()))
        started = time.monotonic()
        self.assertEqual(core.flush(5000), telemetry.ShutdownResult())
        self.assertLess(time.monotonic() - started, 1.0)

    def test_flush_with_a_timeout_of_zero_counts_everything_pending_as_dropped(self):
        core = build(self, long_config(), telemetry.with_log_sink(FakeLogSink()))
        for number in range(3):
            core.emit(log_event(str(number)))
        self.assertEqual(core.flush(0), telemetry.ShutdownResult(sent=0, spooled=0, dropped=3))
        self.assertEqual(core.flush(-1), telemetry.ShutdownResult(sent=0, spooled=0, dropped=3))

    def test_flush_with_a_failing_destination_counts_the_events_as_dropped(self):
        core = build(
            self, long_config(), telemetry.with_log_sink(FailingSink()), telemetry.with_log_sink(FakeLogSink())
        )
        core.emit(log_event())
        self.assertEqual(core.flush(1000), telemetry.ShutdownResult(sent=1, spooled=0, dropped=1))

    def test_flush_never_raises(self):
        core = build(self, long_config(), telemetry.with_log_sink(FakeLogSink()))
        with mock.patch.object(core._delivery, "flush", side_effect=RuntimeError("broken")):
            self.assertEqual(core.flush(100), telemetry.ShutdownResult())
        with mock.patch.object(core._delivery, "shutdown", side_effect=RuntimeError("broken")):
            self.assertEqual(core.shutdown(100), telemetry.ShutdownResult())


class ShutdownOfTheCoreTest(unittest.TestCase):
    def test_shutdown_sends_what_is_queued_and_returns_before_its_timeout(self):
        sink = FakeLogSink()
        core = build(self, long_config(), telemetry.with_log_sink(sink))
        for number in range(5):
            core.emit(log_event(str(number)))
        started = time.monotonic()
        result = core.shutdown(2000)
        self.assertLess(time.monotonic() - started, 1.0)
        self.assertEqual(result, telemetry.ShutdownResult(sent=5, spooled=0, dropped=0))
        self.assertEqual(sum(len(batch) for batch in sink.batches), 5)

    def test_a_second_shutdown_returns_the_first_result_and_a_flush_after_it_is_zero(self):
        core = build(
            self, long_config(), telemetry.with_log_sink(FakeLogSink()), telemetry.with_log_sink(FailingSink())
        )
        core.emit(log_event())
        first = core.shutdown(1000)
        self.assertEqual(first, telemetry.ShutdownResult(sent=1, spooled=0, dropped=1))
        self.assertEqual(core.shutdown(1000), first)
        self.assertEqual(core.shutdown(), first)
        self.assertEqual(core.flush(1000), telemetry.ShutdownResult())

    def test_shutdown_before_any_event_is_zero_twice(self):
        core = build(self, long_config(), telemetry.with_log_sink(FakeLogSink()))
        self.assertEqual(core.shutdown(1000), telemetry.ShutdownResult())
        self.assertEqual(core.shutdown(1000), telemetry.ShutdownResult())

    def test_a_core_without_destinations_shuts_down(self):
        core = build(self, long_config())
        self.assertEqual(core.shutdown(1000), telemetry.ShutdownResult())
        result = core.emit(log_event())
        self.assertEqual((result.dropped, result.reason), (1, "shutdown"))

    def test_an_event_after_shutdown_is_dropped_with_the_reason_shutdown_and_counted(self):
        first, second = FakeLogSink(), FakeLogSink()
        core = build(self, long_config(), telemetry.with_log_sink(first), telemetry.with_log_sink(second))
        core.shutdown(1000)
        result = core.emit(log_event())
        self.assertEqual(result, telemetry.EmitResult(accepted=0, dropped=1, failed=0, reason="shutdown"))
        core.logger.error(telemetry.LogEntry(category="http", message="late"))
        invalid = core.emit(telemetry.NewEventInput(event_type="", severity=telemetry.Severity.INFO, data={}))
        self.assertEqual((invalid.dropped, invalid.reason), (1, "shutdown"))
        for stats in core._delivery.stats():
            self.assertEqual((stats.accepted, stats.dropped, stats.dropped_by_reason), (0, 2, {"shutdown": 2}))
        self.assertEqual((first.calls, second.calls), (0, 0))

    def test_the_timeout_has_a_default_of_five_seconds_and_the_call_without_it_works(self):
        self.assertEqual(DEFAULT_SHUTDOWN_TIMEOUT_MILLISECONDS, 5000)
        for name in ("flush", "shutdown"):
            parameter = inspect.signature(getattr(telemetry.Telemetry, name)).parameters["timeout_ms"]
            self.assertEqual(parameter.default, DEFAULT_SHUTDOWN_TIMEOUT_MILLISECONDS)
        sink = FakeLogSink()
        core = build(self, long_config(), telemetry.with_log_sink(sink))
        core.emit(log_event())
        started = time.monotonic()
        self.assertEqual(core.flush(), telemetry.ShutdownResult(sent=1))
        core.emit(log_event())
        self.assertEqual(core.shutdown(), telemetry.ShutdownResult(sent=1))
        self.assertLess(time.monotonic() - started, 2.0)  # a healthy destination: the default is a limit, not a wait

    def test_a_hanging_destination_holds_shutdown_only_until_its_timeout(self):
        sink = BlockingLogSink()
        core = build(self, long_config(), telemetry.with_log_sink(sink), telemetry.with_log_sink(FakeLogSink()))
        self.addCleanup(sink.release)  # runs before the stop of the workers
        for number in range(4):
            core.emit(log_event(str(number)))
        started = time.monotonic()
        result = core.shutdown(300)
        elapsed = time.monotonic() - started
        self.assertEqual(result, telemetry.ShutdownResult(sent=4, spooled=0, dropped=4))
        self.assertGreaterEqual(elapsed, 0.24)
        self.assertLess(elapsed, 0.6)
        started = time.monotonic()
        self.assertEqual(core.shutdown(300), result)  # the second call does not wait again
        self.assertLess(time.monotonic() - started, 0.2)

    def test_log_calls_from_many_threads_during_shutdown_raise_nothing_and_every_event_is_counted(self):
        sink = FakeLogSink()
        config = with_short_flush(config_for(), 5)
        core = build(self, config, telemetry.with_log_sink(sink))
        threads_count, per_thread = 8, 300
        results: list[list[telemetry.EmitResult]] = [[] for _ in range(threads_count)]
        errors: list[BaseException] = []
        go = threading.Barrier(threads_count + 1)

        def worker(position: int) -> None:
            try:
                go.wait(5.0)
                for number in range(per_thread):
                    results[position].append(core.emit(log_event(str(number))))
            except BaseException as error:  # the test reports any exception of a log call
                errors.append(error)

        threads = [threading.Thread(target=worker, args=(position,)) for position in range(threads_count)]
        for thread in threads:
            thread.start()
        go.wait(5.0)
        time.sleep(0.005)
        shutdown_result = core.shutdown(5000)
        for thread in threads:
            thread.join(10.0)
        self.assertEqual(errors, [])
        flat = [result for per_thread_results in results for result in per_thread_results]
        self.assertEqual(len(flat), threads_count * per_thread)
        accepted = sum(result.accepted for result in flat)
        late = [result for result in flat if result.dropped]
        self.assertTrue(all(result.reason == "shutdown" for result in late))
        self.assertEqual(accepted + len(late), len(flat))
        stats = core._delivery.stats()[0]
        self.assertEqual((stats.accepted, stats.dropped_by_reason.get("shutdown", 0)), (accepted, len(late)))
        self.assertEqual(shutdown_result.dropped, 0)  # a healthy destination: nothing queued was lost
        self.assertEqual(sum(len(batch) for batch in sink.batches), accepted)  # and every accepted event arrived

    def test_shutdown_can_be_called_inside_asyncio_run_and_blocks_the_loop_for_its_duration(self):
        sink = FakeLogSink()
        sink.set_delay(0.1)
        core = build(self, long_config(), telemetry.with_log_sink(sink))

        async def main() -> tuple[telemetry.ShutdownResult, float, str]:
            core.emit(log_event())
            started = time.monotonic()
            result = core.shutdown(2000)  # synchronous: the loop thread is blocked until it returns
            blocked_seconds = time.monotonic() - started
            return result, blocked_seconds, core.emit(log_event()).reason

        result, blocked_seconds, reason_after = asyncio.run(main())
        self.assertEqual(result, telemetry.ShutdownResult(sent=1))
        self.assertGreaterEqual(blocked_seconds, 0.09)  # it waited for the slow destination
        self.assertEqual(reason_after, "shutdown")


if __name__ == "__main__":
    unittest.main()
