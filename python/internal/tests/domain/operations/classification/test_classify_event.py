"""Tests of the classification of an event: the shared cases of integration-tests/fixtures/delivery and edge cases."""

import json
import pathlib
import unittest

from telemetry.internal.domain.operations.classification.classify_event import classify_event
from telemetry.internal.domain.value_objects.enums_generated import EventClass, Severity

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
CASES_PATH = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "delivery" / "classification_cases.json"


class SharedClassificationCasesTest(unittest.TestCase):
    def test_every_shared_case_gives_the_expected_class(self):
        self.assertTrue(CASES_PATH.is_file(), f"the file {CASES_PATH} does not exist")
        cases = json.loads(CASES_PATH.read_text(encoding="utf-8"))["cases"]
        self.assertGreaterEqual(len(cases), 13)
        for case in cases:
            with self.subTest(case=case["description"]):
                overrides = {event_type: EventClass(name) for event_type, name in case["overrides"].items()}
                result = classify_event(case["event_type"], case["severity"], overrides)
                self.assertEqual(result, EventClass(case["expected"]))
                self.assertIsInstance(result, EventClass)

    def test_the_overrides_may_hold_the_texts_of_the_classes_as_the_configuration_has_them(self):
        self.assertEqual(
            classify_event("log.entry.recorded", Severity.INFO, {"log.entry.recorded": "critical"}),
            EventClass.CRITICAL,
        )


class ClassifyEventTest(unittest.TestCase):
    def test_the_severity_may_be_the_enum_or_its_text(self):
        for severity in (Severity.DEBUG, "DEBUG", Severity.TRACE, "TRACE"):
            with self.subTest(severity=severity):
                self.assertEqual(classify_event("log.entry.recorded", severity, {}), EventClass.BULK)

    def test_an_event_type_that_only_contains_log_in_the_middle_is_normal(self):
        self.assertEqual(classify_event("metric.log.recorded", Severity.DEBUG, {}), EventClass.NORMAL)
        self.assertEqual(classify_event("logger.x", Severity.DEBUG, {}), EventClass.NORMAL)

    def test_an_override_needs_the_exact_event_type(self):
        overrides = {"log": EventClass.CRITICAL, "log.entry": EventClass.CRITICAL}
        self.assertEqual(classify_event("log.entry.recorded", Severity.DEBUG, overrides), EventClass.BULK)

    def test_an_override_beats_the_default_in_both_directions(self):
        self.assertEqual(
            classify_event("log.entry.recorded", Severity.TRACE, {"log.entry.recorded": EventClass.NORMAL}),
            EventClass.NORMAL,
        )
        self.assertEqual(
            classify_event("metric.recorded", Severity.INFO, {"metric.recorded": EventClass.BULK}), EventClass.BULK
        )

    def test_an_empty_map_works_and_is_not_changed(self):
        overrides: dict = {}
        self.assertEqual(classify_event("anything", Severity.ERROR, overrides), EventClass.NORMAL)
        self.assertEqual(overrides, {})


if __name__ == "__main__":
    unittest.main()
