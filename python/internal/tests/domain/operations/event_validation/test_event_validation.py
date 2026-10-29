"""Tests of validate_event over the tables shared with Go, and of the guarantee that it never raises."""

import math
import unittest
from typing import Any

from internal.tests.domain.operations.event_validation.helpers import load_cases
from telemetry.internal.domain.errors import ClassifiedError, EventSchemaInvalidError, EventTypeUnknownError
from telemetry.internal.domain.operations.event_validation.event_validation import run_safely, validate_event
from telemetry.internal.domain.value_objects.error_codes_generated import (
    ERROR_CODE_EVENT_SCHEMA_INVALID,
    ERROR_CODE_EVENT_TYPE_UNKNOWN,
)
from telemetry.internal.domain.value_objects.event_types_generated import EVENT_TYPE_LOG_ENTRY_RECORDED, EVENT_TYPES

SECRET = "FAKE-SECRET-VALUE-1234"


class SharedValidFixturesTest(unittest.TestCase):
    def test_every_valid_case_is_accepted(self):
        for case in load_cases("valid_cases.json"):
            with self.subTest(case["name"]):
                self.assertIsNone(validate_event(case["event_type"], case["data"]))


class SharedInvalidFixturesTest(unittest.TestCase):
    def test_every_invalid_case_gives_the_expected_code_and_field_path(self):
        for case in load_cases("invalid_cases.json"):
            with self.subTest(case["name"]):
                error = validate_event(case["event_type"], case["data"])
                self.assertIsInstance(error, ClassifiedError)
                self.assertEqual(error.code, case["error_code"])
                # The same expected detail as the Go test: the event type, the field path and the reason.
                self.assertEqual(str(error), case["message"])
                if case["field_path"] is not None:
                    self.assertIn(f"{case['event_type']}: {case['field_path']}: ", str(error))

    def test_no_message_contains_the_offending_value(self):
        checked = 0
        for case in load_cases("invalid_cases.json"):
            secret = case.get("secret")
            if secret is not None:
                checked += 1
                with self.subTest(case["name"]):
                    self.assertNotIn(secret, str(validate_event(case["event_type"], case["data"])))
        self.assertGreaterEqual(checked, 5)

    def test_the_error_is_returned_not_raised_and_has_the_matching_class(self):
        schema_error = validate_event(EVENT_TYPE_LOG_ENTRY_RECORDED, {})
        self.assertIsInstance(schema_error, EventSchemaInvalidError)
        self.assertNotIsInstance(schema_error, EventTypeUnknownError)
        self.assertEqual(schema_error.code, ERROR_CODE_EVENT_SCHEMA_INVALID)
        unknown_error = validate_event("no.such.type", {})
        self.assertIsInstance(unknown_error, EventTypeUnknownError)
        self.assertEqual(unknown_error.code, ERROR_CODE_EVENT_TYPE_UNKNOWN)

    def test_every_registered_event_type_has_a_validator(self):
        for event_type in EVENT_TYPES:
            error = validate_event(event_type, None)
            self.assertEqual(getattr(error, "code", None), ERROR_CODE_EVENT_SCHEMA_INVALID)

    def test_an_event_type_that_is_not_a_string_is_unknown_and_never_shown(self):
        for event_type in (None, 5, ["log.entry.recorded"], b"log.entry.recorded", {"a": 1}, object(), float("nan")):
            with self.subTest(repr(type(event_type))):
                error = validate_event(event_type, {"category": "c", "message": "m"})  # type: ignore[arg-type]
                self.assertIsInstance(error, EventTypeUnknownError)
                self.assertEqual(str(error), "the event type is not registered: (not a string)")

    def test_a_long_or_hostile_event_type_is_cut_in_the_message(self):
        error = validate_event("a" * 500 + "\n[forged log line]", {})
        self.assertNotIn("forged", str(error))
        self.assertNotIn("\n", str(error))


class _Hostile(dict):
    """A dict whose methods raise: the safety net must catch what the design did not foresee."""

    def __iter__(self):
        raise RuntimeError(SECRET)

    def __contains__(self, key):
        raise RuntimeError(SECRET)

    def __getitem__(self, key):
        raise RuntimeError(SECRET)


class _Cyclic(dict):
    """A dict that holds itself."""


def _cyclic() -> dict[str, Any]:
    cyclic: dict[str, Any] = {"category": "http"}
    cyclic["message"] = cyclic
    cyclic["self"] = cyclic
    return cyclic


def _deep_dict(depth: int) -> Any:
    value: Any = "leaf"
    for _ in range(depth):
        value = {"next": value}
    return value


def _deep_list(depth: int) -> Any:
    value: Any = "leaf"
    for _ in range(depth):
        value = [value]
    return value


def _weird_inputs() -> dict[str, Any]:
    return {
        "None": None,
        "a string": SECRET,
        "an int": 42,
        "a float": 3.14,
        "NaN": math.nan,
        "an infinity": math.inf,
        "a bool": True,
        "an empty list": [],
        "a list of dicts": [{"category": "c", "message": "m"}],
        "a tuple": ("category", "message"),
        "a set": {"category"},
        "bytes": SECRET.encode(),
        "an object": object(),
        "a type": dict,
        "a function": lambda: None,
        "a dict with integer keys": {1: "a"},
        "a dict with None as a key": {None: "a", "category": "c", "message": "m"},
        "a dict with mixed keys": {1: "a", "category": "c", "message": "m"},
        "a dict with tuple keys": {("a", 1): 1, "category": "c", "message": "m"},
        "a cyclic dict": _cyclic(),
        "a dict with a deep message": {"category": "c", "message": _deep_dict(100000)},
        "a dict with a deep list": {"category": "c", "message": _deep_list(100000)},
        "a dict with a huge integer": {"category": "c", "message": 10**5000},
        "a dict with a negative huge integer": {"category": -(10**5000), "message": "m"},
        "a dict with NaN": {"category": "c", "message": math.nan},
        "a dict with an infinity": {"category": -math.inf, "message": "m"},
        "a dict with None values": {"category": None, "message": None},
        "a dict with an empty key": {"": None, "category": "c", "message": "m"},
        "a dict with a very long key": {"category": "c", "message": "m", "k" * 1_000_000: 1},
        "a dict with a lone surrogate": {"category": "c", "message": "m", "\ud800": 1},
        "a hostile dict": _Hostile({"category": "c", "message": "m"}),
        "a dict subclass": _Cyclic({"category": "c", "message": "m"}),
    }


class NoInputRaisesTest(unittest.TestCase):
    def test_no_input_raises_and_every_rejection_is_classified(self):
        event_types = (EVENT_TYPE_LOG_ENTRY_RECORDED, "", "no.such.type", "\ud800", "t" * 100000)
        for name, data in _weird_inputs().items():
            for event_type in event_types:
                with self.subTest(f"{name} / {event_type[:12]!r}"):
                    error = validate_event(event_type, data)
                    if error is None:
                        continue
                    self.assertIn(error.code, (ERROR_CODE_EVENT_SCHEMA_INVALID, ERROR_CODE_EVENT_TYPE_UNKNOWN))
                    self.assertLess(len(str(error)), 1000)
                    self.assertNotIn(SECRET, str(error))

    def test_the_hostile_dict_is_turned_into_a_classified_error_by_the_safety_net(self):
        error = validate_event(EVENT_TYPE_LOG_ENTRY_RECORDED, _Hostile({"category": "c", "message": "m"}))
        self.assertIsInstance(error, EventSchemaInvalidError)
        self.assertEqual(str(error), "log.entry.recorded: validation failed")


class RunSafelyTest(unittest.TestCase):
    def test_an_exception_becomes_event_schema_invalid_without_its_text(self):
        causes = (
            RuntimeError(SECRET),
            KeyError(SECRET),
            ValueError(SECRET),
            RecursionError(SECRET),
            MemoryError(SECRET),
            ZeroDivisionError(SECRET),
        )
        for cause in causes:
            with self.subTest(type(cause).__name__):

                def failing(cause=cause):
                    raise cause

                error = run_safely(EVENT_TYPE_LOG_ENTRY_RECORDED, failing)
                self.assertIsInstance(error, EventSchemaInvalidError)
                self.assertEqual(error.code, ERROR_CODE_EVENT_SCHEMA_INVALID)
                self.assertEqual(str(error), "log.entry.recorded: validation failed")

    def test_it_returns_what_the_validation_returns(self):
        self.assertIsNone(run_safely("x", lambda: None))
        given = EventTypeUnknownError("given")
        self.assertIs(run_safely("x", lambda: given), given)

    def test_an_event_type_that_is_not_a_string_is_not_shown(self):
        def failing():
            raise RuntimeError

        self.assertEqual(str(run_safely(None, failing)), "(not a string): validation failed")

    def test_a_request_to_stop_the_program_is_not_swallowed(self):
        def interrupted():
            raise KeyboardInterrupt

        with self.assertRaises(KeyboardInterrupt):
            run_safely("x", interrupted)
