"""Tests of the rules that the generated validators call (the twin of the Go rules tests)."""

import fractions
import math
import unittest
from typing import Any

from telemetry.internal.domain.operations.event_validation import rules


def reason_of(problem: rules.Problem | None) -> str:
    """Return the reason of a problem, or "valid" when there is none."""
    return "valid" if problem is None else problem.reason


class NumbersTest(unittest.TestCase):
    def test_numbers_and_integers(self):
        cases: list[tuple[str, Any, str, str]] = [
            ("an int", 3, "valid", "valid"),
            ("a negative int", -3, "valid", "valid"),
            ("a huge int", 10**400, "valid", "valid"),
            ("a float whole number", 2.0, "valid", "valid"),
            ("a float with a fraction", 1.5, "valid", "expected integer"),
            ("a float that is huge", 1e300, "valid", "valid"),
            ("zero", 0.0, "valid", "valid"),
            ("negative zero", -0.0, "valid", "valid"),
            ("NaN", math.nan, "number is not finite", "number is not finite"),
            ("positive infinity", math.inf, "number is not finite", "number is not finite"),
            ("negative infinity", -math.inf, "number is not finite", "number is not finite"),
            ("True", True, "expected number", "expected integer"),
            ("False", False, "expected number", "expected integer"),
            ("a string", "1", "expected number", "expected integer"),
            ("None", None, "expected number", "expected integer"),
            ("a list", [1], "expected number", "expected integer"),
            ("a dict", {}, "expected number", "expected integer"),
            ("a fraction", fractions.Fraction(1, 2), "expected number", "expected integer"),
            ("a complex number", 1 + 2j, "expected number", "expected integer"),
            ("bytes", b"1", "expected number", "expected integer"),
        ]
        for name, value, number, integer in cases:
            with self.subTest(name):
                self.assertEqual(reason_of(rules.check_number(value, "data.x")), number)
                self.assertEqual(reason_of(rules.check_integer(value, "data.x")), integer)


class StringsBooleansEnumsAndObjectsTest(unittest.TestCase):
    def test_checks(self):
        bounded = rules.check_string(2, 3)
        unbounded = rules.check_string(0, 0)
        only_min = rules.check_string(2, 0)
        enum = rules.check_enum(("a", "b"))
        empty_enum = rules.check_enum(())
        cases: list[tuple[str, Any, Any, str]] = [
            ("a string inside the limits", bounded, "ab", "valid"),
            ("a string at the maximum", bounded, "abc", "valid"),
            ("a string too short", bounded, "a", "string is shorter than 2 characters"),
            ("a string too long", bounded, "abcd", "string is longer than 3 characters"),
            ("characters are not bytes (too long)", bounded, "éééé", "string is longer than 3 characters"),
            ("characters are not bytes (inside)", bounded, "éé", "valid"),
            ("an emoji counts as one character", bounded, "\U0001f600\U0001f600", "valid"),
            ("no limits", unbounded, "x" * 100000, "valid"),
            ("an empty string without limits", unbounded, "", "valid"),
            ("only a minimum", only_min, "x" * 1000, "valid"),
            ("a number is not a string", bounded, 12, "expected string"),
            ("None is not a string", unbounded, None, "expected string"),
            ("bytes are not a string", unbounded, b"a", "expected string"),
            ("a boolean", rules.check_boolean, True, "valid"),
            ("false is a boolean", rules.check_boolean, False, "valid"),
            ("1 is not a boolean", rules.check_boolean, 1, "expected boolean"),
            ("a string is not a boolean", rules.check_boolean, "true", "expected boolean"),
            ("an enum value", enum, "b", "valid"),
            ("a value that is not in the enum", enum, "c", "not one of the allowed values"),
            ("the case of an enum value matters", enum, "A", "not one of the allowed values"),
            ("an empty enum allows nothing", empty_enum, "a", "not one of the allowed values"),
            ("a number is not an enum value", enum, 1, "expected string"),
            ("an object", rules.check_object, {"a": 1}, "valid"),
            ("an empty object", rules.check_object, {}, "valid"),
            ("a list is not an object", rules.check_object, [], "expected object"),
            ("None is not an object", rules.check_object, None, "expected object"),
            ("a dict with other keys is not an object", rules.check_object, {1: 2}, "expected object"),
            ("a dict with a mix of keys is not an object", rules.check_object, {"a": 1, 2: 2}, "expected object"),
        ]
        for name, check, value, reason in cases:
            with self.subTest(name):
                self.assertEqual(reason_of(check(value, "data.x")), reason)


class AttributesTest(unittest.TestCase):
    def test_attributes_and_their_limits(self):
        limited = rules.check_attributes(rules.AttributesLimits(2, 1, 3, 4))
        unlimited = rules.check_attributes(rules.AttributesLimits())
        cases: list[tuple[str, Any, Any, str, str]] = [
            ("scalars and lists", limited, {"a": "abcd", "b": [1, True, 2.5]}, "", "valid"),
            ("an empty list", limited, {"a": []}, "", "valid"),
            ("too many keys", limited, {"a": 1, "b": 2, "c": 3}, "data.x", "too many keys"),
            ("a key that is too long", limited, {"abcd": 1}, "data.x.abcd", "the key has an invalid length"),
            ("an empty key", limited, {"": 1}, "data.x.", "the key has an invalid length"),
            ("a string that is too long", limited, {"a": "abcde"}, "data.x.a", "string is longer than 4 characters"),
            (
                "a string in a list that is too long",
                limited,
                {"a": ["ab", "abcde"]},
                "data.x.a[1]",
                "string is longer than 4 characters",
            ),
            ("NaN", limited, {"a": math.nan}, "data.x.a", "number is not finite"),
            ("an infinity in a list", limited, {"a": [math.inf]}, "data.x.a[0]", "number is not finite"),
            ("None", limited, {"a": None}, "data.x.a", "expected a scalar or a list of scalars"),
            ("a nested object", limited, {"a": {}}, "data.x.a", "expected a scalar or a list of scalars"),
            ("a nested list", limited, {"a": [[1]]}, "data.x.a[0]", "expected a string, a number or a boolean"),
            ("a tuple is not a list", limited, {"a": ("x",)}, "data.x.a", "expected a scalar or a list of scalars"),
            (
                "the first key in order is reported",
                limited,
                {"b": None, "a": None},
                "data.x.a",
                "expected a scalar or a list of scalars",
            ),
            ("not an object", limited, [], "data.x", "expected object"),
            ("None is not an object", limited, None, "data.x", "expected object"),
            ("keys that are not strings", limited, {1: 1}, "data.x", "expected object"),
            ("no limits at all", unlimited, {"": "x" * 100000, "k": 1, "l": 2, "m": 3}, "", "valid"),
        ]
        for name, check, value, path, reason in cases:
            with self.subTest(name):
                problem = check(value, "data.x")
                self.assertEqual(reason_of(problem), reason)
                if problem is not None:
                    self.assertEqual(problem.path, path)


class ObjectTest(unittest.TestCase):
    def test_an_object_is_checked_in_the_documented_order(self):
        fields = tuple(rules.FieldRule(name, rules.check_string(0, 0)) for name in ("a", "b", "c"))
        closed = rules.ObjectRules(required=("a", "b"), fields=fields, closed=True)
        open_rules = rules.ObjectRules(fields=fields)
        cases: list[tuple[str, rules.ObjectRules, Any, str, str]] = [
            ("a valid object", closed, {"a": "1", "b": "2"}, "", "valid"),
            ("a required field is missing", closed, {"a": "1", "z": 1}, "data.b", "required field is missing"),
            ("a missing field comes before a wrong type", closed, {"a": 1}, "data.b", "required field is missing"),
            ("an optional field of the wrong type", closed, {"a": "1", "b": "2", "c": 3}, "data.c", "expected string"),
            (
                "a wrong type comes before an unknown field",
                closed,
                {"a": 1, "b": "2", "z": 1},
                "data.a",
                "expected string",
            ),
            ("an unknown field", closed, {"a": "1", "b": "2", "z": 1}, "data.z", "unknown field"),
            (
                "the smallest unknown field is reported",
                closed,
                {"a": "1", "b": "2", "z": 1, "m": 1},
                "data.m",
                "unknown field",
            ),
            ("an open object allows an unknown field", open_rules, {"z": 1}, "", "valid"),
            ("an empty object with no rules", rules.ObjectRules(), {"z": 1}, "", "valid"),
            ("not an object", closed, "text", "data", "expected object"),
            ("keys that are not strings", closed, {1: "a"}, "data", "expected object"),
        ]
        for name, object_rules, value, path, reason in cases:
            with self.subTest(name):
                problem = rules.validate_object(value, "data", object_rules)
                self.assertEqual(reason_of(problem), reason)
                if problem is not None:
                    self.assertEqual(problem.path, path)


class DisplayTextTest(unittest.TestCase):
    def test_a_name_is_cut_and_cleaned_but_never_a_value(self):
        cases = [
            ("a plain name", "category", "category"),
            ("an empty name", "", ""),
            ("a control character", "a\nb\x00c\x7fd\te", "a?b?c?d?e"),
            ("exactly the limit", "a" * 64, "a" * 64),
            ("one more than the limit", "a" * 65, "a" * 64 + "..."),
            ("characters, not bytes", "é" * 64, "é" * 64),
            ("many characters", "é" * 100, "é" * 64 + "..."),
        ]
        for name, text, expected in cases:
            with self.subTest(name):
                self.assertEqual(rules.display_text(text), expected)
