"""The shared cases of the logger (integration-tests/fixtures/logging), run through the public package only.

The same files are run by the Go half; the expected values were computed independently of both implementations.
"""

import json
import logging
import pathlib
import unittest
from typing import Any

import telemetry
from internal.tests.e2e.case_helpers import build_options, delivered_texts
from internal.tests.infrastructure.config.helpers import merged_config
from telemetry.internal.infrastructure.inbound.sdk.logging_handler import severity_method_name

LOGGING_FOLDER = pathlib.Path(__file__).resolve().parents[4] / "integration-tests" / "fixtures" / "logging"
KNOWN_KINDS = ("log", "handler", "levels")
LOG_METHODS = ("trace", "debug", "info", "warn", "error", "critical")


def load_cases() -> list[tuple[str, dict[str, Any]]]:
    """Read every case; fail when the folder is missing, empty or holds an unknown kind."""
    if not LOGGING_FOLDER.is_dir():
        raise AssertionError(f"the folder of the shared cases is missing: {LOGGING_FOLDER}")
    paths = sorted(LOGGING_FOLDER.glob("*.json"))
    if not paths:
        raise AssertionError(f"no shared cases in {LOGGING_FOLDER}")
    cases = []
    for path in paths:
        case = json.loads(path.read_text(encoding="utf-8"))
        if case.get("kind") not in KNOWN_KINDS:
            raise AssertionError(f"{path.name}: unknown kind {case.get('kind')!r}")
        cases.append((path.name, case))
    return cases


def new_record(python_level: int, message: str, attrs: dict[str, Any]) -> logging.LogRecord:
    """A record as ``logging`` builds it, with the attrs as the ``extra`` of the call."""
    record = logging.LogRecord("shared", python_level, __file__, 1, message, None, None)
    record.__dict__.update(attrs)
    return record


class LoggingSharedCasesTest(unittest.TestCase):
    def test_every_shared_case(self):
        for name, case in load_cases():
            with self.subTest(case=name):
                self.run_case(case)

    def test_the_cases_exist(self):
        kinds = {case["kind"] for _, case in load_cases()}
        self.assertEqual(kinds, set(KNOWN_KINDS))
        self.assertGreaterEqual(len(load_cases()), 14)

    def run_case(self, case: dict[str, Any]) -> None:
        if case["kind"] == "levels":
            self.run_levels(case)
            return
        config = merged_config(case["config"]["profile"], case["config"].get("overrides", {}))
        options, sinks = build_options(case)
        core = telemetry.new(config, *options)
        context_fields = case.get("call_context")
        if context_fields is None:
            self.run_calls(core, case)
        else:
            created = telemetry.new_call_context(**context_fields)
            self.assertIsNone(created.error)
            with telemetry.use_call_context(created.call_context):
                self.run_calls(core, case)
        for kind, expected_sinks in case["expected"]["deliveries"].items():
            self.assertEqual(delivered_texts(sinks[kind]), expected_sinks, kind)

    def run_calls(self, core: telemetry.Telemetry, case: dict[str, Any]) -> None:
        if case["kind"] == "log":
            for call in case["calls"]:
                self.assertIn(call["method"], LOG_METHODS)
                log_entry = telemetry.LogEntry(
                    category=call["category"],
                    message=call["message"],
                    function=call.get("function", ""),
                    fields=call.get("fields"),
                )
                self.assertIsNone(getattr(core.logger, call["method"])(log_entry))
            return
        handler = telemetry.LoggingHandler(core.logger, case["default_category"])
        for record in case["records"]:
            handler.handle(new_record(record["python_level"], record["message"], record["attrs"]))

    def run_levels(self, case: dict[str, Any]) -> None:
        self.assertTrue(case["cases"])
        for row in case["cases"]:
            with self.subTest(level=row["python_level"]):
                self.assertEqual(severity_method_name(row["python_level"]).upper(), row["severity"])
                # and through the handler: the severity of the event is the expected one
                calls: list[str] = []

                class Recorder:
                    def __getattr__(self, name: str):
                        return lambda log_entry: calls.append(name.upper())

                telemetry.LoggingHandler(Recorder()).handle(new_record(row["python_level"], "m", {}))
                self.assertEqual(calls, [row["severity"]])


if __name__ == "__main__":
    unittest.main()
