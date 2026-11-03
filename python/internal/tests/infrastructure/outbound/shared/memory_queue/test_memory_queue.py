"""Black-box tests of MemoryQueue: order, bound, the policies of a full queue and many threads at once."""

import threading
import unittest

from telemetry.internal.application.dtos.offer_result import OfferResult
from telemetry.internal.application.ports.outbound.event_queue_port import EventQueuePort
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.domain.value_objects.enums_generated import EventClass, QueuePolicy
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.shared.memory_queue.memory_queue import MemoryQueue

NORMAL = EventClass.NORMAL
BULK = EventClass.BULK
CRITICAL = EventClass.CRITICAL
DROP_NEWEST = QueuePolicy.DROP_NEWEST


def push(queue: EventQueuePort, item: object, arrival_ms: int = 1, event_class: EventClass = NORMAL) -> OfferResult:
    """Offer an item (any object stands for an envelope) with the default policy drop_newest."""
    return queue.offer(item, event_class, arrival_ms, DROP_NEWEST)  # type: ignore[arg-type]


def fill(capacity: int, classes: list[EventClass]) -> EventQueuePort:
    """A queue of the capacity holding the events 1, 2, 3... with the classes, oldest first (arrival 10, 20...)."""
    queue = new_memory_queue(capacity)
    for number, event_class in enumerate(classes, start=1):
        assert push(queue, number, number * 10, event_class).admitted
    return queue


class MemoryQueueTest(unittest.TestCase):
    def test_it_is_an_event_queue_port_and_starts_empty(self):
        queue = new_memory_queue(3)
        self.assertIsInstance(queue, EventQueuePort)
        self.assertIsInstance(queue, MemoryQueue)
        self.assertEqual((queue.length(), queue.oldest_arrival_ms(), queue.take(5)), (0, None, []))

    def test_events_leave_in_the_order_they_entered(self):
        queue = new_memory_queue(10)
        for number in range(5):
            self.assertTrue(push(queue, f"event-{number}", 100 + number).admitted)
        self.assertEqual(queue.take(2), ["event-0", "event-1"])
        self.assertEqual(queue.take(2), ["event-2", "event-3"])
        self.assertEqual(queue.take(2), ["event-4"])

    def test_a_full_queue_with_drop_newest_refuses_the_new_event_and_keeps_the_old_ones(self):
        queue = new_memory_queue(2)
        self.assertTrue(push(queue, "a", 1).admitted)
        self.assertTrue(push(queue, "b", 2).admitted)
        refused = push(queue, "c", 3)
        self.assertEqual(
            (refused.admitted, refused.displaced, refused.reason, refused.must_wait),
            (False, None, "queue_full_newest", False),
        )
        self.assertEqual(queue.length(), 2)
        self.assertEqual(queue.take(10), ["a", "b"])
        self.assertTrue(push(queue, "d", 4).admitted)

    def test_an_offer_with_room_is_admitted_with_no_reason_whatever_the_policy(self):
        for policy in QueuePolicy:
            with self.subTest(policy=policy):
                queue = new_memory_queue(2)
                result = queue.offer("a", BULK, 1, policy)  # type: ignore[arg-type]
                self.assertEqual(result, OfferResult(admitted=True, displaced=None, reason="", must_wait=False))

    def test_taking_more_than_it_holds_gives_what_it_holds_and_taking_nothing_gives_nothing(self):
        queue = new_memory_queue(5)
        push(queue, "a", 1)
        self.assertEqual(queue.take(0), [])
        self.assertEqual(queue.take(-4), [])
        self.assertEqual(queue.length(), 1)
        self.assertEqual(queue.take(99), ["a"])
        self.assertEqual(queue.length(), 0)

    def test_the_oldest_arrival_is_the_one_of_the_first_event_and_moves_on(self):
        queue = new_memory_queue(5)
        push(queue, "a", 10)
        push(queue, "b", 20)
        self.assertEqual(queue.oldest_arrival_ms(), 10)
        queue.take(1)
        self.assertEqual(queue.oldest_arrival_ms(), 20)
        queue.take(1)
        self.assertIsNone(queue.oldest_arrival_ms())

    def test_a_capacity_that_is_not_an_integer_above_zero_is_refused_with_the_field(self):
        for bad in (0, -1, True, 1.5, "3", None):
            with self.subTest(bad=bad):
                with self.assertRaises(ConfigurationError) as raised:
                    new_memory_queue(bad)  # type: ignore[arg-type]
                self.assertEqual([problem.field for problem in raised.exception.problems], ["queue_size"])

    def test_the_event_is_held_by_reference_and_never_copied(self):
        queue = new_memory_queue(1)
        item = object()
        push(queue, item, 1)
        self.assertIs(queue.take(1)[0], item)

    def test_many_threads_push_and_take_at_once_and_nothing_is_lost_or_repeated(self):
        queue = new_memory_queue(10_000)
        producers, per_producer = 8, 500
        taken: list = []
        taken_lock = threading.Lock()
        done = threading.Event()

        def produce(number: int) -> None:
            for index in range(per_producer):
                self.assertTrue(push(queue, (number, index), index).admitted)

        def consume() -> None:
            while True:
                chunk = queue.take(7)
                if chunk:
                    with taken_lock:
                        taken.extend(chunk)
                elif done.is_set() and queue.length() == 0:
                    return

        consumers = [threading.Thread(target=consume) for _ in range(3)]
        threads = [threading.Thread(target=produce, args=(number,)) for number in range(producers)]
        for thread in consumers + threads:
            thread.start()
        for thread in threads:
            thread.join()
        done.set()
        for thread in consumers:
            thread.join(timeout=10)
        self.assertEqual(len(taken), producers * per_producer)
        self.assertEqual(len(set(taken)), producers * per_producer)
        for number in range(producers):
            own = sorted(index for item_number, index in taken if item_number == number)
            self.assertEqual(own, list(range(per_producer)))

    def test_the_order_of_one_producer_is_kept_with_one_consumer(self):
        queue = new_memory_queue(10_000)
        stop = threading.Event()
        taken: list = []

        def consume() -> None:
            while not stop.is_set() or queue.length():
                taken.extend(queue.take(13))

        consumer = threading.Thread(target=consume)
        consumer.start()
        for index in range(2000):
            push(queue, index, index)
        stop.set()
        consumer.join(timeout=10)
        self.assertEqual(taken, list(range(2000)))


class FullQueueOfferTest(unittest.TestCase):
    def test_critical_displaces_the_oldest_bulk_and_the_order_of_the_rest_is_kept(self):
        queue = fill(4, [NORMAL, BULK, NORMAL, BULK])
        result = queue.offer(100, CRITICAL, 500, DROP_NEWEST)  # type: ignore[arg-type]
        self.assertEqual(
            (result.admitted, result.displaced, result.reason, result.must_wait),
            (True, 2, "displaced_by_critical", False),
        )
        self.assertEqual(queue.length(), 4)
        self.assertEqual(queue.oldest_arrival_ms(), 10)
        self.assertEqual(queue.take(10), [1, 3, 4, 100])

    def test_drop_oldest_removes_the_oldest_event_that_is_not_of_a_higher_class(self):
        queue = fill(3, [CRITICAL, NORMAL, NORMAL])
        result = queue.offer(100, NORMAL, 500, QueuePolicy.DROP_OLDEST)  # type: ignore[arg-type]
        self.assertEqual((result.admitted, result.displaced, result.reason), (True, 2, "queue_full_oldest"))
        self.assertEqual(queue.take(10), [1, 3, 100])

    def test_drop_oldest_with_only_higher_classes_drops_the_new_event(self):
        queue = fill(2, [CRITICAL, CRITICAL])
        result = queue.offer(100, NORMAL, 500, QueuePolicy.DROP_OLDEST)  # type: ignore[arg-type]
        self.assertEqual(
            (result.admitted, result.displaced, result.reason, result.must_wait),
            (False, None, "queue_full_newest", False),
        )
        self.assertEqual(queue.take(10), [1, 2])

    def test_block_with_timeout_asks_the_caller_to_wait_and_changes_nothing(self):
        queue = fill(2, [NORMAL, NORMAL])
        result = queue.offer(100, NORMAL, 500, QueuePolicy.BLOCK_WITH_TIMEOUT)  # type: ignore[arg-type]
        self.assertEqual(
            (result.admitted, result.displaced, result.reason, result.must_wait),
            (False, None, "block_timeout", True),
        )
        self.assertEqual((queue.length(), queue.oldest_arrival_ms()), (2, 10))
        self.assertEqual(queue.take(10), [1, 2])

    def test_the_arrival_of_a_displacing_event_is_its_own_and_the_oldest_moves_on(self):
        queue = fill(2, [BULK, NORMAL])
        queue.offer(100, CRITICAL, 500, DROP_NEWEST)  # type: ignore[arg-type]
        self.assertEqual(queue.oldest_arrival_ms(), 20)
        queue.take(1)
        self.assertEqual(queue.oldest_arrival_ms(), 500)

    def test_a_removed_event_is_the_same_object_that_was_offered(self):
        queue = new_memory_queue(1)
        first = object()
        push(queue, first, 1, BULK)
        result = queue.offer(object(), CRITICAL, 2, DROP_NEWEST)  # type: ignore[arg-type]
        self.assertIs(result.displaced, first)

    def test_concurrent_offers_never_exceed_the_capacity_and_every_offer_is_counted_once(self):
        capacity, producers, per_producer = 50, 8, 400
        classes = (BULK, NORMAL, CRITICAL)
        for policy in (QueuePolicy.DROP_NEWEST, QueuePolicy.DROP_OLDEST):
            with self.subTest(policy=policy):
                queue = new_memory_queue(capacity)
                outcomes: list[OfferResult] = []
                lock = threading.Lock()
                start = threading.Event()
                over: list[int] = []

                def produce(number: int) -> None:
                    start.wait()
                    mine = []
                    for index in range(per_producer):
                        event_class = classes[(number + index) % 3]
                        mine.append(queue.offer((number, index), event_class, index, policy))  # type: ignore[arg-type]
                        if queue.length() > capacity:
                            over.append(queue.length())
                    with lock:
                        outcomes.extend(mine)

                threads = [threading.Thread(target=produce, args=(number,)) for number in range(producers)]
                for thread in threads:
                    thread.start()
                start.set()
                for thread in threads:
                    thread.join()
                offered = producers * per_producer
                admitted = [result for result in outcomes if result.admitted]
                refused = [result for result in outcomes if not result.admitted]
                displaced = [result.displaced for result in outcomes if result.displaced is not None]
                self.assertEqual(len(admitted) + len(refused), offered)
                self.assertEqual(over, [])
                self.assertEqual(queue.length(), capacity)
                self.assertEqual(len(admitted) - len(displaced), capacity)  # what entered, minus what left, stayed
                held = queue.take(capacity + 10)
                self.assertEqual(len(held), capacity)
                self.assertEqual(len(set(held) | set(displaced)), len(admitted))  # nobody is both held and removed
                self.assertTrue(all(not result.must_wait for result in outcomes))
                self.assertTrue(all(result.reason != "" for result in refused))

    def test_concurrent_offers_and_takes_with_block_with_timeout_lose_nothing(self):
        queue = new_memory_queue(10)
        producers, per_producer = 4, 300
        outcomes: list[OfferResult] = []
        taken: list = []
        lock = threading.Lock()
        done = threading.Event()

        def produce(number: int) -> None:
            mine = [
                queue.offer((number, index), NORMAL, index, QueuePolicy.BLOCK_WITH_TIMEOUT)  # type: ignore[arg-type]
                for index in range(per_producer)
            ]
            with lock:
                outcomes.extend(mine)

        def consume() -> None:
            while not done.is_set() or queue.length():
                chunk = queue.take(3)
                with lock:
                    taken.extend(chunk)

        consumer = threading.Thread(target=consume)
        threads = [threading.Thread(target=produce, args=(number,)) for number in range(producers)]
        consumer.start()
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        done.set()
        consumer.join(timeout=10)
        admitted = sum(1 for result in outcomes if result.admitted)
        waiting = sum(1 for result in outcomes if result.must_wait)
        self.assertEqual(admitted + waiting, producers * per_producer)  # nothing else can happen with this policy
        self.assertEqual(len(taken), admitted)
        self.assertEqual(len(set(taken)), admitted)


if __name__ == "__main__":
    unittest.main()
