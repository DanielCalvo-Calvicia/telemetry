"""The shared cases of the fake receiver (integration-tests/fixtures/http/receiver_script_examples.json).

The Go fake and this one must answer the same script the same way, so both run the same table.
"""

import gzip
import http.client
import json
import time
import unittest
from pathlib import Path
from typing import Any

from internal.tests.fakes.fake_receiver import Answer, Config, Receiver

CASES_FILE = (
    Path(__file__).resolve().parents[5] / "integration-tests" / "fixtures" / "http" / "receiver_script_examples.json"
)
CONNECTION_ERROR = "connection_error"
BODY_ERROR = "body_error"


def load_cases() -> list:
    """The cases of the shared file; a missing file or an empty table is a failure, never a skip."""
    with CASES_FILE.open("r", encoding="utf-8") as handle:
        cases = json.load(handle)["cases"]
    if not cases:
        raise AssertionError("the shared table of the fake receiver is empty")
    return cases


def build_config(raw: dict) -> Config:
    """A Config from the ``config`` object of a case."""
    api_key = raw.get("expected_api_key", {})
    return Config(
        expected_bearer=raw.get("expected_bearer", ""),
        expected_api_key_header=api_key.get("header", ""),
        expected_api_key_value=api_key.get("value", ""),
        default_status=raw.get("default_status", 0),
    )


def build_answer(raw: dict) -> Answer:
    """An Answer from an item of the ``script`` of a case; ``delay_ms`` is converted to seconds."""
    return Answer(
        status=raw.get("status", 0),
        retry_after=raw.get("retry_after", ""),
        location=raw.get("location", ""),
        delay_seconds=raw.get("delay_ms", 0) / 1000.0,
        close=raw.get("close", False),
        truncate=raw.get("truncate", False),
    )


def send(receiver: Receiver, spec: dict) -> dict:
    """POST one request on a new connection and classify the result as the table does."""
    host, port = receiver.url().removeprefix("http://").split(":")
    body = spec["body"].encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if spec.get("gzip"):
        body = gzip.compress(body)
        headers["Content-Encoding"] = "gzip"
    headers.update(spec.get("headers", {}))
    started = time.monotonic()
    connection = http.client.HTTPConnection(host, int(port), timeout=5)
    try:
        try:
            connection.request("POST", "/", body=body, headers=headers)
            response = connection.getresponse()
        except (ConnectionError, http.client.BadStatusLine, OSError):
            return {"status": CONNECTION_ERROR, "headers": {}, "body": "", "elapsed": time.monotonic() - started}
        try:
            text = response.read().decode("utf-8", "replace")
        except http.client.IncompleteRead:
            return {"status": BODY_ERROR, "headers": {}, "body": "", "elapsed": time.monotonic() - started}
        return {
            "status": response.status,
            "headers": {name.lower(): value for name, value in response.getheaders()},
            "body": text,
            "elapsed": time.monotonic() - started,
        }
    finally:
        connection.close()


class SharedCasesTest(unittest.TestCase):
    def test_the_table_is_not_empty(self):
        self.assertGreater(len(load_cases()), 0)

    def test_every_case(self):
        for case in load_cases():
            with self.subTest(case=case["description"][:70]), Receiver(build_config(case["config"])) as receiver:
                receiver.script(*[build_answer(item) for item in case["script"]])
                self.assertEqual(len(case["requests"]), len(case["expected"]))
                for index, (spec, expected) in enumerate(zip(case["requests"], case["expected"])):
                    self.check_exchange(index, send(receiver, spec), expected)
                self.check_recorded(receiver, case["expected_recorded"])

    def check_exchange(self, index: int, got: dict, expected: dict) -> None:
        self.assertEqual(got["status"], expected["status"], f"request {index}")
        for name, value in expected.get("headers", {}).items():
            self.assertEqual(got["headers"].get(name.lower()), value, f"request {index}, header {name}")
        if "body_contains" in expected:
            self.assertIn(expected["body_contains"], got["body"], f"request {index}")
        if "min_elapsed_ms" in expected:
            self.assertGreaterEqual(got["elapsed"] * 1000, expected["min_elapsed_ms"], f"request {index}")

    def check_recorded(self, receiver: Receiver, expected: dict[str, Any]) -> None:
        recorded = receiver.requests()
        self.assertEqual(len(recorded), expected["requests"])
        self.assertEqual(sum(len(item.events or []) for item in recorded if item.valid), expected["events"])
        self.assertEqual(len(receiver.events()), expected["events"])
        reasons = [item.invalid_reason for item in recorded if item.invalid_reason]
        self.assertEqual(len(reasons), len(expected["invalid"]))
        for reason, part in zip(reasons, expected["invalid"]):
            self.assertIn(part, reason)
        self.assertEqual(sum(1 for item in recorded if item.gzipped), expected.get("gzip_requests", 0))


if __name__ == "__main__":
    unittest.main()
