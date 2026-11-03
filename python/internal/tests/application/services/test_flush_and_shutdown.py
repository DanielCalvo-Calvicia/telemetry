"""Black-box tests of ``flush`` and ``shutdown`` of DeliveryService over fakes (T-05.4, REQ-DLV-010).

The shared fixture ``integration-tests/fixtures/delivery/shutdown_results.json`` is run for both calls; the rest of the
cases (a flush that keeps running, the second shutdown, the closed service, the timeouts) are one test each. The time is
real; the bounds are generous so a slow machine does not fail them, and a test never waits for the 5 s default.
"""

import json
import pathlib
import threading
import time
import unittest
from typing import Any, Sequence
from unittest import mock

from internal.tests.application.services.test_delivery_service import Item, ItemDeliveryService, received
from internal.tests.eventually import eventually
from internal.tests.fakes import BlockingLogSink, FakeLogSink
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.dtos.shutdown_result import ShutdownResult
from telemetry.internal.application.services.delivery_service import (
    REASON_SHUTDOWN,
    DeliveryService,
    Destinations,
)
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.infrastructure.outbound.shared.system_clock.system_clock import SystemClock

FIXTURE = pathlib.Path(__file__).resolve().parents[5] / "integration-tests" / "fixtures" / "delivery"
LOG = "log"
MARGIN_SECONDS = 0.3


class BehaviorSink:
    """A log sink with the behavior of the fixture: ok (after a delay), fail, drop; it answers for the whole batch."""

    def __init__(self, behavior: str, delay_ms: int = 0) -> None:
        self.behavior = behavior
        self.delay_seconds = delay_ms / 1000
        self.calls = 0

    def send(self, events: Sequence[Any]) -> SendResult:
        self.calls += 1
        if self.delay_seconds:
            time.sleep(self.delay_seconds)
        size = len(events)
        if self.behavior == "fail":
            return SendResult(accepted=0, dropped=0, failed=size, reason="boom")
        if self.behavior == "drop":
            return SendResult(accepted=0, dropped=size, failed=0, reason="refused")
        return SendResult(accepted=size, dropped=0, failed=0)


class RaisingSink:
    """A sink that breaks the rule of ADR-005 on every call."""

    def send(self, events: Sequence[Any]) -> SendResult:
        raise RuntimeError("the sink broke")


class FlushTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self.blocked: list[BlockingLogSink] = []

    def make(
        self,
        sinks: Sequence[Any],
        batch_size: int = 100,
        flush_interval_ms: int = 60_000,
        queue_size: int = 1000,
        start: bool = True,
    ) -> DeliveryService:
        """A service over the sinks with a long flush interval; it is released and stopped when the test ends."""
        settings = DeliverySettings(
            queue_size=queue_size, batch_size=batch_size, flush_interval_milliseconds=flush_interval_ms
        )
        service = ItemDeliveryService(Destinations(log=tuple(sinks)), settings, SystemClock(), new_memory_queue)
        self.addCleanup(self.release_then_stop, service)
        if start:
            service.start()
        return service

    def block(self) -> BlockingLogSink:
        sink = BlockingLogSink()
        self.blocked.append(sink)
        return sink

    def release_then_stop(self, service: DeliveryService) -> None:
        for sink in self.blocked:
            sink.release()
        service.stop()


def sink_for(spec: dict[str, Any], test: FlushTestCase) -> Any:
    """The fake of one destination of the fixture."""
    if spec["behavior"] == "hang":
        return test.block()
    return BehaviorSink(spec["behavior"], spec.get("delay_ms", 0))


def load_cases() -> list[dict[str, Any]]:
    path = FIXTURE / "shutdown_results.json"
    cases = json.loads(path.read_text(encoding="utf-8"))["cases"]
    if len(cases) < 8:
        raise AssertionError(f"{path.name}: expected 8 cases, found {len(cases)}")
    return cases


class SharedFixtureTest(FlushTestCase):
    def run_case(self, case: dict[str, Any], call: str) -> None:
        sinks = [sink_for(spec, self) for spec in case["destinations"]]
        service = self.make(sinks, batch_size=case["batch_size"], flush_interval_ms=case["flush_interval_ms"])
        for number in range(case["events"]):
            service.enqueue(LOG, number)
        timeout_ms = case["timeout_ms"]
        started = time.monotonic()
        result = getattr(service, call)(timeout_ms)
        elapsed = time.monotonic() - started
        expected = case["expected"]
        self.assertEqual(
            {"sent": result.sent, "spooled": result.spooled, "dropped": result.dropped},
            {key: expected[key] for key in ("sent", "spooled", "dropped")},
        )
        timeout_seconds = timeout_ms / 1000
        if expected["returns_before_timeout"]:
            self.assertLess(elapsed, timeout_seconds)
        else:
            self.assertGreaterEqual(elapsed, 0.8 * timeout_seconds)
            self.assertLessEqual(elapsed, timeout_seconds + MARGIN_SECONDS)

    def test_flush_with_every_shared_case(self):
        for case in load_cases():
            with self.subTest(case=case["name"]):
                self.run_case(case, "flush")

    def test_shutdown_with_every_shared_case(self):
        for case in load_cases():
            with self.subTest(case=case["name"]):
                self.run_case(case, "shutdown")


class FlushTest(FlushTestCase):
    def test_a_flush_sends_the_partial_batch_at_once_and_the_service_keeps_running(self):
        sink = FakeLogSink()
        service = self.make([sink])
        service.enqueue(LOG, 1)
        self.assertEqual(service.flush(1000), ShutdownResult(sent=1, spooled=0, dropped=0))
        service.enqueue(LOG, 2)
        self.assertEqual(len(received(sink)), 1)  # the second event waits for the long interval
        self.assertEqual(service.flush(1000), ShutdownResult(sent=1))
        self.assertEqual([event.value for event in received(sink)], [1, 2])
        self.assertFalse(service.closed)

    def test_the_flush_interval_applies_again_when_the_flush_ends(self):
        sink = FakeLogSink()
        service = self.make([sink])
        service.enqueue(LOG, 1)
        service.flush(1000)
        service.enqueue(LOG, 2)
        time.sleep(0.05)
        self.assertEqual(len(received(sink)), 1)  # no flush is pending: the event waits for its interval
        self.assertEqual(service.stats()[0].queued, 1)

    def test_the_target_is_what_was_queued_when_the_call_began(self):
        sink = self.block()
        service = self.make([sink])
        for number in range(3):
            service.enqueue(LOG, number)
        started = threading.Event()
        found: list[ShutdownResult] = []

        def flush() -> None:
            started.set()
            found.append(service.flush(2000))

        thread = threading.Thread(target=flush)
        thread.start()
        started.wait(5.0)
        eventually(5.0, lambda: sink.entered.is_set(), "the sink entered")
        for number in range(3, 8):  # events after the call began do not change its target
            service.enqueue(LOG, number)
        sink.release()
        thread.join(5.0)
        self.assertEqual(found, [ShutdownResult(sent=3, spooled=0, dropped=0)])

    def test_two_flushes_at_the_same_time_both_return_their_result(self):
        sink = BehaviorSink("ok", delay_ms=100)
        service = self.make([sink])
        for number in range(4):
            service.enqueue(LOG, number)
        found: list[ShutdownResult] = []
        threads = [threading.Thread(target=lambda: found.append(service.flush(2000))) for _ in range(2)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(5.0)
        self.assertEqual(len(found), 2)
        self.assertTrue(all(result.sent + result.dropped <= 4 for result in found))
        self.assertEqual(max(result.sent for result in found), 4)

    def test_a_timeout_of_zero_or_less_returns_at_once_with_everything_pending_dropped(self):
        sink = FakeLogSink()
        service = self.make([sink])
        for number in range(3):
            service.enqueue(LOG, number)
        for timeout_ms in (0, -5):
            started = time.monotonic()
            result = service.flush(timeout_ms)
            self.assertLess(time.monotonic() - started, 0.2)
            self.assertEqual(result, ShutdownResult(sent=0, spooled=0, dropped=3))
        self.assertEqual(len(received(sink)), 0)  # nothing was asked to leave

    def test_a_timeout_that_is_not_a_number_is_a_timeout_of_zero(self):
        service = self.make([FakeLogSink()])
        service.enqueue(LOG, 1)
        self.assertEqual(service.flush("soon"), ShutdownResult(dropped=1))  # type: ignore[arg-type]

    def test_a_raising_sink_is_a_failure_and_the_flush_returns_dropped(self):
        service = self.make([RaisingSink(), FakeLogSink()])
        service.enqueue(LOG, 1)
        self.assertEqual(service.flush(1000), ShutdownResult(sent=1, dropped=1))

    def test_a_flush_of_a_service_whose_workers_are_not_running_does_not_wait(self):
        service = self.make([FakeLogSink()], start=False)
        service.enqueue(LOG, 1)
        service.enqueue(LOG, 2)
        started = time.monotonic()
        self.assertEqual(service.flush(5000), ShutdownResult(dropped=2))
        self.assertLess(time.monotonic() - started, 1.0)
        service.start()
        service.stop()
        started = time.monotonic()  # the workers have ended
        self.assertEqual(service.flush(5000), ShutdownResult(dropped=2))
        self.assertLess(time.monotonic() - started, 1.0)

    def test_an_unexpected_error_inside_a_flush_never_reaches_the_caller(self):
        service = self.make([FakeLogSink()])
        with mock.patch.object(DeliveryService, "_drain", side_effect=RuntimeError("broken")):
            self.assertEqual(service.flush(1000), ShutdownResult())


class ShutdownTest(FlushTestCase):
    def test_shutdown_sends_what_is_queued_stops_the_workers_and_closes_the_service(self):
        sink = FakeLogSink()
        service = self.make([sink])
        for number in range(5):
            service.enqueue(LOG, number)
        self.assertEqual(service.shutdown(1000), ShutdownResult(sent=5, spooled=0, dropped=0))
        self.assertEqual(len(received(sink)), 5)
        self.assertTrue(service.closed)
        self.assertFalse(any(thread.is_alive() for thread in self.workers(service)))

    @staticmethod
    def workers(service: DeliveryService) -> list[threading.Thread]:
        return [destination.thread for destination in service._all if destination.thread is not None]

    def test_a_second_shutdown_returns_the_first_result_even_if_the_first_was_not_zero(self):
        service = self.make([BehaviorSink("fail")])
        service.enqueue(LOG, 1)
        first = service.shutdown(1000)
        self.assertEqual(first, ShutdownResult(sent=0, spooled=0, dropped=1))
        self.assertIs(service.shutdown(1000), first)
        self.assertEqual(service.shutdown(0), first)

    def test_a_flush_after_shutdown_returns_the_zero_result(self):
        service = self.make([FakeLogSink()])
        service.enqueue(LOG, 1)
        service.shutdown(1000)
        self.assertEqual(service.flush(1000), ShutdownResult())

    def test_an_event_after_shutdown_is_dropped_with_the_reason_shutdown_and_counted(self):
        service = self.make([FakeLogSink(), FakeLogSink()])
        service.shutdown(1000)
        result = service.enqueue(LOG, 1)
        self.assertEqual((result.accepted, result.dropped, result.reason), (0, 1, REASON_SHUTDOWN))
        for stats in service.stats():
            self.assertEqual((stats.accepted, stats.dropped_by_reason), (0, {REASON_SHUTDOWN: 1}))
        other = service.enqueue("metrics", 1)  # no destination of the kind: the reason is still shutdown
        self.assertEqual((other.dropped, other.reason), (1, REASON_SHUTDOWN))

    def test_a_caller_that_was_on_its_way_when_the_shutdown_began_is_dropped_not_lost(self):
        sink = FakeLogSink()
        service = self.make([sink])
        destination = service._all[0]
        service.shutdown(1000)
        # The check of the closed flag inside the offer: the call passed the first check before the shutdown began.
        self.assertEqual(service._offer(destination, Item(1), mock.ANY, 0), REASON_SHUTDOWN)

    def test_shutdown_before_any_event_returns_zero_at_once(self):
        service = self.make([FakeLogSink()])
        started = time.monotonic()
        self.assertEqual(service.shutdown(5000), ShutdownResult())
        self.assertLess(time.monotonic() - started, 1.0)

    def test_shutdown_of_a_service_that_never_started_does_not_wait(self):
        service = self.make([FakeLogSink()], start=False)
        service.enqueue(LOG, 1)
        started = time.monotonic()
        self.assertEqual(service.shutdown(5000), ShutdownResult(dropped=1))
        self.assertLess(time.monotonic() - started, 1.0)

    def test_a_hanging_destination_does_not_hold_shutdown_past_the_timeout(self):
        sink = self.block()
        service = self.make([sink], batch_size=2, flush_interval_ms=5)
        for number in range(5):
            service.enqueue(LOG, number)
        self.assertTrue(sink.entered.wait(5.0))
        started = time.monotonic()
        result = service.shutdown(300)
        elapsed = time.monotonic() - started
        self.assertEqual(result, ShutdownResult(sent=0, spooled=0, dropped=5))
        self.assertGreaterEqual(elapsed, 0.24)
        self.assertLess(elapsed, 0.3 + MARGIN_SECONDS)
        self.assertTrue(all(thread.daemon for thread in self.workers(service)))  # the abandoned worker is a daemon

    def test_shutdown_with_a_timeout_of_zero_returns_at_once_and_still_closes(self):
        sink = FakeLogSink()
        service = self.make([sink])
        service.enqueue(LOG, 1)
        started = time.monotonic()
        self.assertEqual(service.shutdown(0), ShutdownResult(dropped=1))
        self.assertLess(time.monotonic() - started, 0.5)
        self.assertTrue(service.closed)
        self.assertEqual(service.enqueue(LOG, 2).reason, REASON_SHUTDOWN)

    def test_shutdown_after_stop_still_returns_and_does_not_raise(self):
        service = self.make([FakeLogSink()])
        service.stop()
        self.assertEqual(service.shutdown(100), ShutdownResult())

    def test_an_unexpected_error_inside_a_shutdown_never_reaches_the_caller_and_still_stops(self):
        service = self.make([FakeLogSink()])
        with mock.patch.object(DeliveryService, "_drain", side_effect=RuntimeError("broken")):
            self.assertEqual(service.shutdown(1000), ShutdownResult())
        self.assertFalse(any(thread.is_alive() for thread in self.workers(service)))

    def test_a_shutdown_that_arrives_during_another_waits_and_returns_the_same_result(self):
        sink = BehaviorSink("ok", delay_ms=200)
        service = self.make([sink])
        service.enqueue(LOG, 1)
        found: list[ShutdownResult] = []
        threads = [threading.Thread(target=lambda: found.append(service.shutdown(2000))) for _ in range(3)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(5.0)
        self.assertEqual(found, [ShutdownResult(sent=1)] * 3)


if __name__ == "__main__":
    unittest.main()
