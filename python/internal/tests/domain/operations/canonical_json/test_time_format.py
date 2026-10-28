"""Tests of the fixed time format YYYY-MM-DDTHH:MM:SS.ffffffZ."""

import datetime
import unittest

from telemetry.internal.domain.errors import ClassifiedError, NonUtcTimeError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.canonical_json.time_format import format_utc_time
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_EVENT_SCHEMA_INVALID

UTC = datetime.timezone.utc


class FormatUtcTimeTest(unittest.TestCase):
    def test_six_decimals_and_the_z_are_always_there(self):
        cases = [
            (datetime.datetime(2026, 10, 28, 10, 0, 0, 0, UTC), "2026-10-28T10:00:00.000000Z"),
            (datetime.datetime(2026, 10, 28, 10, 0, 1, 500000, UTC), "2026-10-28T10:00:01.500000Z"),
            (datetime.datetime(2026, 1, 2, 3, 4, 5, 123, UTC), "2026-01-02T03:04:05.000123Z"),
            (datetime.datetime(2026, 10, 28, 10, 0, 0, 999999, UTC), "2026-10-28T10:00:00.999999Z"),
            (datetime.datetime(5, 1, 2, 3, 4, 5, 0, UTC), "0005-01-02T03:04:05.000000Z"),
            (
                datetime.datetime(2026, 10, 28, 10, 0, 0, 0, datetime.timezone(datetime.timedelta(0))),
                "2026-10-28T10:00:00.000000Z",
            ),
        ]
        for instant, expected in cases:
            with self.subTest(expected=expected):
                self.assertEqual(format_utc_time(instant), expected)

    def test_a_non_utc_instant_is_rejected_not_converted(self):
        madrid = datetime.timezone(datetime.timedelta(hours=2))
        for instant in (
            datetime.datetime(2026, 10, 28, 12, 0, 0, 0, madrid),
            datetime.datetime(2026, 10, 28, 12, 0, 0, 0, datetime.timezone(datetime.timedelta(hours=-5))),
            datetime.datetime(2026, 10, 28, 12, 0, 0, 0, datetime.timezone(datetime.timedelta(minutes=30))),
        ):
            with self.subTest(instant=instant):
                with self.assertRaises(NonUtcTimeError) as raised:
                    format_utc_time(instant)
                self.assertEqual(raised.exception.code, ERROR_CODE_EVENT_SCHEMA_INVALID)

    def test_a_naive_datetime_is_rejected(self):
        with self.assertRaises(ClassifiedError):
            format_utc_time(datetime.datetime(2026, 10, 28, 12, 0, 0))

    def test_a_datetime_inside_a_value_uses_the_fixed_format(self):
        instant = datetime.datetime(2026, 10, 28, 10, 0, 0, 7, UTC)
        self.assertEqual(canonical_json({"occurred_at": instant}), b'{"occurred_at":"2026-10-28T10:00:00.000007Z"}')
        with self.assertRaises(NonUtcTimeError):
            canonical_json({"occurred_at": datetime.datetime(2026, 10, 28, 12, 0, 0)})
