"""The shared fixtures of integration-tests/fixtures give the expected results in Python (REQ-CNF-005).

Every file of the folders ``envelope`` and ``events`` is a case and its ``kind`` says how it is checked, so adding a
file needs no change here. The Go test reads the same files.
"""

import datetime
import json
import pathlib
import unittest

from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.canonical_json.time_format import format_utc_time
from telemetry.internal.domain.operations.event_validation.event_validation import validate_event

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[4]
FIXTURES_ROOT = REPOSITORY_ROOT / "integration-tests" / "fixtures"
FIXTURE_FOLDERS = ("envelope", "events")


def load_shared_fixtures() -> dict[str, dict]:
    """Load every case file of the shared folders, keyed by ``folder/file``."""
    cases: dict[str, dict] = {}
    for folder in FIXTURE_FOLDERS:
        paths = sorted((FIXTURES_ROOT / folder).glob("*.json"))
        if not paths:
            raise AssertionError(f"no fixture files found in {FIXTURES_ROOT / folder}")
        for path in paths:
            cases[f"{folder}/{path.name}"] = json.loads(path.read_text(encoding="utf-8"))
    return cases


class SharedFixturesTest(unittest.TestCase):
    def test_the_shared_fixtures_give_the_expected_results_in_python(self):
        for name, fixture in load_shared_fixtures().items():
            with self.subTest(case=name):
                kind = fixture["kind"]
                if kind == "canonical":
                    # The bytes are compared exactly, not as parsed JSON.
                    self.assertEqual(fixture["expected"].encode("utf-8"), canonical_json(fixture["input"]))
                elif kind == "time":
                    self.check_time(fixture)
                elif kind == "validation":
                    self.check_validation(fixture)
                else:
                    self.fail(f"unknown fixture kind {kind!r} (a typo must not be skipped silently)")

    def check_time(self, fixture: dict) -> None:
        instant = datetime.datetime.fromisoformat(fixture["input"])
        error_code = fixture.get("error_code")
        if error_code is not None:
            with self.assertRaises(ClassifiedError) as raised:
                format_utc_time(instant)
            self.assertEqual(error_code, raised.exception.code)
            return
        self.assertEqual(fixture["expected"], format_utc_time(instant))

    def check_validation(self, fixture: dict) -> None:
        event = fixture["input"]
        error = validate_event(event["event_type"], event.get("data"))
        error_code = fixture.get("error_code")
        if error_code is None:
            self.assertIsNone(error)
            return
        self.assertIsNotNone(error)
        self.assertEqual(error_code, error.code)
        for part in fixture.get("message_contains", []):
            self.assertIn(part, str(error))
