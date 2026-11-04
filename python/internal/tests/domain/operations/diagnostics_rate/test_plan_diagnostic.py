"""Black-box tests of plan_diagnostic: the 30 shared cases of integration-tests/fixtures/delivery and the edges."""

import dataclasses
import json
import pathlib
import unittest
from typing import Any

from telemetry.internal.domain.operations.diagnostics_rate.plan_diagnostic import (
    NO_WINDOW,
    DiagnosticPlan,
    plan_diagnostic,
)
from telemetry.internal.domain.rules import (
    DEFAULT_DIAGNOSTICS_QUEUE_SIZE,
    DEFAULT_DIAGNOSTICS_RATE_LIMIT,
    DEFAULT_DIAGNOSTICS_RATE_WINDOW_MILLISECONDS,
)

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
CASES_FILE = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "delivery" / "diagnostics_rate_cases.json"


def load_cases() -> list[dict[str, Any]]:
    cases = json.loads(CASES_FILE.read_text(encoding="utf-8"))["cases"]
    if len(cases) != 30:
        raise AssertionError(f"diagnostics_rate_cases.json: expected 30 cases, found {len(cases)}")
    return cases


class SharedCasesTest(unittest.TestCase):
    def test_every_case_of_the_fixture(self):
        for case in load_cases():
            with self.subTest(case=case["name"]):
                window_start_ms, count, suppressed = NO_WINDOW, 0, 0
                answers = []
                for now_ms in case["calls"]:
                    plan = plan_diagnostic(now_ms, window_start_ms, count, suppressed, case["limit"], case["window_ms"])
                    answers.append({"allow": plan.allow, "attach_suppressed": plan.attach_suppressed})
                    window_start_ms, count, suppressed = plan.window_start_ms, plan.count, plan.suppressed
                self.assertEqual(answers, case["expected"])
                self.assertEqual(
                    {"window_start_ms": window_start_ms, "count": count, "suppressed": suppressed},
                    case["expected_state"],
                )


class EdgesTest(unittest.TestCase):
    def test_the_defaults_are_the_contract_shared_with_go(self):
        self.assertEqual(
            (
                DEFAULT_DIAGNOSTICS_RATE_LIMIT,
                DEFAULT_DIAGNOSTICS_RATE_WINDOW_MILLISECONDS,
                DEFAULT_DIAGNOSTICS_QUEUE_SIZE,
            ),
            (10, 1000, 64),
        )

    def test_there_is_no_window_before_the_first_call(self):
        self.assertEqual(NO_WINDOW, -1)
        plan = plan_diagnostic(5, NO_WINDOW, 99, 7, 10, 1000)
        self.assertEqual(plan, DiagnosticPlan(True, 5, 1, 0, 7))

    def test_a_limit_of_zero_suppresses_everything(self):
        plan = plan_diagnostic(0, NO_WINDOW, 0, 0, 0, 1000)
        self.assertEqual(plan, DiagnosticPlan(False, 0, 0, 1, 0))

    def test_the_plan_is_immutable(self):
        plan = plan_diagnostic(0, NO_WINDOW, 0, 0, 1, 1000)
        with self.assertRaises(dataclasses.FrozenInstanceError):
            plan.allow = False  # type: ignore[misc]


if __name__ == "__main__":
    unittest.main()
