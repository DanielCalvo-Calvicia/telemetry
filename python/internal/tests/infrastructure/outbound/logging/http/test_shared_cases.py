"""The shared cases of integration-tests/fixtures/http_destination, read by the Go tests too.

The expected values were computed independently of both implementations.
"""

import dataclasses
import gzip
import inspect
import json
import pathlib
import unittest
from typing import Any

from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.infrastructure.outbound.shared.http_client.server_helpers import Reply, StubServer
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.http.http_destination import HttpDestination
from telemetry.internal.infrastructure.outbound.logging.http.settings import Settings, validate_settings
from telemetry.logging import http as public_http

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[7]
DESTINATION_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "http_destination"
REDIRECT_TARGET = "/redirected"


def load_document(file_name: str) -> dict[str, Any]:
    """Read one fixture file; a missing folder or file is an error."""
    assert DESTINATION_FOLDER.is_dir(), f"the folder {DESTINATION_FOLDER} does not exist"
    assert sorted(DESTINATION_FOLDER.glob("*.json")), f"no fixture files found in {DESTINATION_FOLDER}"
    path = DESTINATION_FOLDER / file_name
    assert path.is_file(), f"the fixture {path} does not exist"
    return json.loads(path.read_text(encoding="utf-8"))


def load_cases(file_name: str) -> list[dict[str, Any]]:
    """Read the cases of one fixture file; an empty table is an error."""
    cases = load_document(file_name)["cases"]
    assert cases, f"the table of {file_name} is empty"
    return cases


class SharedHttpDestinationCasesTest(unittest.TestCase):
    def test_the_request_and_the_result_are_the_expected_ones(self):
        for case in load_cases("requests.json"):
            with self.subTest(case=case["name"]):
                headers = {"Location": REDIRECT_TARGET} if 300 <= case["server_status"] < 400 else {}
                with StubServer(
                    lambda received, case=case, headers=headers: Reply(case["server_status"], headers)
                ) as server:
                    overrides: dict[str, Any] = {"url": server.url()}
                    if "minimum_severity" in case:
                        overrides["minimum_severity"] = case["minimum_severity"]
                    overrides.update(case.get("settings", {}))
                    destination = HttpDestination(Settings(**overrides))
                    result = destination.send([envelope_from_json(event) for event in case["events"]])
                    received = list(server.requests)
                expected = case["expected_result"]
                self.assertEqual(
                    (result.accepted, result.dropped, result.failed, result.reason),
                    tuple(expected[key] for key in ("accepted", "dropped", "failed", "reason")),
                )
                if case.get("expected_no_request"):
                    self.assertEqual(received, [])
                    continue
                self.assertEqual(len(received), 1, "exactly one request must arrive (a redirect is not followed)")
                request = received[0]
                self.assertNotEqual(request.path, REDIRECT_TARGET)
                body = request.body
                if request.headers.get("content-encoding") == "gzip":
                    body = gzip.decompress(body)
                self.assertEqual(body.decode("utf-8"), case["expected_body"])
                for name, value in case.get("expected_headers", {}).items():
                    self.assertEqual(request.headers.get(name.lower()), value, f"header {name}")
                if case.get("settings", {}).get("compress"):
                    self.assertEqual(request.headers.get("content-encoding"), "gzip")

    def test_the_settings_are_valid_with_the_expected_form_or_give_the_expected_problems(self):
        for case in load_cases("settings.json"):
            with self.subTest(case=case["description"]):
                settings = Settings(**case["settings"])
                problems = validate_settings(settings)
                if "expected" in case:
                    self.assertEqual(problems, ())
                    self.assertEqual(canonical_json(dataclasses.asdict(settings)).decode("utf-8"), case["expected"])
                    HttpDestination(settings)
                else:
                    expected = [(problem["field"], problem["code"]) for problem in case["problems"]]
                    self.assertEqual([(problem.field, problem.code) for problem in problems], expected)
                    with self.assertRaises(ConfigurationError) as raised:
                        HttpDestination(settings)
                    self.assertEqual([(p.field, p.code) for p in raised.exception.problems], expected)

    def test_every_shared_public_name_is_exported_and_documented(self):
        document = load_document("public_names.json")
        names = [entry["python"] for entry in document["functions"] + document["types"]]
        self.assertTrue(names, "the shared list of public names is empty")
        for name in names:
            with self.subTest(name=name):
                self.assertIn(name, public_http.__all__)
                self.assertTrue(hasattr(public_http, name))
        for name in public_http.__all__:
            with self.subTest(documented=name):
                self.assertTrue(inspect.getdoc(getattr(public_http, name)), f"{name} has no docstring")
        self.assertEqual(len(public_http.__all__), len(set(public_http.__all__)))
        self.assertTrue(public_http.__doc__)
