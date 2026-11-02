"""Table tests of the rule that turns the fields of a log call into attributes (shared with Go)."""

import dataclasses
import datetime
import unittest
from typing import Any
from unittest import mock

from telemetry.internal.application.dtos.log_entry import LogEntry
from telemetry.internal.application.services import log_fields
from telemetry.internal.application.services.log_fields import (
    MAX_EXACT_INTEGER,
    UNSUPPORTED_VALUE_TEXT,
    attribute_value_of,
    fields_to_attribute_map,
    fields_to_attributes,
    log_entry_data,
)


@dataclasses.dataclass
class Point:
    x: int
    y: int


class AttributeValueOfTest(unittest.TestCase):
    def test_the_table_of_values(self):
        table: list[tuple[str, Any, Any]] = [
            ("none is dropped", None, None),
            ("string", "bob", "bob"),
            ("empty string", "", ""),
            ("true", True, True),
            ("false", False, False),
            ("int", 7, 7),
            ("negative int", -3, -3),
            ("int at the top", MAX_EXACT_INTEGER, MAX_EXACT_INTEGER),
            ("int at the bottom", -MAX_EXACT_INTEGER, -MAX_EXACT_INTEGER),
            ("int over the top", MAX_EXACT_INTEGER + 1, str(MAX_EXACT_INTEGER + 1)),
            ("int under the bottom", -MAX_EXACT_INTEGER - 1, str(-MAX_EXACT_INTEGER - 1)),
            ("huge int", 10**30, str(10**30)),
            ("float", 0.5, 0.5),
            ("integral float stays a float", 7.0, 7.0),
            ("nan", float("nan"), "NaN"),
            ("positive infinity", float("inf"), "+Inf"),
            ("negative infinity", float("-inf"), "-Inf"),
            ("exception", ValueError("bad input"), "bad input"),
            ("base exception", KeyboardInterrupt("stop"), "stop"),
            ("list of scalars", ["a", 1, True, 0.5], ["a", 1, True, 0.5]),
            ("tuple of scalars is a list", ("a", "b"), ["a", "b"]),
            ("empty list", [], []),
            ("list with a dict", [{"a": 1}], '[{"a":1}]'),
            ("list with a nested list", [1, [2]], "[1,[2]]"),
            ("list with None", [1, None], "[1,null]"),
            ("list with a nan", [1, float("nan")], UNSUPPORTED_VALUE_TEXT),
            ("list with a big int", [1, 2**60], UNSUPPORTED_VALUE_TEXT),
            ("dict", {"b": 1, "a": [1, 2]}, '{"a":[1,2],"b":1}'),
            ("nested dict", {"a": {"b": True}}, '{"a":{"b":true}}'),
            ("dict with a nan", {"a": float("nan")}, UNSUPPORTED_VALUE_TEXT),
            ("dataclass", Point(1, 2), '{"x":1,"y":2}'),
            ("set", {1, 2}, UNSUPPORTED_VALUE_TEXT),
            ("object", object(), UNSUPPORTED_VALUE_TEXT),
            ("bytes", b"x", UNSUPPORTED_VALUE_TEXT),
        ]
        for name, value, expected in table:
            with self.subTest(name):
                actual = attribute_value_of(value)
                self.assertEqual(actual, expected)
                self.assertIs(type(actual), type(expected))

    def test_a_datetime_becomes_the_text_of_its_canonical_form(self):
        value = datetime.datetime(2026, 10, 28, tzinfo=datetime.timezone.utc)
        self.assertEqual(attribute_value_of(value), '"2026-10-28T00:00:00.000000Z"')

    def test_a_bool_is_not_turned_into_an_int(self):
        self.assertIs(attribute_value_of(True), True)
        self.assertIs(attribute_value_of([True])[0], True)


class FieldsToAttributesTest(unittest.TestCase):
    def test_empty_key_none_value_and_non_text_key_are_dropped(self):
        fields = {"": "x", "gone": None, 3: "y", "kept": 1}
        self.assertEqual(fields_to_attribute_map(fields), {"kept": 1})

    def test_no_fields_give_no_attributes(self):
        for fields in (None, {}, {"": 1}, {"a": None}, "not a mapping"):
            with self.subTest(fields=fields):
                self.assertEqual(fields_to_attribute_map(fields), {})
                self.assertIsNone(fields_to_attributes(fields))

    def test_the_attributes_hold_what_is_kept(self):
        attributes = fields_to_attributes({"order": 7, "tags": ["a", "b"], "user": {"id": 1}})
        self.assertEqual(attributes.as_dict(), {"order": 7, "tags": ["a", "b"], "user": '{"id":1}'})

    def test_attributes_that_cannot_be_built_are_dropped_not_raised(self):
        with mock.patch.object(log_fields, "fields_to_attribute_map", return_value={"a": object()}):
            self.assertIsNone(log_fields.fields_to_attributes({"a": 1}))


class LogEntryDataTest(unittest.TestCase):
    def test_data_has_category_and_message(self):
        self.assertEqual(log_entry_data(LogEntry(category="c", message="m")), {"category": "c", "message": "m"})

    def test_data_has_the_function_only_when_it_is_given(self):
        data = log_entry_data(LogEntry(category="c", message="m", function="run"))
        self.assertEqual(data, {"category": "c", "message": "m", "function": "run"})

    def test_a_log_entry_has_empty_defaults_and_is_frozen(self):
        log_entry = LogEntry()
        self.assertEqual((log_entry.category, log_entry.message, log_entry.function), ("", "", ""))
        self.assertIsNone(log_entry.fields)
        with self.assertRaises(dataclasses.FrozenInstanceError):
            log_entry.message = "x"  # type: ignore[misc]


if __name__ == "__main__":
    unittest.main()
