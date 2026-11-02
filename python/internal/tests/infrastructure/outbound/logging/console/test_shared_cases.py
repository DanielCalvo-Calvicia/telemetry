"""The shared cases of integration-tests/fixtures/console, read by the Go tests too.

The expected values were computed independently of both implementations.
"""

import dataclasses
import inspect
import io
import json
import pathlib
import unittest
from typing import Any

from internal.tests.application.services.masking_helpers import envelope_from_json
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.console.console_destination import ConsoleDestination
from telemetry.internal.infrastructure.outbound.logging.console.settings import Settings, validate_settings
from telemetry.logging import console as public_console

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[7]
CONSOLE_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "console"


def load_document(file_name: str) -> dict[str, Any]:
    """Read one fixture file; a missing folder or file is an error."""
    assert CONSOLE_FOLDER.is_dir(), f"the folder {CONSOLE_FOLDER} does not exist"
    assert sorted(CONSOLE_FOLDER.glob("*.json")), f"no fixture files found in {CONSOLE_FOLDER}"
    path = CONSOLE_FOLDER / file_name
    assert path.is_file(), f"the fixture {path} does not exist"
    return json.loads(path.read_text(encoding="utf-8"))


def load_cases(file_name: str) -> list[dict[str, Any]]:
    """Read the cases of one fixture file; an empty table is an error."""
    cases = load_document(file_name)["cases"]
    assert cases, f"the table of {file_name} is empty"
    return cases


class SharedConsoleCasesTest(unittest.TestCase):
    def test_the_output_and_the_result_are_the_expected_ones(self):
        for case in load_cases("lines.json"):
            with self.subTest(case=case["name"]):
                overrides = {"format": case["format"]}
                if "minimum_severity" in case:
                    overrides["minimum_severity"] = case["minimum_severity"]
                writer = io.StringIO()
                destination = ConsoleDestination(Settings(**overrides), writer)
                result = destination.send([envelope_from_json(event) for event in case["events"]])
                self.assertEqual(writer.getvalue(), case["expected_output"])
                expected = case["expected_result"]
                self.assertEqual(
                    (result.accepted, result.dropped, result.failed),
                    tuple(expected[k] for k in ("accepted", "dropped", "failed")),
                )
                self.assertEqual(result.reason, "")

    def test_the_settings_are_valid_with_the_expected_form_or_give_the_expected_problems(self):
        for case in load_cases("settings.json"):
            with self.subTest(case=case["description"]):
                settings = Settings(**case["settings"])
                problems = validate_settings(settings)
                if "expected" in case:
                    self.assertEqual(problems, ())
                    self.assertEqual(canonical_json(dataclasses.asdict(settings)).decode("utf-8"), case["expected"])
                    ConsoleDestination(settings, io.StringIO())
                else:
                    expected = [(problem["field"], problem["code"]) for problem in case["problems"]]
                    self.assertEqual([(problem.field, problem.code) for problem in problems], expected)
                    with self.assertRaises(ConfigurationError) as raised:
                        ConsoleDestination(settings, io.StringIO())
                    self.assertEqual([(p.field, p.code) for p in raised.exception.problems], expected)

    def test_every_shared_public_name_is_exported_and_documented(self):
        document = load_document("public_names.json")
        names = [entry["python"] for entry in document["functions"] + document["types"]]
        self.assertTrue(names, "the shared list of public names is empty")
        for name in names:
            with self.subTest(name=name):
                self.assertIn(name, public_console.__all__)
                self.assertTrue(hasattr(public_console, name))
        for name in public_console.__all__:
            with self.subTest(documented=name):
                self.assertTrue(inspect.getdoc(getattr(public_console, name)), f"{name} has no docstring")
        self.assertEqual(len(public_console.__all__), len(set(public_console.__all__)))
        self.assertTrue(public_console.__doc__)
