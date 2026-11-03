"""Black-box tests of plan_batch: the shared cases of integration-tests/fixtures/delivery, the edges and a property."""

import dataclasses
import json
import pathlib
import random
import unittest
from typing import Any, Optional

from telemetry.internal.domain.operations.batching.plan_batch import (
    PLAN_IDLE,
    PLAN_SEND,
    PLAN_WAIT,
    Plan,
    plan_batch,
)

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
CASES_FILE = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "delivery" / "batching_cases.json"


def simulate(case: dict[str, Any]) -> list[dict[str, Any]]:
    """Run the algorithm of the description of the fixture with plan_batch and return the batches it sends."""
    arrivals = case["arrivals"]
    queue: list[tuple[int, int]] = []  # (id, arrival) of the events waiting, oldest first
    batches: list[dict[str, Any]] = []
    now = 0
    free_at = 0
    pushed = 0
    while True:
        while pushed < len(arrivals) and arrivals[pushed] <= now:
            queue.append((pushed + 1, arrivals[pushed]))
            pushed += 1
        next_decision: Optional[int] = None
        if now >= free_at:
            plan = plan_batch(
                now, len(queue), queue[0][1] if queue else 0, case["batch_size"], case["flush_interval_ms"]
            )
            if plan.action == PLAN_SEND:
                taken, queue = queue[: plan.take], queue[plan.take :]
                batches.append({"at_ms": now, "ids": [item[0] for item in taken]})
                free_at = now + case["send_duration_ms"]
                continue
            if plan.action == PLAN_WAIT:
                next_decision = now + plan.wait_ms
        if now < free_at:
            next_decision = free_at
        next_arrival = arrivals[pushed] if pushed < len(arrivals) else None
        candidates = [value for value in (next_decision, next_arrival) if value is not None]
        if not candidates:
            return batches
        now = min(candidates)


class SharedBatchingCasesTest(unittest.TestCase):
    def test_the_file_has_the_cases(self):
        self.assertTrue(CASES_FILE.is_file(), f"the file {CASES_FILE} does not exist")
        cases = json.loads(CASES_FILE.read_text(encoding="utf-8"))["cases"]
        self.assertEqual(len(cases), 39)
        self.assertEqual(len({case["name"] for case in cases}), 39)

    def test_every_shared_case_gives_the_expected_batches(self):
        for case in json.loads(CASES_FILE.read_text(encoding="utf-8"))["cases"]:
            with self.subTest(case=case["name"]):
                self.assertEqual(simulate(case), case["expected_batches"])


class PlanBatchTest(unittest.TestCase):
    def test_nothing_queued_is_idle_and_waits_for_ever(self):
        plan = plan_batch(1000, 0, 0, 100, 2000)
        self.assertEqual(plan, Plan(PLAN_IDLE, take=0, wait_ms=-1))

    def test_a_negative_count_is_idle_too(self):
        self.assertEqual(plan_batch(5, -3, 0, 100, 2000).action, PLAN_IDLE)

    def test_a_full_batch_leaves_at_once_even_when_the_oldest_is_new(self):
        self.assertEqual(plan_batch(10, 100, 10, 100, 2000), Plan(PLAN_SEND, take=100, wait_ms=0))

    def test_more_than_a_batch_takes_exactly_a_batch(self):
        self.assertEqual(plan_batch(10, 250, 10, 100, 2000), Plan(PLAN_SEND, take=100, wait_ms=0))

    def test_a_short_queue_waits_for_what_is_left_of_the_interval(self):
        self.assertEqual(plan_batch(500, 5, 100, 100, 2000), Plan(PLAN_WAIT, take=0, wait_ms=1600))

    def test_the_interval_reached_exactly_sends_what_is_queued(self):
        self.assertEqual(plan_batch(2100, 5, 100, 100, 2000), Plan(PLAN_SEND, take=5, wait_ms=0))

    def test_one_millisecond_before_the_interval_still_waits_one_millisecond(self):
        self.assertEqual(plan_batch(2099, 5, 100, 100, 2000), Plan(PLAN_WAIT, wait_ms=1))

    def test_a_batch_of_one_leaves_at_once(self):
        self.assertEqual(plan_batch(0, 1, 0, 1, 2000), Plan(PLAN_SEND, take=1, wait_ms=0))

    def test_a_very_old_event_sends_what_is_queued(self):
        self.assertEqual(plan_batch(10_000_000, 3, 0, 100, 2000), Plan(PLAN_SEND, take=3, wait_ms=0))

    def test_a_plan_is_a_frozen_value(self):
        plan = plan_batch(0, 1, 0, 1, 1)
        with self.assertRaises(dataclasses.FrozenInstanceError):
            plan.take = 5  # type: ignore[misc]

    def test_take_never_exceeds_the_batch_size_or_the_queue(self):
        generator = random.Random(20261103)
        for _ in range(5000):
            batch_size = generator.randint(1, 50)
            queued = generator.randint(0, 120)
            flush = generator.randint(1, 3000)
            oldest = generator.randint(0, 5000)
            now = oldest + generator.randint(0, 6000)
            plan = plan_batch(now, queued, oldest, batch_size, flush)
            with self.subTest(now=now, queued=queued, oldest=oldest, batch_size=batch_size, flush=flush):
                if plan.action == PLAN_SEND:
                    self.assertGreaterEqual(plan.take, 1)
                    self.assertLessEqual(plan.take, batch_size)
                    self.assertLessEqual(plan.take, queued)
                elif plan.action == PLAN_WAIT:
                    self.assertGreaterEqual(plan.wait_ms, 1)
                    self.assertLessEqual(plan.wait_ms, flush)
                    self.assertLess(queued, batch_size)
                else:
                    self.assertEqual((plan.action, queued, plan.wait_ms), (PLAN_IDLE, 0, -1))


if __name__ == "__main__":
    unittest.main()
