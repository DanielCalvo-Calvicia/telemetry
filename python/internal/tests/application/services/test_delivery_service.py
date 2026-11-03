"""Black-box tests of DeliveryService over fakes: the queue, the worker, the batches, the stats and the lifecycle.

The time is real and short (milliseconds): a worker waits on a condition, so a test waits for the asynchronous fact
with ``eventually`` and never for a fixed time, except to prove that something does NOT happen.
"""

import threading
import time
import unittest
from typing import Any, Optional, Sequence

from internal.tests.eventually import eventually
from internal.tests.fakes import BlockingLogSink, FakeEventSink, FakeLogSink
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.services.delivery_service import (
    REASON_NO_DESTINATION,
    REASON_QUEUE_FULL,
    REASON_SINK_PANICKED,
    DeliveryService,
    Destinations,
    DestinationStats,
)
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.infrastructure.outbound.shared.system_clock.system_clock import SystemClock

WORKER_PREFIX = "telemetry-delivery-"


def worker_threads() -> list[threading.Thread]:
    """The delivery workers that are alive now."""
    return [thread for thread in threading.enumerate() if thread.name.startswith(WORKER_PREFIX)]


def received(sink: Any) -> list:
    """Every event the fake sink has received, in order, whatever the batches."""
    return [event for batch in sink.batches for event in batch]


class RaisingOnceSink:
    """A sink that breaks the rule of ADR-005 on its first call and then behaves."""

    def __init__(self) -> None:
        self.batches: list = []

    def send(self, events: Sequence[Any]) -> SendResult:
        self.batches.append(list(events))
        if len(self.batches) == 1:
            raise RuntimeError("the sink broke with secret hunter2secret")
        return SendResult(accepted=len(events), dropped=0, failed=0)


class DeliveryServiceTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self.blocked: list[BlockingLogSink] = []

    def make(
        self,
        destinations: Destinations,
        queue_size: int = 1000,
        batch_size: int = 100,
        flush_interval_milliseconds: int = 5,
        start: bool = True,
    ) -> DeliveryService:
        """A service over the system clock and the memory queue; it stops when the test ends."""
        settings = DeliverySettings(
            queue_size=queue_size, batch_size=batch_size, flush_interval_milliseconds=flush_interval_milliseconds
        )
        service = DeliveryService(destinations, settings, SystemClock(), new_memory_queue)
        self.addCleanup(self.release_then_stop, service)
        if start:
            service.start()
        return service

    def block(self) -> BlockingLogSink:
        """A hanging destination that is released before the service is stopped at the end of the test."""
        sink = BlockingLogSink()
        self.blocked.append(sink)
        return sink

    def release_then_stop(self, service: DeliveryService) -> None:
        """The cleanup: let the hanging sinks return, so that the join of stop ends, then stop the service."""
        for sink in self.blocked:
            sink.release()
        service.stop()

    def wait_for_events(self, sink: Any, count: int) -> None:
        eventually(5.0, lambda: len(received(sink)) >= count, f"{count} events at the sink")


class LogCallNeverWaitsTest(DeliveryServiceTestCase):
    def test_ten_thousand_enqueues_against_a_destination_that_never_answers_return_at_once(self):
        sink = self.block()
        service = self.make(Destinations(log=(sink,)), queue_size=20_000, batch_size=100)
        service.enqueue("log", "first")
        self.assertTrue(sink.entered.wait(5.0), "the worker never called the sink")
        started = time.monotonic()
        results = [service.enqueue("log", f"event-{number}") for number in range(10_000)]
        elapsed = time.monotonic() - started
        self.assertLess(elapsed, 2.0)
        self.assertTrue(all(result.accepted == 1 and result.dropped == 0 for result in results))
        stats = service.stats()[0]
        self.assertEqual((stats.queued, stats.batches, stats.accepted), (10_000, 0, 10_001))

    def test_a_hanging_destination_does_not_delay_a_second_one(self):
        hanging, healthy = self.block(), FakeLogSink()
        service = self.make(Destinations(log=(hanging, healthy)))
        for number in range(500):
            service.enqueue("log", number)
        self.assertTrue(hanging.entered.wait(5.0))
        self.wait_for_events(healthy, 500)
        self.assertEqual(received(healthy), list(range(500)))
        hanging_stats, healthy_stats = service.stats()
        self.assertEqual((hanging_stats.batches, healthy_stats.queued), (0, 0))
        self.assertGreater(hanging_stats.queued, 0)

    def test_a_hanging_destination_of_one_kind_does_not_delay_another_kind(self):
        hanging, events = self.block(), FakeEventSink()
        service = self.make(Destinations(log=(hanging,), event=(events,)))
        service.enqueue("log", "stuck")
        self.assertTrue(hanging.entered.wait(5.0))
        service.enqueue("event", "free")
        self.wait_for_events(events, 1)

    def test_a_slow_sink_does_not_slow_the_log_call(self):
        slow = FakeLogSink()
        slow.set_delay(0.2)
        service = self.make(Destinations(log=(slow,)), batch_size=10)
        started = time.monotonic()
        for number in range(50):
            service.enqueue("log", number)
        self.assertLess(time.monotonic() - started, 0.15)
        self.wait_for_events(slow, 50)

    def test_many_threads_enqueue_at_once_and_every_event_arrives_once(self):
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,)), queue_size=10_000, batch_size=64)

        def work(number: int) -> None:
            for index in range(200):
                service.enqueue("log", (number, index))

        threads = [threading.Thread(target=work, args=(number,)) for number in range(8)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        self.wait_for_events(sink, 1600)
        events = received(sink)
        self.assertEqual(len(events), 1600)
        self.assertEqual(len(set(events)), 1600)
        for number in range(8):  # the order of one caller is kept
            own = [index for item_number, index in events if item_number == number]
            self.assertEqual(own, list(range(200)))


class BatchTest(DeliveryServiceTestCase):
    def test_a_batch_of_exactly_batch_size_leaves_without_waiting_for_the_interval(self):
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,)), batch_size=5, flush_interval_milliseconds=30_000)
        started = time.monotonic()
        for number in range(5):
            service.enqueue("log", number)
        self.wait_for_events(sink, 5)
        self.assertLess(time.monotonic() - started, 5.0)
        self.assertEqual(sink.batches, [[0, 1, 2, 3, 4]])

    def test_a_smaller_batch_leaves_after_the_interval_and_not_before(self):
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,)), batch_size=100, flush_interval_milliseconds=200)
        started = time.monotonic()
        for number in range(3):
            service.enqueue("log", number)
        time.sleep(0.05)
        self.assertEqual(sink.calls, 0)  # the interval has not ended
        self.wait_for_events(sink, 3)
        self.assertGreaterEqual(time.monotonic() - started, 0.15)  # generous: the interval is 200 ms
        self.assertEqual(sink.batches, [[0, 1, 2]])

    def test_the_interval_counts_from_the_oldest_event_and_a_new_one_does_not_extend_it(self):
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,)), batch_size=100, flush_interval_milliseconds=150)
        service.enqueue("log", "old")
        for number in range(4):
            time.sleep(0.02)
            service.enqueue("log", number)
        self.wait_for_events(sink, 5)
        self.assertEqual(received(sink), ["old", 0, 1, 2, 3])

    def test_a_burst_is_cut_in_batches_of_at_most_batch_size_and_the_order_is_kept(self):
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,)), queue_size=2000, batch_size=7)
        for number in range(1000):
            service.enqueue("log", number)
        self.wait_for_events(sink, 1000)
        self.assertEqual(received(sink), list(range(1000)))
        self.assertTrue(all(1 <= len(batch) <= 7 for batch in sink.batches))

    def test_one_sink_call_per_batch_and_no_retry(self):
        sink = FakeLogSink()
        sink.set_result(SendResult(accepted=0, dropped=0, failed=3, reason="network_error"))
        service = self.make(Destinations(log=(sink,)), batch_size=3)
        for number in range(3):
            service.enqueue("log", number)
        self.wait_for_events(sink, 3)
        time.sleep(0.05)
        self.assertEqual(sink.calls, 1)
        self.assertEqual(service.stats()[0].failed, 3)

    def test_every_destination_of_the_kind_receives_every_event(self):
        first, second = FakeLogSink(), FakeLogSink()
        service = self.make(Destinations(log=(first, second)))
        result = service.enqueue("log", "x")
        self.assertEqual((result.accepted, result.dropped, result.reason), (2, 0, ""))
        self.wait_for_events(first, 1)
        self.wait_for_events(second, 1)


class EnqueueResultTest(DeliveryServiceTestCase):
    def test_no_destination_of_the_kind_is_dropped_with_no_destination(self):
        service = self.make(Destinations(event=(FakeEventSink(),)))
        result = service.enqueue("log", "x")
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (0, 1, 0, "no_destination"))
        self.assertEqual(REASON_NO_DESTINATION, "no_destination")
        self.assertEqual(service.enqueue("not_a_kind", "x").reason, REASON_NO_DESTINATION)

    def test_a_full_queue_drops_the_new_event_and_counts_it_with_queue_full(self):
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,)), queue_size=2, batch_size=2, start=False)
        results = [service.enqueue("log", number) for number in range(3)]
        self.assertEqual(
            [(r.accepted, r.dropped, r.reason) for r in results], [(1, 0, ""), (1, 0, ""), (0, 1, "queue_full")]
        )
        self.assertEqual(REASON_QUEUE_FULL, "queue_full")
        stats = service.stats()[0]
        self.assertEqual((stats.queued, stats.accepted, stats.dropped, stats.last_reason), (2, 2, 1, "queue_full"))
        service.start()
        self.wait_for_events(sink, 2)
        self.assertEqual(received(sink), [0, 1])  # the NEW event was the one dropped

    def test_the_result_adds_the_destinations_and_the_first_reason_wins(self):
        full, roomy = FakeLogSink(), FakeLogSink()
        # one queue per destination: a size that fills the first and not the second cannot be told apart, so
        # both are full here and the sum is two drops with one reason
        service = self.make(Destinations(log=(full, roomy)), queue_size=1, batch_size=1, start=False)
        self.assertEqual(service.enqueue("log", 1).accepted, 2)
        result = service.enqueue("log", 2)
        self.assertEqual((result.accepted, result.dropped, result.reason), (0, 2, "queue_full"))

    def test_the_arrival_uses_the_clock_in_milliseconds(self):
        class Clock(SystemClock):
            def monotonic_nanoseconds(self) -> int:
                return 7_999_999

        sink = FakeLogSink()
        service = DeliveryService(
            Destinations(log=(sink,)), DeliverySettings(batch_size=1), Clock(), self.recording_queue()
        )
        service.enqueue("log", "x")
        self.assertEqual(self.pushed, [("x", 7)])

    def recording_queue(self):
        self.pushed: list = []
        pushed = self.pushed

        class Queue:
            def push(self, envelope: Any, arrival_ms: int) -> bool:
                pushed.append((envelope, arrival_ms))
                return True

            def take(self, n: int) -> list:
                return []

            def length(self) -> int:
                return len(pushed)

            def oldest_arrival_ms(self) -> Optional[int]:
                return None

        return lambda capacity: Queue()


class FailureTest(DeliveryServiceTestCase):
    def test_a_sink_that_raises_is_contained_counted_and_its_worker_goes_on(self):
        sink = RaisingOnceSink()
        service = self.make(Destinations(log=(sink,)), batch_size=2)
        service.enqueue("log", "a")
        service.enqueue("log", "b")
        eventually(5.0, lambda: service.stats()[0].batches == 1, "the first batch counted")
        stats = service.stats()[0]
        self.assertEqual((stats.failed, stats.last_reason), (2, REASON_SINK_PANICKED))
        self.assertNotIn("hunter2secret", repr(stats))
        self.assertEqual(REASON_SINK_PANICKED, "sink_panicked")
        service.enqueue("log", "c")
        service.enqueue("log", "d")
        eventually(5.0, lambda: len(sink.batches) == 2, "the second batch")
        self.assertEqual(sink.batches[1], ["c", "d"])

    def test_a_sink_that_returns_garbage_is_a_failure_too(self):
        class Garbage:
            def send(self, events: Sequence[Any]) -> Any:
                return "not a result"

        service = self.make(Destinations(log=(Garbage(),)), batch_size=1)
        service.enqueue("log", "x")
        eventually(5.0, lambda: service.stats()[0].batches == 1, "the batch counted")
        stats = service.stats()[0]
        self.assertEqual((stats.failed, stats.last_reason), (1, REASON_SINK_PANICKED))

    def test_a_failing_sink_never_stops_another(self):
        raising, healthy = RaisingOnceSink(), FakeLogSink()
        service = self.make(Destinations(log=(raising, healthy)), batch_size=1)
        for number in range(3):
            service.enqueue("log", number)
        self.wait_for_events(healthy, 3)
        self.assertEqual(received(healthy), [0, 1, 2])


class StatsTest(DeliveryServiceTestCase):
    def test_the_stats_list_every_destination_by_kind_and_index(self):
        service = self.make(Destinations(log=(FakeLogSink(), FakeLogSink()), event=(FakeEventSink(),)), start=False)
        stats = service.stats()
        self.assertEqual([(item.kind, item.index) for item in stats], [("log", 0), ("log", 1), ("event", 0)])
        self.assertTrue(all(isinstance(item, DestinationStats) for item in stats))
        self.assertEqual(
            [(item.queued, item.batches, item.accepted, item.dropped, item.failed, item.last_reason) for item in stats],
            [(0, 0, 0, 0, 0, "")] * 3,
        )

    def test_the_counters_follow_what_the_sink_reports(self):
        sink = FakeLogSink()
        sink.set_result(SendResult(accepted=0, dropped=2, failed=0, reason="status_413"))
        service = self.make(Destinations(log=(sink,)), batch_size=2)
        service.enqueue("log", 1)
        service.enqueue("log", 2)
        eventually(5.0, lambda: service.stats()[0].batches == 1, "the batch counted")
        stats = service.stats()[0]
        self.assertEqual(
            (stats.queued, stats.accepted, stats.dropped, stats.failed, stats.last_reason), (0, 2, 2, 0, "status_413")
        )

    def test_a_sink_that_accepts_changes_no_reason_and_adds_no_second_accepted(self):
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,)), batch_size=2)
        service.enqueue("log", 1)
        service.enqueue("log", 2)
        eventually(5.0, lambda: service.stats()[0].batches == 1, "the batch counted")
        stats = service.stats()[0]
        self.assertEqual((stats.accepted, stats.dropped, stats.failed, stats.last_reason), (2, 0, 0, ""))

    def test_a_snapshot_is_a_value_that_does_not_change_afterwards(self):
        service = self.make(Destinations(log=(FakeLogSink(),)), start=False)
        before = service.stats()[0]
        service.enqueue("log", 1)
        self.assertEqual(before.queued, 0)
        self.assertEqual(service.stats()[0].queued, 1)
        with self.assertRaises(Exception):
            before.queued = 5  # type: ignore[misc]


class LifecycleTest(DeliveryServiceTestCase):
    def test_nothing_starts_before_start(self):
        before = threading.active_count()
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,), event=(FakeEventSink(),)), start=False)
        self.assertEqual(threading.active_count(), before)
        for number in range(3):
            service.enqueue("log", number)
        time.sleep(0.05)
        self.assertEqual((sink.calls, threading.active_count(), worker_threads()), (0, before, []))
        self.assertEqual(service.stats()[0].queued, 3)
        service.start()
        self.assertEqual(threading.active_count(), before + 2)
        self.wait_for_events(sink, 3)

    def test_start_is_idempotent_and_makes_one_daemon_thread_per_destination(self):
        service = self.make(Destinations(log=(FakeLogSink(), FakeLogSink()), metrics=(FakeLogSink(),)), start=False)
        service.start()
        service.start()
        threads = worker_threads()
        self.assertEqual(
            sorted(thread.name for thread in threads),
            ["telemetry-delivery-log-0", "telemetry-delivery-log-1", "telemetry-delivery-metrics-0"],
        )
        self.assertTrue(all(thread.daemon for thread in threads))

    def test_a_service_without_destinations_starts_no_thread(self):
        before = threading.active_count()
        service = self.make(Destinations())
        self.assertEqual(threading.active_count(), before)
        service.stop()

    def test_stop_joins_the_workers_and_is_idempotent(self):
        service = self.make(Destinations(log=(FakeLogSink(),), event=(FakeEventSink(),)))
        threads = worker_threads()
        self.assertEqual(len(threads), 2)
        service.stop()
        self.assertTrue(all(not thread.is_alive() for thread in threads))
        self.assertEqual(worker_threads(), [])
        service.stop()

    def test_stop_before_start_is_fine_and_a_stopped_service_does_not_start_again(self):
        service = self.make(Destinations(log=(FakeLogSink(),)), start=False)
        service.stop()
        service.start()
        self.assertEqual(worker_threads(), [])

    def test_stop_does_not_drain_the_queue(self):
        sink = FakeLogSink()
        service = self.make(Destinations(log=(sink,)), flush_interval_milliseconds=60_000)
        for number in range(3):
            service.enqueue("log", number)
        service.stop()
        self.assertEqual(sink.calls, 0)
        self.assertEqual(service.stats()[0].queued, 3)

    def test_stop_wakes_a_worker_that_waits_for_ever_or_for_an_interval(self):
        idle, waiting = FakeLogSink(), FakeEventSink()
        service = self.make(Destinations(log=(idle,), event=(waiting,)), flush_interval_milliseconds=60_000)
        service.enqueue("event", "x")
        started = time.monotonic()
        service.stop()
        self.assertLess(time.monotonic() - started, 5.0)

    def test_stop_waits_for_the_batch_in_flight(self):
        sink = self.block()
        service = self.make(Destinations(log=(sink,)), batch_size=1)
        service.enqueue("log", "x")
        self.assertTrue(sink.entered.wait(5.0))
        stopped = threading.Event()
        stopper = threading.Thread(target=lambda: (service.stop(), stopped.set()))
        stopper.start()
        self.assertFalse(stopped.wait(0.1))  # the join waits for the send
        sink.release()
        self.assertTrue(stopped.wait(5.0))
        stopper.join()


if __name__ == "__main__":
    unittest.main()
