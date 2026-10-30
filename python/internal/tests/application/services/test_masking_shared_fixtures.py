"""The shared masking cases of integration-tests/fixtures/masking/envelopes, read by the Go tests too."""

import json
import pathlib
import unittest

from internal.tests.application.services.masking_helpers import envelope_from_json, masker_from_json
from telemetry.internal.application.services.masking_service import MaskingService
from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[5]
ENVELOPES_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "masking" / "envelopes"


class SharedMaskingCasesTest(unittest.TestCase):
    def test_every_shared_case_gives_the_expected_result(self):
        self.assertTrue(ENVELOPES_FOLDER.is_dir(), f"the folder {ENVELOPES_FOLDER} does not exist")
        paths = sorted(ENVELOPES_FOLDER.glob("*.json"))
        self.assertTrue(paths, f"no fixture files found in {ENVELOPES_FOLDER}")
        for path in paths:
            with self.subTest(case=path.name):
                case = json.loads(path.read_text(encoding="utf-8"))
                self.assertEqual(case["kind"], "mask")
                options = case.get("options", {})
                if "error_code" in case:
                    with self.assertRaises(ClassifiedError) as raised:
                        masker_from_json(options)
                    self.assertEqual(raised.exception.code, case["error_code"])
                    continue
                service = MaskingService(masker_from_json(options))
                result = service.mask(envelope_from_json(case["input"]))
                self.assertIsNone(result.error)
                self.assertIsNotNone(result.envelope)
                self.assertEqual(canonical_json(result.envelope).decode("utf-8"), case["expected"])
