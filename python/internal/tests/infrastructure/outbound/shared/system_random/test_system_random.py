"""Black-box tests of SystemRandom and of the reading of the random generator."""

import threading
import unittest

from telemetry.internal.application.ports.outbound.random_port import RandomPort
from telemetry.internal.composition_root.dependencies.system_random_dependencies import new_system_random
from telemetry.internal.infrastructure.outbound.shared.system_random.external.read_random import read_random
from telemetry.internal.infrastructure.outbound.shared.system_random.system_random import SystemRandom


class SystemRandomTest(unittest.TestCase):
    def test_it_is_a_random_port(self):
        self.assertIsInstance(SystemRandom(), RandomPort)
        self.assertIsInstance(new_system_random(), RandomPort)

    def test_the_reading_of_the_generator_is_in_the_unit_interval_and_varies(self):
        values = {read_random() for _ in range(200)}
        self.assertTrue(all(0.0 <= value < 1.0 for value in values))
        self.assertGreater(len(values), 1)

    def test_the_numbers_of_the_system_source_are_in_the_unit_interval(self):
        source = new_system_random()
        self.assertTrue(all(0.0 <= source.random() <= 1.0 for _ in range(200)))

    def test_a_reading_outside_the_unit_interval_is_cut(self):
        self.assertEqual(SystemRandom(lambda: 7.5).random(), 1.0)
        self.assertEqual(SystemRandom(lambda: -2.0).random(), 0.0)
        self.assertEqual(SystemRandom(lambda: float("nan")).random(), 0.0)
        self.assertEqual(SystemRandom(lambda: 0.25).random(), 0.25)

    def test_a_generator_that_raises_gives_the_middle(self):
        def broken() -> float:
            raise RuntimeError("no generator")

        self.assertEqual(SystemRandom(broken).random(), 0.5)

    def test_it_is_safe_to_call_from_many_threads(self):
        source = SystemRandom()
        seen: list[float] = []

        def work() -> None:
            seen.extend(source.random() for _ in range(100))

        threads = [threading.Thread(target=work) for _ in range(4)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        self.assertEqual(len(seen), 400)
        self.assertTrue(all(0.0 <= value <= 1.0 for value in seen))


if __name__ == "__main__":
    unittest.main()
