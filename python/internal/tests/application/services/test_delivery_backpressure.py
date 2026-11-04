"""Black-box tests of the rules of a full queue in DeliveryService: policies, classes, drops per reason, the wait.

The service is built over the real memory queue and fake sinks. A service that is NOT started never takes from its
queue, so a test fills it exactly and sees what the policy does; the wait of ``block_with_timeout`` uses the real
clock with short timeouts, with a generous upper bound so a slow machine does not fail it.
"""

import threading
import time
import unittest
from typing import Any, Optional

from internal.tests.application.services.test_delivery_service import Item, ItemDeliveryService, received
from internal.tests.eventually import eventually
from internal.tests.fakes import BlockingLogSink, FakeLogSink, FakeRandom
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.services.delivery_service import (
    REASON_BLOCK_TIMEOUT,
    REASON_DISPLACED_BY_CRITICAL,
    REASON_QUEUE_FULL_NEWEST,
    REASON_QUEUE_FULL_OLDEST,
    REASON_UNSPECIFIED,
    Destinations,
)
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.domain.value_objects.enums_generated import EventClass, QueuePolicy, Severity
from telemetry.internal.infrastructure.outbound.shared.system_clock.system_clock import SystemClock

LOG = "log.entry.recorded"
ALERT = "alert.raised"
INFO, DEBUG = Severity.INFO, Severity.DEBUG
BLOCK_MS = 200


def normal(value: Any) -> Item:
    """A log event at INFO: class normal by default."""
    return Item(value, LOG, INFO)


def bulk(value: Any) -> Item:
    """A log event at DEBUG: class bulk by default."""
    return Item(value, LOG, DEBUG)


def critical(value: Any) -> Item:
    """An alert: critical only when the configuration says so (ALERT is assigned in ``make``)."""
    return Item(value, ALERT, INFO)


class BackpressureTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self.blocked: list[BlockingLogSink] = []

    def make(
        self,
        sink: Any,
        policy: QueuePolicy = QueuePolicy.DROP_NEWEST,
        queue_size: int = 3,
        block_timeout_ms: int = 0,
        event_classes: Optional[dict] = None,
        start: bool = False,
        batch_size: int = 1,
    ) -> ItemDeliveryService:
        """A service over one sink; by default NOT started, so the queue only fills. It stops with the test."""
        classes = {ALERT: EventClass.CRITICAL} if event_classes is None else event_classes
        settings = DeliverySettings(
            queue_size=queue_size,
            batch_size=batch_size,
            flush_interval_milliseconds=5,
            queue_policy=policy,
            block_timeout_milliseconds=block_timeout_ms,
            event_classes=classes,
        )
        service = ItemDeliveryService(
            Destinations(log=(sink,)), settings, SystemClock(), new_memory_queue, FakeRandom()
        )
        self.addCleanup(self.release_then_stop, service)
        if start:
            service.start()
        return service

    def block(self) -> BlockingLogSink:
        sink = BlockingLogSink()
        self.blocked.append(sink)
        return sink

    def release_then_stop(self, service: ItemDeliveryService) -> None:
        for sink in self.blocked:
            sink.release()
        service.stop()

    @staticmethod
    def results(service: ItemDeliveryService, *events: Item) -> list[tuple[int, int, str]]:
        return [(r.accepted, r.dropped, r.reason) for r in (service.enqueue("log", event) for event in events)]

    def queued_values(self, service: ItemDeliveryService, sink: FakeLogSink, count: int) -> list:
        """Start the service and return the values the sink receives, in order (the queue holds ``count``)."""
        service.start()
        eventually(5.0, lambda: len(received(sink)) >= count, f"{count} events at the sink")
        eventually(5.0, lambda: service.stats()[0].queued == 0, "the queue empty")
        return [event.value for event in received(sink)]


class PolicyTest(BackpressureTestCase):
    def test_drop_newest_drops_the_new_event_and_counts_it_per_reason(self):
        sink = FakeLogSink()
        service = self.make(sink, QueuePolicy.DROP_NEWEST, queue_size=2)
        found = self.results(service, normal(0), normal(1), normal(2), bulk(3))
        self.assertEqual(found, [(1, 0, ""), (1, 0, ""), (0, 1, REASON_QUEUE_FULL_NEWEST), (0, 1, "queue_full_newest")])
        stats = service.stats()[0]
        self.assertEqual((stats.queued, stats.accepted, stats.dropped), (2, 2, 2))
        self.assertEqual(stats.dropped_by_reason, {"queue_full_newest": 2})
        self.assertEqual(stats.last_reason, "queue_full_newest")
        self.assertEqual(self.queued_values(service, sink, 2), [0, 1])

    def test_drop_oldest_removes_the_oldest_event_and_the_result_of_the_call_is_not_changed(self):
        sink = FakeLogSink()
        service = self.make(sink, QueuePolicy.DROP_OLDEST, queue_size=2)
        found = self.results(service, normal(0), normal(1), normal(2), normal(3))
        self.assertEqual(found, [(1, 0, "")] * 4)  # the displaced event is not part of the result of the call
        stats = service.stats()[0]
        self.assertEqual((stats.queued, stats.accepted, stats.dropped), (2, 4, 2))
        self.assertEqual(stats.dropped_by_reason, {REASON_QUEUE_FULL_OLDEST: 2})
        self.assertEqual(stats.last_reason, "queue_full_oldest")
        self.assertEqual(self.queued_values(service, sink, 2), [2, 3])

    def test_drop_oldest_keeps_higher_classes_and_drops_the_new_event_when_nothing_is_lower(self):
        sink = FakeLogSink()
        service = self.make(sink, QueuePolicy.DROP_OLDEST, queue_size=2)
        self.results(service, critical(0), critical(1))
        found = self.results(service, normal(2), bulk(3))
        self.assertEqual(found, [(0, 1, "queue_full_newest")] * 2)
        self.assertEqual(service.stats()[0].dropped_by_reason, {"queue_full_newest": 2})
        self.assertEqual(self.queued_values(service, sink, 2), [0, 1])

    def test_drop_oldest_does_not_remove_a_critical_event_for_a_normal_one_while_a_lower_one_is_queued(self):
        sink = FakeLogSink()
        service = self.make(sink, QueuePolicy.DROP_OLDEST, queue_size=3)
        self.results(service, critical(0), normal(1), bulk(2))
        self.results(service, normal(3))  # the oldest event that is not higher than normal is 1
        self.assertEqual(self.queued_values(service, sink, 3), [0, 2, 3])

    def test_a_critical_event_displaces_the_oldest_bulk_and_is_counted_as_displaced_by_critical(self):
        sink = FakeLogSink()
        service = self.make(sink, QueuePolicy.DROP_NEWEST, queue_size=3)
        self.results(service, normal(0), bulk(1), bulk(2))
        found = self.results(service, critical(3))
        self.assertEqual(found, [(1, 0, "")])
        stats = service.stats()[0]
        self.assertEqual((stats.queued, stats.accepted, stats.dropped), (3, 4, 1))
        self.assertEqual(stats.dropped_by_reason, {REASON_DISPLACED_BY_CRITICAL: 1})
        self.assertEqual(stats.last_reason, "displaced_by_critical")
        self.assertEqual(self.queued_values(service, sink, 3), [0, 2, 3])

    def test_a_critical_event_displaces_a_normal_one_when_no_bulk_is_queued(self):
        sink = FakeLogSink()
        service = self.make(sink, QueuePolicy.DROP_NEWEST, queue_size=2)
        self.results(service, normal(0), critical(1))
        self.results(service, critical(2))
        self.assertEqual(service.stats()[0].dropped_by_reason, {REASON_DISPLACED_BY_CRITICAL: 1})
        self.assertEqual(self.queued_values(service, sink, 2), [1, 2])

    def test_a_critical_event_with_only_critical_queued_follows_the_policy(self):
        for policy, expected_reason, expected_queue in (
            (QueuePolicy.DROP_NEWEST, "queue_full_newest", [0, 1]),
            (QueuePolicy.DROP_OLDEST, "queue_full_oldest", [1, 2]),
        ):
            with self.subTest(policy=policy):
                sink = FakeLogSink()
                service = self.make(sink, policy, queue_size=2)
                self.results(service, critical(0), critical(1), critical(2))
                self.assertEqual(service.stats()[0].dropped_by_reason, {expected_reason: 1})
                self.assertEqual(self.queued_values(service, sink, 2), expected_queue)

    def test_the_reasons_of_every_drop_accumulate_per_destination_and_reason(self):
        sink = FakeLogSink()
        service = self.make(sink, QueuePolicy.DROP_OLDEST, queue_size=2)
        self.results(service, bulk(0), bulk(1))
        self.results(service, normal(2), normal(3))  # two queue_full_oldest
        self.results(service, critical(4))  # one displaced_by_critical
        stats = service.stats()[0]
        self.assertEqual(stats.dropped_by_reason, {REASON_QUEUE_FULL_OLDEST: 2, REASON_DISPLACED_BY_CRITICAL: 1})
        self.assertEqual(stats.dropped, sum(stats.dropped_by_reason.values()))

    def test_each_destination_counts_its_own_drops(self):
        first, second = FakeLogSink(), FakeLogSink()
        settings = DeliverySettings(queue_size=1, batch_size=1, queue_policy=QueuePolicy.DROP_NEWEST)
        service = ItemDeliveryService(
            Destinations(log=(first, second)), settings, SystemClock(), new_memory_queue, FakeRandom()
        )
        self.addCleanup(service.stop)
        service.enqueue("log", normal(0))
        result = service.enqueue("log", normal(1))
        self.assertEqual((result.accepted, result.dropped, result.reason), (0, 2, "queue_full_newest"))
        for stats in service.stats():
            self.assertEqual(stats.dropped_by_reason, {"queue_full_newest": 1})

    def test_the_by_reason_of_a_snapshot_is_a_copy(self):
        service = self.make(FakeLogSink(), queue_size=1)
        self.results(service, normal(0), normal(1))
        before = service.stats()[0]
        before.dropped_by_reason["queue_full_newest"] = 99  # type: ignore[index]
        self.results(service, normal(2))
        after = service.stats()[0]
        self.assertEqual(after.dropped_by_reason, {"queue_full_newest": 2})
        self.assertEqual(before.dropped_by_reason, {"queue_full_newest": 99})

    def test_a_snapshot_without_drops_has_an_empty_map(self):
        service = self.make(FakeLogSink())
        self.assertEqual(service.stats()[0].dropped_by_reason, {})

    def test_the_drops_a_sink_reports_are_counted_per_reason_too(self):
        sink = FakeLogSink()
        sink.set_result(SendResult(accepted=0, dropped=2, failed=0, reason="status_413"))
        service = self.make(sink, start=True, batch_size=2)
        service.enqueue("log", normal(0))
        service.enqueue("log", normal(1))
        eventually(5.0, lambda: service.stats()[0].batches == 1, "the batch counted")
        stats = service.stats()[0]
        self.assertEqual((stats.dropped, stats.dropped_by_reason), (2, {"status_413": 2}))
        sink.set_result(SendResult(accepted=0, dropped=1, failed=0))
        service.enqueue("log", normal(2))
        eventually(5.0, lambda: service.stats()[0].batches == 2, "the second batch counted")
        self.assertEqual(service.stats()[0].dropped_by_reason, {"status_413": 2, REASON_UNSPECIFIED: 1})
        self.assertEqual(REASON_UNSPECIFIED, "unspecified")


class ClassAssignmentTest(BackpressureTestCase):
    def test_changing_the_class_of_an_event_type_changes_where_it_is_dropped(self):
        for classes, expected_queue, expected_reasons in (
            ({}, [0], {REASON_QUEUE_FULL_NEWEST: 1}),  # the default: the DEBUG log is bulk and finds no room
            ({LOG: EventClass.CRITICAL}, [1], {REASON_DISPLACED_BY_CRITICAL: 1}),  # critical: the old one goes
        ):
            with self.subTest(classes=classes):
                sink = FakeLogSink()
                service = self.make(sink, QueuePolicy.DROP_NEWEST, queue_size=1, event_classes=classes)
                service.enqueue("log", Item(0, "metric.recorded", INFO))  # normal
                service.enqueue("log", bulk(1))
                self.assertEqual(service.stats()[0].dropped_by_reason, expected_reasons)
                self.assertEqual(self.queued_values(service, sink, 1), expected_queue)

    def test_an_assignment_to_bulk_makes_an_event_the_first_to_be_displaced(self):
        sink = FakeLogSink()
        classes = {"metric.recorded": EventClass.BULK, ALERT: EventClass.CRITICAL}
        service = self.make(sink, queue_size=2, event_classes=classes)
        service.enqueue("log", normal(0))
        service.enqueue("log", Item(1, "metric.recorded", INFO))
        service.enqueue("log", critical(2))
        self.assertEqual(self.queued_values(service, sink, 2), [0, 2])

    def test_an_empty_map_works_and_every_event_follows_the_defaults(self):
        sink = FakeLogSink()
        service = self.make(sink, QueuePolicy.DROP_OLDEST, queue_size=2, event_classes={})
        self.results(service, normal(0), normal(1), bulk(2), Item(3, "x.y", INFO))
        self.assertEqual(
            service.stats()[0].dropped_by_reason, {REASON_QUEUE_FULL_NEWEST: 1, REASON_QUEUE_FULL_OLDEST: 1}
        )  # the bulk one finds only higher classes
        self.assertEqual(self.queued_values(service, sink, 2), [1, 3])

    def test_the_class_is_decided_once_by_the_event_type_and_the_severity_of_the_envelope(self):
        sink = FakeLogSink()
        service = self.make(sink, queue_size=1, event_classes={ALERT: EventClass.CRITICAL})
        service.enqueue("log", Item(0, LOG, Severity.TRACE))  # bulk
        service.enqueue("log", Item(1, ALERT, Severity.DEBUG))  # critical whatever its severity
        self.assertEqual(self.queued_values(service, sink, 1), [1])


class NoDropRaisesTest(BackpressureTestCase):
    def test_no_drop_of_any_policy_and_class_raises(self):
        events = (normal("n"), bulk("b"), critical("c"))
        for policy in QueuePolicy:
            for event in events:
                with self.subTest(policy=policy, event=event.value):
                    service = self.make(FakeLogSink(), policy, queue_size=1, block_timeout_ms=1)
                    for _ in range(3):
                        result = service.enqueue("log", event)
                        self.assertIsInstance(result, EmitResult)
                        self.assertEqual(result.accepted + result.dropped, 1)

    def test_many_threads_against_a_full_queue_count_every_event_once_and_never_exceed_the_capacity(self):
        capacity, threads_count, per_thread = 20, 8, 100
        for policy in (QueuePolicy.DROP_NEWEST, QueuePolicy.DROP_OLDEST):
            with self.subTest(policy=policy):
                service = self.make(FakeLogSink(), policy, queue_size=capacity)
                outcomes: list[EmitResult] = []
                lock = threading.Lock()
                start = threading.Event()
                kinds = (normal, bulk, critical)

                def work(number: int) -> None:
                    start.wait()
                    mine = [service.enqueue("log", kinds[(number + index) % 3](index)) for index in range(per_thread)]
                    with lock:
                        outcomes.extend(mine)

                threads = [threading.Thread(target=work, args=(number,)) for number in range(threads_count)]
                for thread in threads:
                    thread.start()
                start.set()
                for thread in threads:
                    thread.join()
                offered = threads_count * per_thread
                stats = service.stats()[0]
                refused = sum(result.dropped for result in outcomes)
                self.assertEqual(sum(result.accepted + result.dropped for result in outcomes), offered)
                self.assertEqual(stats.accepted + refused, offered)
                self.assertEqual(stats.queued, capacity)
                self.assertEqual(stats.accepted - (stats.dropped - refused), capacity)  # what stayed
                self.assertEqual(stats.dropped, sum(stats.dropped_by_reason.values()))


class BlockWithTimeoutTest(BackpressureTestCase):
    def test_a_full_queue_blocks_the_caller_for_about_the_timeout_and_then_drops_with_block_timeout(self):
        service = self.make(FakeLogSink(), QueuePolicy.BLOCK_WITH_TIMEOUT, queue_size=1, block_timeout_ms=BLOCK_MS)
        self.assertEqual(self.results(service, normal(0)), [(1, 0, "")])
        started = time.monotonic()
        result = service.enqueue("log", normal(1))
        elapsed = time.monotonic() - started
        self.assertEqual((result.accepted, result.dropped, result.reason), (0, 1, REASON_BLOCK_TIMEOUT))
        self.assertGreaterEqual(elapsed, BLOCK_MS / 1000 * 0.8)
        self.assertLess(elapsed, 5.0)
        stats = service.stats()[0]
        self.assertEqual((stats.queued, stats.accepted, stats.dropped), (1, 1, 1))
        self.assertEqual(stats.dropped_by_reason, {"block_timeout": 1})
        self.assertEqual(stats.last_reason, "block_timeout")

    def test_with_room_the_call_does_not_wait_at_all(self):
        service = self.make(FakeLogSink(), QueuePolicy.BLOCK_WITH_TIMEOUT, queue_size=3, block_timeout_ms=5000)
        started = time.monotonic()
        self.results(service, normal(0), normal(1), normal(2))
        self.assertLess(time.monotonic() - started, 1.0)

    def test_a_critical_event_displaces_a_lower_one_at_once_instead_of_waiting(self):
        service = self.make(FakeLogSink(), QueuePolicy.BLOCK_WITH_TIMEOUT, queue_size=1, block_timeout_ms=5000)
        service.enqueue("log", bulk(0))
        started = time.monotonic()
        result = service.enqueue("log", critical(1))
        self.assertLess(time.monotonic() - started, 1.0)
        self.assertEqual((result.accepted, result.dropped), (1, 0))
        self.assertEqual(service.stats()[0].dropped_by_reason, {REASON_DISPLACED_BY_CRITICAL: 1})

    def test_the_call_returns_at_once_when_the_worker_frees_room(self):
        sink = self.block()
        service = self.make(
            sink, QueuePolicy.BLOCK_WITH_TIMEOUT, queue_size=1, block_timeout_ms=20_000, start=True, batch_size=1
        )
        service.enqueue("log", normal(0))
        self.assertTrue(sink.entered.wait(5.0), "the worker never called the sink")
        service.enqueue("log", normal(1))  # the queue (size 1) is full now: the worker is stuck in the sink
        outcome: list[Any] = []
        done = threading.Event()

        def call() -> None:
            outcome.append(service.enqueue("log", normal(2)))
            done.set()

        caller = threading.Thread(target=call)
        caller.start()
        self.assertFalse(done.wait(0.15), "the call did not wait for room")
        released = time.monotonic()
        sink.release()  # the worker returns, takes event 1 and notifies the room
        self.assertTrue(done.wait(5.0), "the call did not return when room appeared")
        self.assertLess(time.monotonic() - released, 5.0)  # far from the 20 s of the timeout
        caller.join()
        self.assertEqual((outcome[0].accepted, outcome[0].dropped, outcome[0].reason), (1, 0, ""))
        eventually(5.0, lambda: len(received(sink)) == 3, "the three events at the sink")
        self.assertEqual([event.value for event in received(sink)], [0, 1, 2])
        self.assertEqual(service.stats()[0].dropped, 0)

    def test_stop_wakes_a_waiting_caller_and_its_event_is_dropped_with_block_timeout(self):
        service = self.make(FakeLogSink(), QueuePolicy.BLOCK_WITH_TIMEOUT, queue_size=1, block_timeout_ms=30_000)
        service.enqueue("log", normal(0))
        outcome: list[Any] = []
        done = threading.Event()

        def call() -> None:
            outcome.append(service.enqueue("log", normal(1)))
            done.set()

        caller = threading.Thread(target=call)
        caller.start()
        self.assertFalse(done.wait(0.15))
        started = time.monotonic()
        service.stop()
        self.assertTrue(done.wait(5.0), "stop did not wake the waiting call")
        caller.join()
        self.assertLess(time.monotonic() - started, 5.0)
        self.assertEqual((outcome[0].accepted, outcome[0].dropped, outcome[0].reason), (0, 1, REASON_BLOCK_TIMEOUT))
        self.assertEqual(service.stats()[0].dropped_by_reason, {"block_timeout": 1})

    def test_after_stop_a_full_queue_drops_at_once_without_waiting(self):
        service = self.make(FakeLogSink(), QueuePolicy.BLOCK_WITH_TIMEOUT, queue_size=1, block_timeout_ms=30_000)
        service.enqueue("log", normal(0))
        service.stop()
        started = time.monotonic()
        result = service.enqueue("log", normal(1))
        self.assertLess(time.monotonic() - started, 5.0)
        self.assertEqual((result.accepted, result.dropped, result.reason), (0, 1, REASON_BLOCK_TIMEOUT))

    def test_a_timeout_of_zero_drops_at_once(self):
        service = self.make(FakeLogSink(), QueuePolicy.BLOCK_WITH_TIMEOUT, queue_size=1, block_timeout_ms=0)
        service.enqueue("log", normal(0))
        started = time.monotonic()
        result = service.enqueue("log", normal(1))
        self.assertLess(time.monotonic() - started, 1.0)
        self.assertEqual(result.reason, REASON_BLOCK_TIMEOUT)

    def test_many_callers_waiting_on_a_fast_worker_lose_nothing(self):
        sink = FakeLogSink()
        service = self.make(
            sink, QueuePolicy.BLOCK_WITH_TIMEOUT, queue_size=5, block_timeout_ms=20_000, start=True, batch_size=2
        )
        callers, per_caller = 4, 150
        outcomes: list[EmitResult] = []
        lock = threading.Lock()

        def work(number: int) -> None:
            mine = [service.enqueue("log", normal((number, index))) for index in range(per_caller)]
            with lock:
                outcomes.extend(mine)

        threads = [threading.Thread(target=work, args=(number,)) for number in range(callers)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(timeout=30)
        self.assertEqual(sum(result.accepted for result in outcomes), callers * per_caller)
        self.assertEqual(sum(result.dropped for result in outcomes), 0)
        eventually(10.0, lambda: len(received(sink)) == callers * per_caller, "every event at the sink")
        values = [event.value for event in received(sink)]
        self.assertEqual(len(set(values)), callers * per_caller)
        for number in range(callers):  # the order of one caller is kept
            own = [index for item_number, index in values if item_number == number]
            self.assertEqual(own, list(range(per_caller)))
        self.assertEqual(service.stats()[0].dropped_by_reason, {})


if __name__ == "__main__":
    unittest.main()
