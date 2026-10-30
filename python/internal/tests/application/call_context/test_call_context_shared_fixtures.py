"""The shared cases of integration-tests/fixtures/call_context: the id shapes and the mapping to the envelope."""

import json
import pathlib
import unittest

from telemetry.internal.application.call_context.correlation import to_correlation_context
from telemetry.internal.application.call_context.new_call_context import new_call_context
from telemetry.internal.application.mappers.envelope_mapper import to_context
from telemetry.internal.domain.errors import error_code_of
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[5]
CALL_CONTEXT_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "call_context"


def load_cases(file_name: str) -> list:
    """Read one shared table; fail when the folder, the file or the table is missing or empty."""
    assert CALL_CONTEXT_FOLDER.is_dir(), f"the folder {CALL_CONTEXT_FOLDER} does not exist"
    assert list(CALL_CONTEXT_FOLDER.glob("*.json")), f"no fixture files found in {CALL_CONTEXT_FOLDER}"
    path = CALL_CONTEXT_FOLDER / file_name
    assert path.is_file(), f"the file {path} does not exist"
    cases = json.loads(path.read_text(encoding="utf-8"))["cases"]
    assert cases, f"the table {path} has no cases"
    return cases


class SharedCallContextFixturesTest(unittest.TestCase):
    def test_the_shapes_of_the_ids(self):
        for case in load_cases("ids.json"):
            with self.subTest(case=case["description"]):
                field_name, value = case["input"]["field"], case["input"]["value"]
                result = new_call_context(**{field_name: value})
                if case["valid"]:
                    self.assertIsNone(result.error)
                    self.assertEqual(getattr(result.call_context, field_name), value)
                    continue
                self.assertIsNone(result.call_context)
                self.assertEqual(error_code_of(result.error), "event_schema_invalid")
                if value.strip():  # a blank value (" ") is trivially a substring of any message
                    self.assertNotIn(value, str(result.error))

    def test_the_mapping_to_the_envelope_context(self):
        for case in load_cases("mapping.json"):
            with self.subTest(case=case["description"]):
                result = new_call_context(**case["input"])
                self.assertIsNone(result.error)
                domain_context, error = to_correlation_context(result.call_context)
                self.assertIsNone(error)
                if case["expected"] is None:
                    self.assertIsNone(domain_context)
                    continue
                envelope_context = to_context(domain_context)
                self.assertEqual(canonical_json(envelope_context).decode("utf-8"), case["expected"])


if __name__ == "__main__":
    unittest.main()
