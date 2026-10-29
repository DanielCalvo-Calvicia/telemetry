"""Black-box tests of UtcInstant.parse and of the schema version constant."""

import datetime
import re
import unittest

from telemetry.internal.domain import errors
from telemetry.internal.domain.rules import SCHEMA_VERSION
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant


class UtcInstantParseTest(unittest.TestCase):
    def test_parse_reads_the_fixed_format_and_round_trips(self):
        instant = UtcInstant.parse("2026-10-28T10:00:00.123456Z")
        self.assertEqual(instant.value, datetime.datetime(2026, 10, 28, 10, 0, 0, 123456, tzinfo=datetime.timezone.utc))
        self.assertEqual(str(instant), "2026-10-28T10:00:00.123456Z")

    def test_parse_rejects_every_other_shape_without_echoing_it(self):
        bad_texts = [
            "2026-10-28T10:00:00Z",
            "2026-10-28T10:00:00.123Z",
            "2026-10-28T10:00:00.123456+00:00",
            "2026-10-28 10:00:00.123456Z",
            "2026-10-28T10:00:00.123456z",
            " 2026-10-28T10:00:00.123456Z",
            "2026-13-28T10:00:00.123456Z",
            "2026-02-30T10:00:00.123456Z",
            "2026-10-28T25:00:00.123456Z",
            "",
            "secret-value",
        ]
        for text in bad_texts:
            with self.subTest(text=text):
                with self.assertRaises(errors.NonUtcTimeError) as raised:
                    UtcInstant.parse(text)
                self.assertIsInstance(raised.exception, errors.InstantFormatInvalidError)
                if text:
                    self.assertNotIn(text, str(raised.exception))

    def test_parse_rejects_what_is_not_text(self):
        with self.assertRaises(errors.InstantFormatInvalidError):
            UtcInstant.parse(None)  # type: ignore[arg-type]


class SchemaVersionTest(unittest.TestCase):
    def test_the_schema_version_is_major_dot_minor(self):
        self.assertRegex(SCHEMA_VERSION, r"^[0-9]+\.[0-9]+$")
        self.assertIsNotNone(re.fullmatch(r"[0-9]+\.[0-9]+", SCHEMA_VERSION))
