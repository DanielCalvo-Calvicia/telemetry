"""The shared limits cases of integration-tests/fixtures/limits, through LimitsService."""

import json
import pathlib
import unittest

from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.domain.operations.limits.helpers import limits_from_json
from telemetry.internal.application.services.limits_service import LimitsService
from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[5]
LIMITS_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "limits"
REPORT_FIELDS = ("attribute_keys_dropped", "attribute_keys_truncated", "attribute_values_truncated", "events_rejected")


class SharedLimitsFixturesTest(unittest.TestCase):
    def test_every_shared_case_gives_the_expected_result(self):
        self.assertTrue(LIMITS_FOLDER.is_dir(), f"the folder {LIMITS_FOLDER} does not exist")
        paths = sorted(LIMITS_FOLDER.glob("*.json"))
        self.assertTrue(paths, f"no fixture files found in {LIMITS_FOLDER}")
        for path in paths:
            with self.subTest(case=path.name):
                case = json.loads(path.read_text(encoding="utf-8"))
                if case["kind"] == "batch":
                    continue  # fits_in_batch is a domain function, read by the domain test
                self.assertEqual(case["kind"], "limits")
                options = case.get("options", {})
                if case.get("error_code") == "config_field_invalid":
                    with self.assertRaises(ClassifiedError) as raised:
                        LimitsService(limits_from_json(options))
                    self.assertEqual(raised.exception.code, "config_field_invalid")
                    continue
                result = LimitsService(limits_from_json(options)).apply(envelope_from_json(case["input"]))
                for field_name in REPORT_FIELDS:
                    self.assertEqual(getattr(result.report, field_name), case["report"][field_name], field_name)
                if "error_code" in case:
                    self.assertIsNone(result.envelope)
                    self.assertIsInstance(result.error, ClassifiedError)
                    self.assertEqual(result.error.code, case["error_code"])
                    continue
                self.assertIsNone(result.error)
                self.assertEqual(canonical_json(result.envelope).decode("utf-8"), case["expected"])
