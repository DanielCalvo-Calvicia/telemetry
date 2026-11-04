"""Black-box tests of the retry rule: the shared backoff and Retry-After cases, the decision to retry and the edges."""

import json
import pathlib
import unittest
from typing import Any

from telemetry.internal.domain.operations.retry_delay.retry_delay import (
    MAX_RETRY_AFTER_MILLISECONDS,
    backoff_with_jitter,
    next_delay,
    retry_delay,
)

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
HTTP_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "http"


def load_cases(file_name: str) -> list[dict[str, Any]]:
    """Read the cases of one fixture file of the shared HTTP rules; a missing file or an empty table is an error."""
    path = HTTP_FOLDER / file_name
    assert path.is_file(), f"the fixture {path} does not exist"
    cases = json.loads(path.read_text(encoding="utf-8"))["cases"]
    assert cases, f"the table of {file_name} is empty"
    return cases


class SharedCasesTest(unittest.TestCase):
    def test_the_backoff_gives_the_expected_milliseconds(self):
        for case in load_cases("backoff.json"):
            with self.subTest(case=case["description"]):
                result = backoff_with_jitter(case["attempt"], case["base_ms"], case["max_ms"], case["random"])
                self.assertEqual(result, case["expected_ms"])

    def test_the_next_delay_with_no_hint_is_the_backoff(self):
        for case in load_cases("backoff.json"):
            for hint in (None, 0, -5):
                with self.subTest(case=case["description"], hint=hint):
                    result = next_delay(case["attempt"], case["base_ms"], case["max_ms"], hint, case["random"])
                    self.assertEqual(result, case["expected_ms"])

    def test_the_retry_decision_gives_the_backoff_of_every_case_while_attempts_remain(self):
        for case in load_cases("backoff.json"):
            with self.subTest(case=case["description"]):
                attempts = case["attempt"] + 2
                result = retry_delay(
                    case["attempt"], attempts, True, None, case["base_ms"], case["max_ms"], case["random"]
                )
                self.assertEqual(result, (case["expected_ms"], True))

    def test_the_hints_of_the_retry_after_cases_are_waited_whole(self):
        checked = 0
        for case in load_cases("retry_after.json"):
            seconds = case["expected_seconds"]
            if seconds is None:
                continue
            with self.subTest(case=case["description"]):
                self.assertEqual(next_delay(0, 1, 1, seconds * 1000, 0.0), seconds * 1000)
                checked += 1
        self.assertGreater(checked, 0)


class BackoffTest(unittest.TestCase):
    def test_a_large_attempt_stops_at_the_maximum_and_the_random_is_cut(self):
        self.assertEqual(backoff_with_jitter(10**9, 200, 30000, 1.0), 30000)
        self.assertEqual(backoff_with_jitter(2, 200, 30000, 5.0), 800)
        self.assertEqual(backoff_with_jitter(2, 200, 30000, -3.0), 400)
        self.assertEqual(backoff_with_jitter(2, 200, 30000, float("nan")), 400)

    def test_a_base_above_the_maximum_is_cut(self):
        self.assertEqual(backoff_with_jitter(0, 1000, 500, 1.0), 500)

    def test_the_delays_grow_with_the_attempt_and_carry_the_jitter(self):
        for random_value in (0.0, 0.5, 1.0):
            delays = [backoff_with_jitter(attempt, 200, 30000, random_value) for attempt in range(8)]
            self.assertEqual(delays, sorted(delays))
            self.assertLess(delays[0], delays[-1])
        self.assertEqual(
            [backoff_with_jitter(2, 200, 30000, random_value) for random_value in (0.0, 0.5, 1.0)], [400, 600, 800]
        )

    def test_the_same_inputs_give_the_same_delay(self):
        self.assertEqual(backoff_with_jitter(3, 200, 30000, 0.37), backoff_with_jitter(3, 200, 30000, 0.37))


class NextDelayTest(unittest.TestCase):
    def test_the_larger_of_the_backoff_and_the_hint_wins(self):
        self.assertEqual(next_delay(0, 200, 30000, 5000, 1.0), 5000)
        self.assertEqual(next_delay(0, 200, 30000, 10, 1.0), 200)

    def test_a_hint_is_cut_to_a_day(self):
        self.assertEqual(next_delay(0, 200, 30000, 10**12, 0.5), MAX_RETRY_AFTER_MILLISECONDS)
        self.assertEqual(next_delay(0, 200, 30000, MAX_RETRY_AFTER_MILLISECONDS, 0.5), MAX_RETRY_AFTER_MILLISECONDS)


class RetryDecisionTest(unittest.TestCase):
    def test_a_transient_failure_retries_until_the_last_attempt(self):
        self.assertEqual(retry_delay(0, 5, True, None, 200, 30000, 0.0), (100, True))
        self.assertEqual(retry_delay(3, 5, True, None, 200, 30000, 1.0), (1600, True))
        self.assertEqual(retry_delay(4, 5, True, None, 200, 30000, 0.5), (0, False))

    def test_a_failure_that_is_not_transient_never_retries(self):
        self.assertEqual(retry_delay(0, 5, False, 9000, 200, 30000, 0.5), (0, False))

    def test_one_attempt_never_retries(self):
        self.assertEqual(retry_delay(0, 1, True, None, 200, 30000, 0.5), (0, False))

    def test_the_hint_of_the_destination_is_honored(self):
        self.assertEqual(retry_delay(0, 5, True, 5000, 200, 30000, 1.0), (5000, True))
        self.assertEqual(retry_delay(0, 5, True, 10, 200, 30000, 1.0), (200, True))


if __name__ == "__main__":
    unittest.main()
