"""The shared cases of integration-tests/fixtures/aws_http, read by the Go tests too.

The expected values were computed independently of both implementations. Every translated event is sent to a local
http.server and the body that arrives is compared byte for byte with the fixture.
"""

import dataclasses
import inspect
import json
import pathlib
import unittest
from typing import Any

from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.infrastructure.outbound.shared.http_client.server_helpers import StubServer
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.aws_http.aws_http_destination import AwsHttpDestination
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.build_add_log_request import (
    build_add_log_message,
    build_add_log_request,
)
from telemetry.internal.infrastructure.outbound.logging.aws_http.settings import Settings, validate_settings
from telemetry.logging import aws_http as public_aws_http

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[7]
FIXTURE_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "aws_http"
PATH = "/api/v1/cloudwatch/logs/add"


def load_document(file_name: str) -> dict[str, Any]:
    """Read one fixture file; a missing folder or file is an error."""
    path = FIXTURE_FOLDER / file_name
    assert path.is_file(), f"the fixture {path} does not exist"
    return json.loads(path.read_text(encoding="utf-8"))


class SharedAwsHttpCasesTest(unittest.TestCase):
    def test_the_request_is_the_expected_one_byte_for_byte(self):
        document = load_document("add_log_request.json")
        self.assertTrue(document["cases"], "the table of add_log_request.json is empty")
        for case in document["cases"]:
            with self.subTest(case=case["name"]):
                with StubServer() as server:
                    settings = Settings(
                        host="http://127.0.0.1",
                        port=str(server.port),
                        path=PATH,
                        log_group_name=document["log_group_name"],
                        log_stream_name=document["log_stream_name"],
                    )
                    delivery = AwsHttpDestination(settings).deliver([envelope_from_json(case["event"])])
                    received = list(server.requests)
                if case.get("expected_skipped"):
                    self.assertEqual(received, [])
                    self.assertEqual((delivery.sent, delivery.skipped, delivery.outcome), (0, 1, None))
                    self.assertEqual(dataclasses.astuple(delivery.result), (1, 0, 0, "", 0))
                    continue
                self.assertEqual(len(received), 1)
                request = received[0]
                self.assertEqual((request.method, request.path), ("POST", PATH))
                self.assertEqual(request.headers.get("content-type"), "application/json")
                self.assertNotIn("content-encoding", request.headers)
                self.assertEqual(request.body.decode("utf-8"), case["expected_request"])
                self.assertEqual(dataclasses.astuple(delivery.result), (1, 0, 0, "", 0))
                self.assertEqual((delivery.sent, delivery.skipped), (1, 0))
                message = json.loads(request.body)["message"]
                self.assertEqual(message, case["expected_message"])

    def test_the_helpers_build_the_expected_bytes_without_a_server(self):
        for case in load_document("add_log_request.json")["cases"]:
            if case.get("expected_skipped"):
                continue
            with self.subTest(case=case["name"]):
                event = envelope_from_json(case["event"])
                self.assertEqual(build_add_log_message(event), case["expected_message"])
                body = build_add_log_request(event, "my-log-group", "my-log-stream")
                self.assertEqual(body.decode("utf-8"), case["expected_request"])

    def test_the_settings_are_valid_with_the_expected_form_or_give_the_expected_problems(self):
        cases = load_document("settings.json")["cases"]
        self.assertTrue(cases, "the table of settings.json is empty")
        for case in cases:
            with self.subTest(case=case["description"]):
                settings = Settings(**case["settings"])
                problems = validate_settings(settings)
                if "expected" in case:
                    self.assertEqual(problems, ())
                    self.assertEqual(canonical_json(dataclasses.asdict(settings)).decode("utf-8"), case["expected"])
                    AwsHttpDestination(settings)
                else:
                    expected = [(problem["field"], problem["code"]) for problem in case["problems"]]
                    self.assertEqual([(problem.field, problem.code) for problem in problems], expected)
                    with self.assertRaises(ConfigurationError) as raised:
                        AwsHttpDestination(settings)
                    self.assertEqual([(p.field, p.code) for p in raised.exception.problems], expected)

    def test_every_shared_public_name_is_exported_and_documented(self):
        document = load_document("public_names.json")
        names = [entry["python"] for entry in document["functions"] + document["types"]]
        self.assertTrue(names, "the shared list of public names is empty")
        for name in names:
            with self.subTest(name=name):
                self.assertIn(name, public_aws_http.__all__)
                self.assertTrue(hasattr(public_aws_http, name))
        for name in public_aws_http.__all__:
            with self.subTest(documented=name):
                self.assertTrue(inspect.getdoc(getattr(public_aws_http, name)), f"{name} has no docstring")
        self.assertEqual(sorted(public_aws_http.__all__), sorted(names))
        self.assertTrue(public_aws_http.__doc__)
