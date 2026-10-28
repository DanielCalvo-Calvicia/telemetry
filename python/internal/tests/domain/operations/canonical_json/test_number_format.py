"""Tests of the canonical form of numbers, over the table shared with Go."""

import math
import struct
import unittest

from internal.tests.domain.operations.canonical_json.helpers import load_cases
from telemetry.internal.domain.errors import ClassifiedError, NumberOutOfRangeError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.canonical_json.number_format import (
    MAX_EXACT_INTEGER,
    ecmascript_form,
    format_float,
    format_int,
)
from telemetry.internal.domain.value_objects.error_codes_generated import (
    ERROR_CODE_EVENT_SCHEMA_INVALID,
    ERROR_CODE_LIMIT_EXCEEDED,
)


class SharedNumberTableTest(unittest.TestCase):
    def test_every_case_of_the_table(self):
        for case in load_cases("numbers.json"):
            with self.subTest(case["name"]):
                if case["kind"] == "bits":
                    number = struct.unpack(">d", bytes.fromhex(case["input"]))[0]
                    functions = [format_float, lambda value: canonical_json(value).decode("utf-8")]
                else:
                    number = int(case["input"])
                    functions = [format_int, lambda value: canonical_json(value).decode("utf-8")]
                for function in functions:
                    if case["expected"] is None:
                        with self.assertRaises(NumberOutOfRangeError) as raised:
                            function(number)
                        self.assertEqual(raised.exception.code, case["error_code"])
                    else:
                        self.assertEqual(function(number), case["expected"])


class IntegerRangeTest(unittest.TestCase):
    def test_the_largest_exact_integer_prints(self):
        self.assertEqual(MAX_EXACT_INTEGER, 9007199254740991)
        self.assertEqual(format_int(MAX_EXACT_INTEGER), "9007199254740991")
        self.assertEqual(format_int(-MAX_EXACT_INTEGER), "-9007199254740991")

    def test_an_integer_beyond_the_range_is_limit_exceeded(self):
        for number in (MAX_EXACT_INTEGER + 1, -MAX_EXACT_INTEGER - 1, 10**30):
            with self.assertRaises(ClassifiedError) as raised:
                format_int(number)
            self.assertEqual(raised.exception.code, ERROR_CODE_LIMIT_EXCEEDED)

    def test_a_bool_is_never_an_integer(self):
        self.assertEqual(canonical_json(True), b"true")
        self.assertEqual(canonical_json(False), b"false")
        self.assertEqual(canonical_json([True, 1, False, 0]), b"[true,1,false,0]")


class FloatTest(unittest.TestCase):
    def test_nan_and_infinities_are_rejected(self):
        for number in (math.nan, math.inf, -math.inf):
            with self.assertRaises(ClassifiedError) as raised:
                format_float(number)
            self.assertEqual(raised.exception.code, ERROR_CODE_EVENT_SCHEMA_INVALID)

    def test_a_float_that_is_an_integer_prints_without_a_dot(self):
        self.assertEqual(format_float(3.0), "3")
        self.assertEqual(canonical_json(3.0), b"3")
        self.assertEqual(canonical_json(-0.0), b"0")

    def test_the_layout_of_the_digits(self):
        cases = [
            ("1", 1, "1"),
            ("1", 21, "100000000000000000000"),
            ("1", 22, "1e+21"),
            ("15", 22, "1.5e+21"),
            ("15", 1, "1.5"),
            ("1", 0, "0.1"),
            ("1", -5, "0.000001"),
            ("1", -6, "1e-7"),
            ("12", -6, "1.2e-7"),
        ]
        for digits, point_position, expected in cases:
            with self.subTest(digits=digits, point_position=point_position):
                self.assertEqual(ecmascript_form(digits, point_position), expected)
