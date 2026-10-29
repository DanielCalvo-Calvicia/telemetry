"""Black-box tests of Identifier and HexDigits over the shared tables, plus what the tables do not cover."""

import dataclasses
import unittest

from internal.tests.domain.value_objects.helpers import load_cases
from telemetry.internal.domain.base_value_objects.errors import (
    HexDigitsInvalidError,
    IdentifierBlankError,
    IdentifierHasControlCharactersError,
    ValueObjectError,
)
from telemetry.internal.domain.base_value_objects.hex_digits import HexDigits
from telemetry.internal.domain.base_value_objects.identifier import Identifier


class SharedTablesTest(unittest.TestCase):
    def test_identifier_accepts_exactly_the_valid_cases(self):
        for case in load_cases("identifier.json"):
            with self.subTest(case["description"]):
                if case["valid"]:
                    self.assertEqual(Identifier(case["input"]).value, case["input"])
                else:
                    with self.assertRaises(ValueObjectError):
                        Identifier(case["input"])

    def test_hex_digits_accepts_exactly_the_valid_cases(self):
        for case in load_cases("hex_digits.json"):
            with self.subTest(case["description"]):
                value, width = case["input"]["value"], case["input"]["width"]
                if case["valid"]:
                    self.assertEqual(str(HexDigits(value, width)), value)
                else:
                    with self.assertRaises(HexDigitsInvalidError):
                        HexDigits(value, width)


class IdentifierTest(unittest.TestCase):
    def test_the_text_is_never_trimmed(self):
        self.assertEqual(str(Identifier("  padded  ")), "  padded  ")

    def test_the_error_classes_tell_the_rule(self):
        for bad_value in ("", "   ", None, 5):
            with self.subTest(bad_value=bad_value):
                with self.assertRaises(IdentifierBlankError):
                    Identifier(bad_value)
        with self.assertRaises(IdentifierHasControlCharactersError):
            Identifier("a\x00b")

    def test_it_is_immutable_and_comparable(self):
        identifier = Identifier("hub")
        with self.assertRaises(dataclasses.FrozenInstanceError):
            identifier.value = "other"
        self.assertEqual(identifier, Identifier("hub"))
        self.assertEqual(hash(identifier), hash(Identifier("hub")))


class HexDigitsTest(unittest.TestCase):
    def test_a_bad_width_is_rejected(self):
        for bad_width in (0, -1, True, "16", None):
            with self.subTest(bad_width=bad_width):
                with self.assertRaises(HexDigitsInvalidError):
                    HexDigits("ab", bad_width)

    def test_a_value_that_is_not_text_is_rejected(self):
        with self.assertRaises(HexDigitsInvalidError):
            HexDigits(None, 2)

    def test_it_is_immutable(self):
        digits = HexDigits("ab", 2)
        with self.assertRaises(dataclasses.FrozenInstanceError):
            digits.width = 3
