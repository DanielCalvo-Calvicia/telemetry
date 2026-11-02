"""Black-box tests of SystemClock."""

import datetime
import unittest

from telemetry.internal.application.dtos.clock_info import ClockInfo
from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant
from telemetry.internal.infrastructure.outbound.shared.system_clock.system_clock import SystemClock


class SystemClockTest(unittest.TestCase):
    def test_it_is_a_clock_port(self):
        self.assertIsInstance(SystemClock(), ClockPort)

    def test_now_is_an_instant_in_utc_close_to_the_wall_time(self):
        before = datetime.datetime.now(datetime.timezone.utc)
        now = SystemClock().now()
        after = datetime.datetime.now(datetime.timezone.utc)
        self.assertIsInstance(now, UtcInstant)
        self.assertEqual(now.value.utcoffset(), datetime.timedelta(0))
        self.assertLessEqual(before, now.value)
        self.assertLessEqual(now.value, after)

    def test_now_has_microsecond_precision_in_the_contract_format(self):
        text = str(SystemClock().now())
        self.assertRegex(text, r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$")

    def test_the_monotonic_reading_never_goes_back(self):
        clock = SystemClock()
        readings = [clock.monotonic_nanoseconds() for _ in range(2000)]
        self.assertEqual(readings, sorted(readings))
        self.assertGreaterEqual(readings[0], 0)

    def test_the_monotonic_reading_counts_from_the_creation_of_the_clock(self):
        machine = iter([1_000, 1_500, 4_000])
        clock = SystemClock(read_monotonic=lambda: next(machine))
        self.assertEqual(clock.monotonic_nanoseconds(), 500)
        self.assertEqual(clock.monotonic_nanoseconds(), 3_000)

    def test_the_injected_wall_time_is_used(self):
        wall_time = datetime.datetime(2026, 10, 28, 10, 0, 0, 123456, tzinfo=datetime.timezone.utc)
        clock = SystemClock(read_time=lambda: wall_time)
        self.assertEqual(str(clock.now()), "2026-10-28T10:00:00.123456Z")

    def test_a_wall_time_that_is_not_utc_is_refused_never_converted(self):
        from telemetry.internal.domain.errors import NonUtcTimeError

        naive = datetime.datetime(2026, 10, 28, 10, 0, 0)
        with self.assertRaises(NonUtcTimeError):
            SystemClock(read_time=lambda: naive).now()

    def test_the_clock_info_is_empty(self):
        info = SystemClock().clock_info()
        self.assertEqual(info, ClockInfo())
        self.assertIsNone(info.source)
        self.assertIsNone(info.synced)


if __name__ == "__main__":
    unittest.main()
