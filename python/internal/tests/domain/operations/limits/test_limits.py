"""Tests of Limits, apply_limits and fits_in_batch."""

import copy
import dataclasses
import unittest

from internal.tests.domain.operations.limits.helpers import make_envelope
from telemetry.internal.domain.errors import (
    BatchLimitBelowEventLimitError,
    ClassifiedError,
    EventTooLargeError,
    LimitInvalidError,
)
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.limits.apply_limits import apply_limits
from telemetry.internal.domain.operations.limits.fits_in_batch import fits_in_batch
from telemetry.internal.domain.operations.limits.report import LimitsReport
from telemetry.internal.domain.value_objects.limits import Limits


class LimitsValueTest(unittest.TestCase):
    def test_the_defaults_are_the_contract(self):
        limits = Limits()
        self.assertEqual(
            (
                limits.max_attribute_keys,
                limits.max_attribute_key_chars,
                limits.max_attribute_value_chars,
                limits.max_event_bytes,
                limits.max_batch_bytes,
            ),
            (32, 64, 256, 65536, 1048576),
        )

    def test_an_invalid_limit_is_rejected(self):
        for name in ("max_attribute_keys", "max_attribute_key_chars", "max_attribute_value_chars", "max_event_bytes"):
            for bad in (0, -1, True, 1.5, "10", None):
                with self.subTest(name=name, bad=bad):
                    with self.assertRaises(LimitInvalidError) as raised:
                        Limits(**{name: bad})
                    self.assertEqual(raised.exception.code, "config_field_invalid")
        with self.assertRaises(LimitInvalidError):
            Limits(max_batch_bytes=0)

    def test_a_batch_below_the_event_limit_is_rejected(self):
        with self.assertRaises(BatchLimitBelowEventLimitError) as raised:
            Limits(max_event_bytes=100, max_batch_bytes=99)
        self.assertEqual(raised.exception.code, "config_field_invalid")
        Limits(max_event_bytes=100, max_batch_bytes=100)


class ApplyLimitsTest(unittest.TestCase):
    def test_nothing_is_cut_under_the_limits(self):
        envelope = make_envelope({"b": "x", "a": [1, "y"]})
        outcome = apply_limits(envelope, Limits())
        self.assertEqual(outcome.report, LimitsReport())
        self.assertIsNone(outcome.error)
        self.assertEqual(canonical_json(outcome.envelope), canonical_json(envelope))

    def test_an_envelope_without_attributes(self):
        for attributes in (None, {}):
            outcome = apply_limits(make_envelope(attributes), Limits())
            self.assertIsNone(outcome.envelope.attributes)
            self.assertEqual(outcome.report, LimitsReport())

    def test_the_input_is_never_modified(self):
        attributes = {f"k{index:02d}": "v" * 300 for index in range(40)}
        attributes["x" * 100] = ["y" * 300, 1]
        envelope = make_envelope(attributes)
        before = copy.deepcopy(envelope)
        outcome = apply_limits(envelope, Limits())
        self.assertEqual(envelope, before)
        self.assertEqual(envelope.attributes, attributes)
        self.assertIsNot(outcome.envelope, envelope)
        self.assertEqual(outcome.report.attribute_keys_dropped, 9)

    def test_the_keys_are_cut_in_canonical_order_not_given_order(self):
        outcome = apply_limits(make_envelope({"b": 1, "a": 2, "c": 3}), Limits(max_attribute_keys=2))
        self.assertEqual(outcome.envelope.attributes, {"a": 2, "b": 1})
        self.assertEqual(outcome.report.attribute_keys_dropped, 1)

    def test_emoji_are_counted_as_characters(self):
        outcome = apply_limits(make_envelope({"k": "\U0001f600" * 257}), Limits())
        self.assertEqual(outcome.envelope.attributes["k"], "\U0001f600" * 256)
        self.assertEqual(outcome.report.attribute_values_truncated, 1)

    def test_numbers_bools_and_none_are_never_cut(self):
        attributes = {"a": True, "b": 12345678901234, "c": 1.5, "d": None, "e": (True, 7, "z" * 5)}
        outcome = apply_limits(make_envelope(attributes), Limits(max_attribute_value_chars=3))
        self.assertEqual(outcome.envelope.attributes["a"], True)
        self.assertEqual(outcome.envelope.attributes["b"], 12345678901234)
        self.assertIsNone(outcome.envelope.attributes["d"])
        self.assertEqual(outcome.envelope.attributes["e"], (True, 7, "zzz"))
        self.assertEqual(outcome.report.attribute_values_truncated, 1)

    def test_a_list_stays_a_list(self):
        outcome = apply_limits(make_envelope({"t": ["aaaa", "b"]}), Limits(max_attribute_value_chars=2))
        self.assertEqual(outcome.envelope.attributes["t"], ["aa", "b"])

    def test_the_event_limit_is_inclusive(self):
        base = len(canonical_json(make_envelope(message="")))
        outcome = apply_limits(make_envelope(message="a" * 10), Limits(max_event_bytes=base + 10))
        self.assertIsNone(outcome.error)
        outcome = apply_limits(make_envelope(message="a" * 11), Limits(max_event_bytes=base + 10))
        self.assertIsInstance(outcome.error, EventTooLargeError)
        self.assertEqual(outcome.error.code, "limit_exceeded")
        self.assertIsNone(outcome.envelope)
        self.assertEqual(outcome.report.events_rejected, 1)

    def test_a_rejected_event_keeps_the_other_counters_and_hides_the_content(self):
        secret = "do-not-leak-this-message"
        outcome = apply_limits(
            make_envelope({"k": "v" * 300}, message=secret + "a" * 100),
            Limits(max_event_bytes=300, max_batch_bytes=300),
        )
        self.assertEqual(outcome.report.attribute_values_truncated, 1)
        self.assertEqual(outcome.report.events_rejected, 1)
        self.assertNotIn(secret, str(outcome.error))
        self.assertRegex(str(outcome.error), r"\d+ bytes.*\d+")

    def test_the_report_has_only_integers(self):
        for field in dataclasses.fields(LimitsReport):
            self.assertIn(field.type, (int, "int"))
        report = apply_limits(make_envelope({"k": "v" * 300}), Limits()).report
        for field in dataclasses.fields(report):
            self.assertIsInstance(getattr(report, field.name), int)

    def test_the_size_is_measured_after_the_cuts(self):
        outcome = apply_limits(make_envelope({"k": "v" * 100000}), Limits())
        self.assertIsNone(outcome.error)
        self.assertEqual(outcome.report.attribute_values_truncated, 1)

    def test_a_cut_key_that_collides_is_dropped(self):
        outcome = apply_limits(make_envelope({"aaa1": 1, "aaa2": 2}), Limits(max_attribute_key_chars=3))
        self.assertEqual(outcome.envelope.attributes, {"aaa": 1})
        self.assertEqual(outcome.report.attribute_keys_dropped, 1)
        self.assertEqual(outcome.report.attribute_keys_truncated, 1)


class FitsInBatchTest(unittest.TestCase):
    def test_the_limit_is_inclusive(self):
        limits = Limits(max_event_bytes=10, max_batch_bytes=100)
        self.assertTrue(fits_in_batch(90, 10, limits))
        self.assertFalse(fits_in_batch(90, 11, limits))
        self.assertTrue(fits_in_batch(0, 10, limits))


class ClassifiedErrorsTest(unittest.TestCase):
    def test_the_limit_errors_are_classified(self):
        for error_class in (LimitInvalidError, BatchLimitBelowEventLimitError, EventTooLargeError):
            self.assertTrue(issubclass(error_class, ClassifiedError))
