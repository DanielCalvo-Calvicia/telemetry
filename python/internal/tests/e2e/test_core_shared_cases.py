"""The shared cases of the core (integration-tests/fixtures/core), run through the public package only.

The same files are run by the Go half; the expected values were computed independently of both implementations.
"""

import json
import pathlib
import unittest
from typing import Any

import telemetry
from internal.tests.e2e.case_helpers import build_options, parse_time
from internal.tests.infrastructure.config.helpers import merged_config
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant

CORE_FOLDER = pathlib.Path(__file__).resolve().parents[4] / "integration-tests" / "fixtures" / "core"
SKIPPED_FILES = ("public_names.json",)
KNOWN_KINDS = ("core",)


def load_cases() -> list[tuple[str, dict[str, Any]]]:
    """Read every case; fail when the folder is missing, empty or holds an unknown kind."""
    if not CORE_FOLDER.is_dir():
        raise AssertionError(f"the folder of the shared cases is missing: {CORE_FOLDER}")
    paths = sorted(path for path in CORE_FOLDER.glob("*.json") if path.name not in SKIPPED_FILES)
    if not paths:
        raise AssertionError(f"no shared cases in {CORE_FOLDER}")
    cases = []
    for path in paths:
        case = json.loads(path.read_text(encoding="utf-8"))
        if case.get("kind") not in KNOWN_KINDS:
            raise AssertionError(f"{path.name}: unknown kind {case.get('kind')!r}")
        cases.append((path.name, case))
    return cases


def build_event(emit: dict[str, Any]) -> telemetry.NewEventInput:
    """Build the input of one emit of a case."""
    occurred_at = emit.get("occurred_at")
    actor = emit.get("actor")
    attributes = emit.get("attributes")
    return telemetry.NewEventInput(
        event_type=emit["event_type"],
        severity=telemetry.Severity(emit["severity"]),
        data=emit["data"],
        occurred_at=None if occurred_at is None else UtcInstant(parse_time(occurred_at)),
        actor=None if actor is None else telemetry.ActorIdentity(**actor),
        attributes=None if attributes is None else telemetry.EventAttributes(attributes),
    )


def as_dict(result: Any) -> dict[str, Any]:
    """The fields of an EmitResult as a mapping."""
    return {"accepted": result.accepted, "dropped": result.dropped, "failed": result.failed, "reason": result.reason}


class CoreSharedCasesTest(unittest.TestCase):
    def test_every_shared_case(self):
        for name, case in load_cases():
            with self.subTest(case=name):
                self.run_case(case)

    def test_the_cases_exist_and_the_loader_refuses_what_is_wrong(self):
        self.assertGreaterEqual(len(load_cases()), 15)

    def run_case(self, case: dict[str, Any]) -> None:
        config = merged_config(case["config"]["profile"], case["config"].get("overrides", {}))
        options, sinks = build_options(case)
        if "new_error" in case:
            with self.assertRaises(telemetry.ConfigurationError) as raised:
                telemetry.new(config, *options)
            actual = [{"field": problem.field, "code": problem.code} for problem in raised.exception.problems]
            self.assertEqual(actual, case["new_error"]["problems"])
            return
        core = telemetry.new(config, *options)
        results = []
        context_fields = case.get("call_context")
        if context_fields is None:
            results = [core.emit(build_event(emit)) for emit in case["emits"]]
        else:
            created = telemetry.new_call_context(**context_fields)
            self.assertIsNone(created.error)
            with telemetry.use_call_context(created.call_context):
                results = [core.emit(build_event(emit)) for emit in case["emits"]]
        expected = case["expected"]
        self.assertEqual([as_dict(result) for result in results], expected["results"])
        for kind, expected_sinks in expected["deliveries"].items():
            actual_sinks = [
                [canonical_json(envelope).decode("utf-8") for batch in sink.batches for envelope in batch]
                for sink in sinks[kind]
            ]
            self.assertEqual(actual_sinks, expected_sinks, kind)
        shutdown = core.shutdown()
        self.assertEqual(
            {"sent": shutdown.sent, "spooled": shutdown.spooled, "dropped": shutdown.dropped}, expected["shutdown"]
        )


if __name__ == "__main__":
    unittest.main()
