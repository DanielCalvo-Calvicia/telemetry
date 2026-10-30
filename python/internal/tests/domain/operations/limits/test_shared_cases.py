"""The shared limits cases of integration-tests/fixtures/limits, read by the Go tests too."""

import json
import pathlib
import unittest

from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.domain.operations.limits.helpers import limits_from_json
from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.limits.apply_limits import apply_limits
from telemetry.internal.domain.operations.limits.fits_in_batch import fits_in_batch

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
LIMITS_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "limits"
REPORT_FIELDS = ("attribute_keys_dropped", "attribute_keys_truncated", "attribute_values_truncated", "events_rejected")


class SharedLimitsCasesTest(unittest.TestCase):
    def test_every_shared_case_gives_the_expected_result(self):
        self.assertTrue(LIMITS_FOLDER.is_dir(), f"the folder {LIMITS_FOLDER} does not exist")
        paths = sorted(LIMITS_FOLDER.glob("*.json"))
        self.assertTrue(paths, f"no fixture files found in {LIMITS_FOLDER}")
        for path in paths:
            with self.subTest(case=path.name):
                case = json.loads(path.read_text(encoding="utf-8"))
                if case["kind"] == "limits":
                    self._check_limits_case(case)
                elif case["kind"] == "batch":
                    self._check_batch_case(case)
                else:
                    self.fail(f"unknown kind {case['kind']!r}")

    def _check_limits_case(self, case):
        options = case.get("options", {})
        if case.get("error_code") == "config_field_invalid":
            with self.assertRaises(ClassifiedError) as raised:
                limits_from_json(options)
            self.assertEqual(raised.exception.code, "config_field_invalid")
            return
        outcome = apply_limits(envelope_from_json(case["input"]), limits_from_json(options))
        for field_name in REPORT_FIELDS:
            self.assertEqual(getattr(outcome.report, field_name), case["report"][field_name], field_name)
        if "error_code" in case:
            self.assertIsNone(outcome.envelope)
            self.assertIsInstance(outcome.error, ClassifiedError)
            self.assertEqual(outcome.error.code, case["error_code"])
            return
        self.assertIsNone(outcome.error)
        self.assertEqual(canonical_json(outcome.envelope).decode("utf-8"), case["expected"])

    def _check_batch_case(self, case):
        self.assertTrue(case["cases"])
        for sub_case in case["cases"]:
            with self.subTest(sub_case=sub_case["description"]):
                limits = limits_from_json(sub_case.get("options", {}))
                sizes = sub_case["input"]
                self.assertEqual(
                    fits_in_batch(sizes["batch_bytes"], sizes["event_bytes"], limits), sub_case["expected"]
                )
