"""The shared cases of integration-tests/fixtures/file, read by the Go tests too.

The expected values were computed independently of both implementations.
"""

import dataclasses
import inspect
import json
import pathlib
import tempfile
import unittest
from typing import Any

from internal.tests.application.services.masking_helpers import envelope_from_json
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.file.file_destination import FileDestination
from telemetry.internal.infrastructure.outbound.logging.file.helpers.plan_quota import plan_quota
from telemetry.internal.infrastructure.outbound.logging.file.helpers.plan_rotation import plan_rotation
from telemetry.internal.infrastructure.outbound.logging.file.settings import Settings, validate_settings
from telemetry.logging import file as public_file

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[7]
FILE_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "file"


def load_document(file_name: str) -> dict[str, Any]:
    """Read one fixture file; a missing folder or file is an error."""
    assert FILE_FOLDER.is_dir(), f"the folder {FILE_FOLDER} does not exist"
    assert sorted(FILE_FOLDER.glob("*.json")), f"no fixture files found in {FILE_FOLDER}"
    path = FILE_FOLDER / file_name
    assert path.is_file(), f"the fixture {path} does not exist"
    return json.loads(path.read_text(encoding="utf-8"))


def load_cases(file_name: str) -> list[dict[str, Any]]:
    """Read the cases of one fixture file; an empty table is an error."""
    cases = load_document(file_name)["cases"]
    assert cases, f"the table of {file_name} is empty"
    return cases


class SharedFileCasesTest(unittest.TestCase):
    def test_the_files_and_the_counters_are_the_expected_ones(self):
        for case in load_cases("scenarios.json"):
            with self.subTest(case=case["name"]):
                with tempfile.TemporaryDirectory() as directory:
                    destination = FileDestination(Settings(directory=directory, **case["settings"]))
                    for batch in case["batches"]:
                        destination.send([envelope_from_json(event) for event in batch])
                    found = {
                        path.name: path.read_bytes().decode("utf-8")
                        for path in sorted(pathlib.Path(directory).iterdir())
                    }
                    self.assertEqual(found, case["expected_files"])
                    expected = case["expected_counters"]
                    self.assertEqual(destination.counters.files_rotated, expected["files_rotated"])
                    self.assertEqual(destination.counters.files_deleted_for_quota, expected["files_deleted_for_quota"])

    def test_the_scenarios_declare_the_size_of_a_line(self):
        self.assertGreater(load_document("scenarios.json")["line_bytes"], 0)

    def test_the_rotation_plan_is_the_expected_one(self):
        for case in load_cases("rotation_plan.json"):
            with self.subTest(case=case["description"]):
                steps = plan_rotation(case["base"], case["max_files"], case["existing"])
                found = [
                    {"op": step.op, "from": step.from_name, **({"to": step.to_name} if step.to_name else {})}
                    for step in steps
                ]
                self.assertEqual(found, case["expected_steps"])

    def test_the_quota_plan_is_the_expected_one(self):
        for case in load_cases("quota_plan.json"):
            with self.subTest(case=case["description"]):
                found = plan_quota(case["base"], case["max_files"], case["sizes"], case["quota_bytes"])
                self.assertEqual(found, case["expected_delete"])

    def test_the_settings_are_valid_with_the_expected_form_or_give_the_expected_problems(self):
        for case in load_cases("settings.json"):
            with self.subTest(case=case["description"]):
                settings = Settings(**case["settings"])
                problems = validate_settings(settings)
                if "expected" in case:
                    self.assertEqual(problems, ())
                    self.assertEqual(canonical_json(dataclasses.asdict(settings)).decode("utf-8"), case["expected"])
                else:
                    expected = [(problem["field"], problem["code"]) for problem in case["problems"]]
                    self.assertEqual([(problem.field, problem.code) for problem in problems], expected)
                    with self.assertRaises(ConfigurationError) as raised:
                        FileDestination(settings)
                    self.assertEqual([(p.field, p.code) for p in raised.exception.problems], expected)

    def test_every_shared_public_name_is_exported_and_documented(self):
        document = load_document("public_names.json")
        names = [entry["python"] for entry in document["functions"] + document["types"]]
        self.assertTrue(names, "the shared list of public names is empty")
        for name in names:
            with self.subTest(name=name):
                self.assertIn(name, public_file.__all__)
                self.assertTrue(hasattr(public_file, name))
        for name in public_file.__all__:
            with self.subTest(documented=name):
                self.assertTrue(inspect.getdoc(getattr(public_file, name)), f"{name} has no docstring")
        self.assertEqual(len(public_file.__all__), len(set(public_file.__all__)))
        self.assertTrue(public_file.__doc__)
