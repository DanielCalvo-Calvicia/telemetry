"""Tests of the canonical JSON (RFC 8785) over the tables shared with Go."""

import collections
import dataclasses
import json
import unittest

from internal.tests.domain.operations.canonical_json.helpers import envelope_from_dict, load_cases
from telemetry.internal.domain.errors import ClassifiedError, UnsupportedValueError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_EVENT_SCHEMA_INVALID


class SharedStringTableTest(unittest.TestCase):
    def test_every_case_of_the_table(self):
        for case in load_cases("strings.json"):
            with self.subTest(case["name"]):
                self.assertEqual(canonical_json(case["input"]).decode("utf-8"), case["expected"])

    def test_the_output_is_utf_8(self):
        self.assertEqual(canonical_json("€\U0001f600"), b'"\xe2\x82\xac\xf0\x9f\x98\x80"')

    def test_a_lone_surrogate_is_rejected(self):
        for value in ("a\ud800b", {"a\udc00": 1}):
            with self.assertRaises(UnsupportedValueError) as raised:
                canonical_json(value)
            self.assertEqual(raised.exception.code, ERROR_CODE_EVENT_SCHEMA_INVALID)


class SharedSortingTableTest(unittest.TestCase):
    def test_every_case_of_the_table(self):
        for case in load_cases("sorting.json"):
            with self.subTest(case["name"]):
                self.assertEqual(canonical_json(case["input"]).decode("utf-8"), case["expected"])

    def test_non_bmp_keys_sort_by_utf_16_code_units(self):
        # U+1F600 is the surrogate pair D83D DE00: it sorts before U+FB33, while by code point it would sort after.
        self.assertEqual(canonical_json({"דּ": 1, "\U0001f600": 2}).decode("utf-8"), '{"\U0001f600":2,"דּ":1}')

    def test_the_order_of_the_input_never_changes_the_output(self):
        first = {"alpha": "a", "beta": 2, "gamma": {"x": 1, "y": [1, {"q": 1, "p": 2}]}}
        second = {"gamma": {"y": [1, {"p": 2, "q": 1}], "x": 1}, "beta": 2, "alpha": "a"}
        self.assertEqual(canonical_json(first), canonical_json(second))
        self.assertEqual(canonical_json(collections.OrderedDict(reversed(list(first.items())))), canonical_json(first))


class ValueTypesTest(unittest.TestCase):
    def test_scalars_lists_and_tuples(self):
        self.assertEqual(canonical_json(None), b"null")
        self.assertEqual(canonical_json([None, "a", 1, 2.5, False, (1, 2)]), b'[null,"a",1,2.5,false,[1,2]]')
        self.assertEqual(canonical_json({}), b"{}")
        self.assertEqual(canonical_json([]), b"[]")

    def test_an_enum_prints_as_its_value(self):
        self.assertEqual(canonical_json({"severity": Severity.WARN}), b'{"severity":"WARN"}')

    def test_values_without_a_json_form_are_rejected(self):
        for value in ({1, 2}, object(), b"bytes", {1: "a"}, {"a": {3}}):
            with self.subTest(value=value):
                with self.assertRaises(UnsupportedValueError) as raised:
                    canonical_json(value)
                self.assertEqual(raised.exception.code, ERROR_CODE_EVENT_SCHEMA_INVALID)

    def test_a_dataclass_class_is_not_an_instance(self):
        @dataclasses.dataclass
        class Point:
            x: int

        with self.assertRaises(ClassifiedError):
            canonical_json(Point)
        self.assertEqual(canonical_json(Point(1)), b'{"x":1}')

    def test_numbers_beyond_the_exact_range_are_rejected_inside_a_value(self):
        with self.assertRaises(ClassifiedError):
            canonical_json({"a": [2**60]})
        with self.assertRaises(ClassifiedError):
            canonical_json({"a": float("nan")})


class SharedEventTableTest(unittest.TestCase):
    def test_every_event_as_a_dict_and_as_the_generated_dataclass(self):
        for case in load_cases("events.json"):
            with self.subTest(case["name"]):
                expected = case["expected"].encode("utf-8")
                self.assertEqual(canonical_json(case["input"]), expected)
                self.assertEqual(canonical_json(envelope_from_dict(case["input"])), expected)

    def test_the_expected_text_is_json_that_round_trips(self):
        for case in load_cases("events.json"):
            with self.subTest(case["name"]):
                self.assertEqual(json.loads(case["expected"]), case["input"])

    def test_the_times_have_six_decimals_and_the_z(self):
        for case in load_cases("events.json"):
            with self.subTest(case["name"]):
                for name in ("occurred_at", "recorded_at"):
                    text = case["input"][name]
                    self.assertRegex(text, r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$")
                    self.assertIn(f'"{name}":"{text}"', case["expected"])

    def test_optional_fields_that_are_none_are_omitted(self):
        minimal = [case for case in load_cases("events.json") if case["name"] == "minimal"][0]
        text = canonical_json(envelope_from_dict(minimal["input"])).decode("utf-8")
        for name in ("actor", "attributes", "context"):
            self.assertNotIn(f'"{name}"', text)
        self.assertNotIn("environment", text)
