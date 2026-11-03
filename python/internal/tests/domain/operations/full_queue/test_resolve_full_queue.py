"""Tests of the rule of a full queue: the shared cases of integration-tests/fixtures/delivery and edge cases."""

import json
import pathlib
import unittest

from telemetry.internal.domain.operations.full_queue.resolve_full_queue import (
    NO_INDEX,
    REASON_BLOCK_TIMEOUT,
    REASON_DISPLACED_BY_CRITICAL,
    REASON_QUEUE_FULL_NEWEST,
    REASON_QUEUE_FULL_OLDEST,
    RESOLUTION_DISPLACE,
    RESOLUTION_DROP_NEW,
    RESOLUTION_WAIT,
    Resolution,
    resolve_full_queue,
)
from telemetry.internal.domain.value_objects.enums_generated import EventClass, QueuePolicy

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
CASES_PATH = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "delivery" / "full_queue_cases.json"

NEW_ID = 100
BULK, NORMAL, CRITICAL = EventClass.BULK, EventClass.NORMAL, EventClass.CRITICAL


def apply_resolution(queue: list[dict], new: dict, resolution: Resolution) -> tuple[list[int], list[dict]]:
    """Apply the answer to a list that models the queue; return the ids after and the removed (id and reason)."""
    ids = [item["id"] for item in queue]
    removed: list[dict] = []
    if resolution.outcome == RESOLUTION_DISPLACE:
        removed.append({"id": ids.pop(resolution.index), "reason": resolution.reason})
        ids.append(new["id"])
    return ids, removed


class SharedFullQueueCasesTest(unittest.TestCase):
    def test_every_shared_case_gives_the_expected_resolution_and_queue(self):
        self.assertTrue(CASES_PATH.is_file(), f"the file {CASES_PATH} does not exist")
        cases = json.loads(CASES_PATH.read_text(encoding="utf-8"))["cases"]
        self.assertGreaterEqual(len(cases), 50)
        for case in cases:
            with self.subTest(case=case["name"]):
                expected = case["expected"]
                new = case["new"]
                self.assertEqual(new["id"], NEW_ID)
                resolution = resolve_full_queue(
                    QueuePolicy(case["policy"]),
                    EventClass(new["class"]),
                    [EventClass(item["class"]) for item in case["queue"]],
                )
                self.assertEqual(resolution.outcome, expected["outcome"])
                self.assertEqual(resolution.reason, expected["reason"])
                ids, removed = apply_resolution(case["queue"], new, resolution)
                self.assertEqual(ids, expected["queue_after"])
                self.assertEqual(removed, expected["removed"])
                if expected["outcome"] != "displace":  # nothing changes now: the new event is dropped or waits
                    self.assertEqual(resolution.index, NO_INDEX)

    def test_the_fixture_covers_every_policy_and_every_outcome_and_reason(self):
        cases = json.loads(CASES_PATH.read_text(encoding="utf-8"))["cases"]
        self.assertEqual({case["policy"] for case in cases}, {policy.value for policy in QueuePolicy})
        self.assertEqual({case["expected"]["outcome"] for case in cases}, {"drop_new", "displace", "wait"})
        self.assertEqual(
            {case["expected"]["reason"] for case in cases},
            {"queue_full_newest", "queue_full_oldest", "displaced_by_critical", "block_timeout"},
        )


class ResolveFullQueueTest(unittest.TestCase):
    def test_the_outcomes_and_reasons_are_the_texts_of_the_contract(self):
        self.assertEqual((RESOLUTION_DROP_NEW, RESOLUTION_DISPLACE, RESOLUTION_WAIT), ("drop_new", "displace", "wait"))
        self.assertEqual(
            (REASON_QUEUE_FULL_NEWEST, REASON_QUEUE_FULL_OLDEST, REASON_DISPLACED_BY_CRITICAL, REASON_BLOCK_TIMEOUT),
            ("queue_full_newest", "queue_full_oldest", "displaced_by_critical", "block_timeout"),
        )

    def test_drop_newest_drops_the_new_event_whatever_its_class_when_nothing_is_lower_for_a_critical(self):
        for new_class in (BULK, NORMAL, CRITICAL):
            with self.subTest(new_class=new_class):
                resolution = resolve_full_queue(QueuePolicy.DROP_NEWEST, new_class, [CRITICAL, CRITICAL])
                self.assertEqual(resolution, Resolution(RESOLUTION_DROP_NEW, NO_INDEX, REASON_QUEUE_FULL_NEWEST))

    def test_a_critical_event_displaces_the_oldest_bulk_before_any_normal_under_every_policy(self):
        for policy in QueuePolicy:
            with self.subTest(policy=policy):
                resolution = resolve_full_queue(policy, CRITICAL, [NORMAL, CRITICAL, BULK, BULK])
                self.assertEqual(resolution, Resolution(RESOLUTION_DISPLACE, 2, REASON_DISPLACED_BY_CRITICAL))

    def test_a_critical_event_displaces_the_oldest_normal_when_there_is_no_bulk(self):
        resolution = resolve_full_queue(QueuePolicy.DROP_NEWEST, CRITICAL, [CRITICAL, NORMAL, NORMAL])
        self.assertEqual(resolution, Resolution(RESOLUTION_DISPLACE, 1, REASON_DISPLACED_BY_CRITICAL))

    def test_drop_oldest_takes_the_oldest_event_of_a_class_not_higher_than_the_new_one(self):
        self.assertEqual(
            resolve_full_queue(QueuePolicy.DROP_OLDEST, NORMAL, [CRITICAL, NORMAL, BULK]),
            Resolution(RESOLUTION_DISPLACE, 1, REASON_QUEUE_FULL_OLDEST),
        )
        self.assertEqual(
            resolve_full_queue(QueuePolicy.DROP_OLDEST, BULK, [CRITICAL, NORMAL, BULK]),
            Resolution(RESOLUTION_DISPLACE, 2, REASON_QUEUE_FULL_OLDEST),
        )

    def test_drop_oldest_drops_the_new_event_when_every_queued_event_is_of_a_higher_class(self):
        self.assertEqual(
            resolve_full_queue(QueuePolicy.DROP_OLDEST, BULK, [NORMAL, CRITICAL]),
            Resolution(RESOLUTION_DROP_NEW, NO_INDEX, REASON_QUEUE_FULL_NEWEST),
        )

    def test_block_with_timeout_waits_and_names_the_reason_of_the_end_of_the_wait(self):
        self.assertEqual(
            resolve_full_queue(QueuePolicy.BLOCK_WITH_TIMEOUT, NORMAL, [NORMAL]),
            Resolution(RESOLUTION_WAIT, NO_INDEX, REASON_BLOCK_TIMEOUT),
        )

    def test_it_is_pure_the_queued_classes_are_not_changed(self):
        queued = [BULK, NORMAL]
        resolve_full_queue(QueuePolicy.DROP_OLDEST, CRITICAL, queued)
        self.assertEqual(queued, [BULK, NORMAL])

    def test_a_resolution_is_a_value(self):
        with self.assertRaises(Exception):
            Resolution(RESOLUTION_WAIT).outcome = "x"  # type: ignore[misc]


if __name__ == "__main__":
    unittest.main()
