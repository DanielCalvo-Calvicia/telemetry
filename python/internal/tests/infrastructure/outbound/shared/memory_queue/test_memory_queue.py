"""Black-box tests of MemoryQueue: order, bound, taking more than it holds and many threads at once."""

import threading
import unittest

from telemetry.internal.application.ports.outbound.event_queue_port import EventQueuePort
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.shared.memory_queue.memory_queue import MemoryQueue


class MemoryQueueTest(unittest.TestCase):
    def test_it_is_an_event_queue_port_and_starts_empty(self):
        queue = new_memory_queue(3)
        self.assertIsInstance(queue, EventQueuePort)
        self.assertIsInstance(queue, MemoryQueue)
        self.assertEqual((queue.length(), queue.oldest_arrival_ms(), queue.take(5)), (0, None, []))

    def test_events_leave_in_the_order_they_entered(self):
        queue = new_memory_queue(10)
        for number in range(5):
            self.assertTrue(queue.push(f"event-{number}", 100 + number))  # type: ignore[arg-type]
        self.assertEqual(queue.take(2), ["event-0", "event-1"])
        self.assertEqual(queue.take(2), ["event-2", "event-3"])
        self.assertEqual(queue.take(2), ["event-4"])

    def test_a_full_queue_refuses_the_new_event_and_keeps_the_old_ones(self):
        queue = new_memory_queue(2)
        self.assertTrue(queue.push("a", 1))  # type: ignore[arg-type]
        self.assertTrue(queue.push("b", 2))  # type: ignore[arg-type]
        self.assertFalse(queue.push("c", 3))  # type: ignore[arg-type]
        self.assertEqual(queue.length(), 2)
        self.assertEqual(queue.take(10), ["a", "b"])
        self.assertTrue(queue.push("d", 4))  # type: ignore[arg-type]

    def test_taking_more_than_it_holds_gives_what_it_holds_and_taking_nothing_gives_nothing(self):
        queue = new_memory_queue(5)
        queue.push("a", 1)  # type: ignore[arg-type]
        self.assertEqual(queue.take(0), [])
        self.assertEqual(queue.take(-4), [])
        self.assertEqual(queue.length(), 1)
        self.assertEqual(queue.take(99), ["a"])
        self.assertEqual(queue.length(), 0)

    def test_the_oldest_arrival_is_the_one_of_the_first_event_and_moves_on(self):
        queue = new_memory_queue(5)
        queue.push("a", 10)  # type: ignore[arg-type]
        queue.push("b", 20)  # type: ignore[arg-type]
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
        queue.push(item, 1)  # type: ignore[arg-type]
        self.assertIs(queue.take(1)[0], item)

    def test_many_threads_push_and_take_at_once_and_nothing_is_lost_or_repeated(self):
        queue = new_memory_queue(10_000)
        producers, per_producer = 8, 500
        taken: list = []
        taken_lock = threading.Lock()
        done = threading.Event()

        def produce(number: int) -> None:
            for index in range(per_producer):
                self.assertTrue(queue.push((number, index), index))  # type: ignore[arg-type]

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
            queue.push(index, index)  # type: ignore[arg-type]
        stop.set()
        consumer.join(timeout=10)
        self.assertEqual(taken, list(range(2000)))


if __name__ == "__main__":
    unittest.main()
