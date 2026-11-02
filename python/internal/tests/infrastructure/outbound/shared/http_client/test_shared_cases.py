"""The shared HTTP cases of integration-tests/fixtures/http, read by the Go tests too."""

import dataclasses
import datetime
import gzip
import json
import pathlib
import unittest
from typing import Any

from internal.tests.application.services.masking_helpers import envelope_from_json
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.auth_header import auth_header
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.backoff_with_jitter import (
    backoff_with_jitter,
)
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.build_batch_body import build_batch_body
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.classify_response import classify_status
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.compress_gzip import compress_gzip
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.parse_retry_after import parse_retry_after
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import HttpSettings, validate_settings

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[7]
HTTP_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "http"


def load_cases(file_name: str) -> list[dict[str, Any]]:
    """Read the cases of one fixture file; a missing folder or file, or an empty table, is an error."""
    assert HTTP_FOLDER.is_dir(), f"the folder {HTTP_FOLDER} does not exist"
    assert sorted(HTTP_FOLDER.glob("*.json")), f"no fixture files found in {HTTP_FOLDER}"
    path = HTTP_FOLDER / file_name
    assert path.is_file(), f"the fixture {path} does not exist"
    cases = json.loads(path.read_text(encoding="utf-8"))["cases"]
    assert cases, f"the table of {file_name} is empty"
    return cases


def parse_instant(text: str) -> datetime.datetime:
    """Parse an RFC 3339 instant that ends with Z (fromisoformat reads no Z before Python 3.11)."""
    return datetime.datetime.strptime(text, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=datetime.timezone.utc)


class SharedHttpCasesTest(unittest.TestCase):
    def test_the_batch_body_is_the_expected_text_plain_and_gzipped(self):
        for case in load_cases("batch_body.json"):
            with self.subTest(case=case["description"]):
                events = [envelope_from_json(event) for event in case["events"]]
                body = build_batch_body(events)
                self.assertEqual(body.decode("utf-8"), case["expected"])
                compressed = compress_gzip(body)
                self.assertEqual(gzip.decompress(compressed), body)
                self.assertEqual(compress_gzip(body), compressed)

    def test_every_status_has_the_expected_class(self):
        for case in load_cases("response_classification.json"):
            with self.subTest(case=case["description"]):
                self.assertEqual(classify_status(case["status"]), case["class"])

    def test_retry_after_gives_the_expected_milliseconds(self):
        for case in load_cases("retry_after.json"):
            with self.subTest(case=case["description"]):
                expected = case["expected_seconds"]
                result = parse_retry_after(case["header"], parse_instant(case["now"]))
                self.assertEqual(result, None if expected is None else expected * 1000)

    def test_the_backoff_gives_the_expected_milliseconds(self):
        for case in load_cases("backoff.json"):
            with self.subTest(case=case["description"]):
                result = backoff_with_jitter(case["attempt"], case["base_ms"], case["max_ms"], case["random"])
                self.assertEqual(result, case["expected_ms"])

    def test_the_authentication_header_is_the_expected_one(self):
        for case in load_cases("auth_header.json"):
            with self.subTest(case=case["description"]):
                settings = case["settings"]
                result = auth_header(
                    settings.get("bearer_token", ""),
                    settings.get("api_key_header", ""),
                    settings.get("api_key_value", ""),
                )
                expected = case["expected"]
                self.assertEqual(result, None if expected is None else (expected["name"], expected["value"]))

    def test_the_settings_are_valid_with_the_expected_form_or_give_the_expected_problems(self):
        for case in load_cases("settings.json"):
            with self.subTest(case=case["description"]):
                settings = HttpSettings(**case["settings"])
                problems = validate_settings(settings)
                if "expected" in case:
                    self.assertEqual(problems, ())
                    self.assertEqual(canonical_json(dataclasses.asdict(settings)).decode("utf-8"), case["expected"])
                else:
                    expected = [(problem["field"], problem["code"]) for problem in case["problems"]]
                    self.assertEqual([(problem.field, problem.code) for problem in problems], expected)
