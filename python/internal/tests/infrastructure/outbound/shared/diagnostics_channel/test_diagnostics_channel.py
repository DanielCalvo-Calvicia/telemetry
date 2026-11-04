"""Black-box tests of the diagnostics channel: passivity, the rate limit, the dispatcher thread and its containment."""

import io
import threading
import time
import unittest
from typing import Any, Optional

from internal.tests.application.services.diagnostics_helpers import Collector, new_channel
from internal.tests.eventually import eventually
from internal.tests.fakes import FakeClock
from telemetry.internal.application.dtos.channel_diagnostics import ChannelDiagnostics
from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent
from telemetry.internal.application.ports.outbound.diagnostics_channel_port import DiagnosticsChannelPort
from telemetry.internal.composition_root.dependencies.diagnostics_channel_dependencies import new_diagnostics_channel
from telemetry.internal.domain.value_objects.enums_generated import DiagnosticCode
from telemetry.internal.infrastructure.outbound.shared.diagnostics_channel.diagnostics_channel import (
    THREAD_NAME,
    DiagnosticsChannel,
)


def diagnostic(number: int = 0, reason: str = "status_503") -> DiagnosticEvent:
    return DiagnosticEvent(DiagnosticCode.EXPORT_FAILED, "log", 0, reason, number, at_ms=1)


def dispatcher_threads() -> list[threading.Thread]:
    return [thread for thread in threading.enumerate() if thread.name == THREAD_NAME and thread.is_alive()]


class ThreadTestCase(unittest.TestCase):
    """A test case that sees only the dispatcher threads that appeared after it began (other tests may leave some)."""

    def setUp(self) -> None:
        self.baseline = set(dispatcher_threads())

    def mine(self) -> list[threading.Thread]:
        return [thread for thread in dispatcher_threads() if thread not in self.baseline]


class FakeMilliseconds:
    """A monotonic clock in milliseconds that the test advances."""

    def __init__(self) -> None:
        self.value = 0

    def __call__(self) -> int:
        return self.value


class Hanging:
    """A callback that never returns until the test releases it; it counts the calls it received."""

    def __init__(self) -> None:
        self.release = threading.Event()
        self.entered = threading.Event()
        self.calls = 0

    def __call__(self, event: DiagnosticEvent) -> None:
        self.calls += 1
        self.entered.set()
        self.release.wait(30)


class PassivityTest(ThreadTestCase):
    def test_constructing_a_channel_starts_no_thread(self):
        channel = new_channel(self, [Collector()])
        self.assertEqual(self.mine(), [])
        channel.publish(diagnostic())
        self.assertEqual(self.mine(), [])

    def test_a_channel_with_nobody_to_call_starts_no_thread_and_still_counts(self):
        clock = FakeMilliseconds()
        channel = new_channel(self, read_monotonic_milliseconds=clock, limit=2)
        channel.start()
        self.assertEqual(self.mine(), [])
        for number in range(5):
            channel.publish(diagnostic(number))
        self.assertEqual(
            channel.counters(), ChannelDiagnostics(published=2, suppressed=3, dropped=0, callback_errors=0)
        )

    def test_start_starts_one_thread_and_twice_starts_no_second(self):
        channel = new_channel(self, [Collector()])
        channel.start()
        channel.start()
        threads = self.mine()
        self.assertEqual(len(threads), 1)
        self.assertTrue(threads[0].daemon)
        channel.stop()
        eventually(5.0, lambda: self.mine() == [], "the dispatcher to end")

    def test_after_stop_publish_does_nothing_and_start_does_not_start_again(self):
        collector = Collector()
        channel = new_channel(self, [collector])
        channel.start()
        channel.stop()
        channel.stop()
        eventually(5.0, lambda: self.mine() == [], "the dispatcher to end")
        channel.publish(diagnostic())
        channel.start()
        self.assertEqual((channel.counters(), self.mine(), collector.events), (ChannelDiagnostics(), [], []))

    def test_a_channel_stopped_before_it_started_never_starts(self):
        channel = new_channel(self, [Collector()])
        channel.stop()
        channel.start()
        self.assertEqual(self.mine(), [])

    def test_the_channel_satisfies_its_port(self):
        self.assertIsInstance(new_channel(self), DiagnosticsChannelPort)


class DeliveryOfDiagnosticsTest(ThreadTestCase):
    def test_the_callbacks_receive_the_diagnostics_in_order_and_each_callback_in_the_order_given(self):
        calls: list[tuple[str, int]] = []
        lock = threading.Lock()

        def named(name: str) -> Any:
            def callback(event: DiagnosticEvent) -> None:
                with lock:
                    calls.append((name, event.count))

            return callback

        channel = new_channel(self, [named("first"), named("second")])
        channel.start()
        for number in range(3):
            channel.publish(diagnostic(number))
        eventually(5.0, lambda: len(calls) == 6, "six calls")
        self.assertEqual(calls, [("first", 0), ("second", 0), ("first", 1), ("second", 1), ("first", 2), ("second", 2)])
        self.assertEqual(channel.counters().published, 3)

    def test_the_writer_gets_one_line_per_diagnostic_after_the_callbacks(self):
        order: list[str] = []

        class Writer:
            def write(self, text: str) -> int:
                order.append("writer:" + text)
                return len(text)

        channel = new_channel(self, [lambda event: order.append("callback")], Writer())
        channel.start()
        channel.publish(diagnostic(3, "timeout"))
        eventually(5.0, lambda: len(order) == 2, "the callback and the writer")
        self.assertEqual(
            order,
            [
                "callback",
                "writer:telemetry diagnostics: code=export_failed destination=log/0 "
                "reason=timeout count=3 suppressed=0\n",
            ],
        )

    def test_a_writer_alone_is_enough_to_start_the_thread(self):
        buffer = io.StringIO()
        channel = new_channel(self, writer=buffer)
        channel.start()
        self.assertEqual(len(self.mine()), 1)
        channel.publish(diagnostic(1))
        eventually(5.0, lambda: buffer.getvalue() != "", "the line")

    def test_the_reason_is_sanitized_and_the_suppressed_is_the_one_of_the_limiter(self):
        collector = Collector()
        channel = new_channel(self, [collector])
        channel.start()
        event = DiagnosticEvent(DiagnosticCode.EVENTS_DROPPED, "log", 1, "a b://c", 2, suppressed=99, at_ms=5)
        channel.publish(event)
        received = collector.wait_for(1)[0]
        self.assertEqual(
            (received.reason, received.suppressed, received.at_ms, received.kind, received.index),
            ("a_b:__c", 0, 5, "log", 1),
        )

    def test_what_was_published_before_start_is_delivered_after_it(self):
        collector = Collector()
        channel = new_channel(self, [collector])
        channel.publish(diagnostic(1))
        channel.publish(diagnostic(2))
        self.assertEqual(collector.events, [])
        channel.start()
        self.assertEqual([event.count for event in collector.wait_for(2)], [1, 2])

    def test_stop_hands_over_what_is_queued_before_the_thread_ends(self):
        collector = Collector()
        channel = new_channel(self, [collector])
        for number in range(5):
            channel.publish(diagnostic(number))
        channel.start()
        channel.stop()
        self.assertEqual([event.count for event in collector.wait_for(5)], [0, 1, 2, 3, 4])
        eventually(5.0, lambda: self.mine() == [], "the dispatcher to end")


class RateLimitTest(ThreadTestCase):
    def test_a_storm_in_one_window_lets_ten_through_and_the_next_window_reports_the_rest(self):
        clock = FakeMilliseconds()
        collector = Collector()
        channel = new_channel(self, [collector], read_monotonic_milliseconds=clock, limit=10, queue_size=1000)
        channel.start()
        for number in range(1000):
            channel.publish(diagnostic(number))
        collector.wait_for(10)
        self.assertEqual(
            channel.counters(), ChannelDiagnostics(published=10, suppressed=990, dropped=0, callback_errors=0)
        )
        self.assertEqual([event.count for event in collector.events], list(range(10)))
        self.assertEqual({event.suppressed for event in collector.events}, {0})
        clock.value = 1000
        channel.publish(diagnostic(1000))
        found = collector.wait_for(11)[10]
        self.assertEqual((found.count, found.suppressed), (1000, 990))
        channel.publish(diagnostic(1001))
        self.assertEqual(collector.wait_for(12)[11].suppressed, 0)

    def test_the_defaults_are_ten_per_second_and_a_queue_of_64(self):
        clock = FakeMilliseconds()
        channel = DiagnosticsChannel([Collector()], None, clock)
        self.addCleanup(channel.stop)
        for number in range(100):
            channel.publish(diagnostic(number))
        self.assertEqual((channel.counters().published, channel.counters().suppressed), (10, 90))

    def test_the_dependencies_time_the_limit_with_the_monotonic_clock(self):
        clock = FakeClock(step_nanoseconds=0)
        channel = new_diagnostics_channel([Collector()], None, clock)
        self.addCleanup(channel.stop)
        for number in range(12):
            channel.publish(diagnostic(number))
        self.assertEqual((channel.counters().published, channel.counters().suppressed), (10, 2))
        clock.set_step(1_000_000_000)  # one second of the monotonic clock on each read: a new window
        channel.publish(diagnostic(13))
        self.assertEqual(channel.counters().published, 11)


class ContainmentTest(ThreadTestCase):
    def test_a_callback_that_raises_is_counted_and_the_others_and_the_writer_still_run(self):
        collector = Collector()
        buffer = io.StringIO()

        def raising(event: DiagnosticEvent) -> None:
            raise RuntimeError("secret inside the exception")

        def exiting(event: DiagnosticEvent) -> None:
            raise SystemExit(3)

        channel = new_channel(self, [raising, exiting, collector], buffer)
        channel.start()
        channel.publish(diagnostic(1))
        channel.publish(diagnostic(2))
        collector.wait_for(2)
        eventually(5.0, lambda: buffer.getvalue().count("\n") == 2, "two lines")
        eventually(5.0, lambda: channel.counters().callback_errors == 4, "four errors counted")
        self.assertEqual(channel.counters().published, 2)
        self.assertEqual(len(self.mine()), 1)

    def test_a_writer_that_raises_is_counted_and_the_thread_goes_on(self):
        collector = Collector()

        class Broken:
            def write(self, text: str) -> int:
                raise OSError("disk full: secret")

        channel = new_channel(self, [collector], Broken())
        channel.start()
        channel.publish(diagnostic(1))
        channel.publish(diagnostic(2))
        collector.wait_for(2)
        eventually(5.0, lambda: channel.counters().callback_errors == 2, "two errors counted")

    def test_a_failure_of_a_callback_creates_no_diagnostic(self):
        collector = Collector()
        channel = new_channel(self, [lambda event: 1 / 0, collector])
        channel.start()
        channel.publish(diagnostic(1))
        collector.wait_for(1)
        eventually(5.0, lambda: channel.counters().callback_errors == 1, "the error counted")
        time.sleep(0.05)
        self.assertEqual((len(collector.events), channel.counters().published), (1, 1))

    def test_a_clock_that_raises_never_fails_publish(self):
        def broken_clock() -> int:
            raise RuntimeError("no clock")

        channel = new_channel(self, [Collector()], read_monotonic_milliseconds=broken_clock)
        channel.publish(diagnostic(1))
        self.assertEqual(channel.counters(), ChannelDiagnostics())

    def test_a_callback_that_hangs_blocks_only_the_dispatcher_and_the_queue_drops(self):
        hanging = Hanging()
        self.addCleanup(hanging.release.set)
        channel = new_channel(self, [hanging], queue_size=64)
        channel.start()
        channel.publish(diagnostic(0))
        self.assertTrue(hanging.entered.wait(5))
        started = time.monotonic()
        for number in range(1, 201):
            channel.publish(diagnostic(number))
        self.assertLess(time.monotonic() - started, 1.0, "publish never waits for the callback")
        counters = channel.counters()
        self.assertEqual((counters.published, counters.dropped), (65, 136))
        self.assertEqual(hanging.calls, 1)

    def test_stop_does_not_wait_for_a_callback_that_hangs(self):
        hanging = Hanging()
        channel = new_channel(self, [hanging])
        channel.start()
        channel.publish(diagnostic(0))
        self.assertTrue(hanging.entered.wait(5))
        started = time.monotonic()
        channel.stop()
        self.assertLess(time.monotonic() - started, 1.0)
        self.assertEqual(len(self.mine()), 1, "the thread is abandoned, it is a daemon")
        hanging.release.set()
        eventually(5.0, lambda: self.mine() == [], "the dispatcher to end once released")


class ConcurrencyTest(ThreadTestCase):
    def test_many_threads_publishing_are_counted_once_each(self):
        collector = Collector()
        channel = new_channel(self, [collector], limit=10_000, queue_size=10_000)
        channel.start()

        def publish_many() -> None:
            for number in range(250):
                channel.publish(diagnostic(number))

        workers = [threading.Thread(target=publish_many) for _ in range(8)]
        for worker in workers:
            worker.start()
        for worker in workers:
            worker.join()
        collector.wait_for(2000)
        counters: Optional[ChannelDiagnostics] = channel.counters()
        self.assertEqual(counters, ChannelDiagnostics(published=2000, suppressed=0, dropped=0, callback_errors=0))


if __name__ == "__main__":
    unittest.main()
