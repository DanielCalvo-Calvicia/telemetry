"""Black-box tests of the snapshot of the diagnostics of DeliveryService and of the diagnostics of the shared scenarios.

The 40 cases of ``integration-tests/fixtures/delivery/diagnostics_snapshot.json`` run through a real service with a
scripted destination (batch size 10, flush interval 60 s, backoff 1/5 ms, random 0.5): the events are queued, ONE
flush sends them and the counters and the diagnostics are compared. The other tests are the edges of the snapshot:
it is a copy, its queue figures, the spool figures, the last flush and the latencies.
"""

import dataclasses
import json
import pathlib
import unittest
from typing import Any, Sequence

from internal.tests.application.services.diagnostics_helpers import Collector, new_channel, new_service, triples
from internal.tests.application.services.test_delivery_modes import ScriptedSink
from internal.tests.eventually import eventually
from telemetry.internal.application.dtos.destination_diagnostics import DestinationDiagnostics
from telemetry.internal.application.dtos.diagnostics_snapshot import DiagnosticsSnapshot
from telemetry.internal.application.dtos.flush_outcome import FlushOutcome
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.services.delivery_service import (
    REASON_SHUTDOWN,
    REASON_SINK_PANICKED,
    DeliveryService,
)
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode, DiagnosticCode

FIXTURE = pathlib.Path(__file__).resolve().parents[5] / "integration-tests" / "fixtures" / "delivery"
ACCEPTED_3 = SendResult(accepted=3, dropped=0, failed=0)
FAILED_3 = SendResult(accepted=0, dropped=0, failed=3, reason="timeout")


def load_cases() -> list[dict[str, Any]]:
    cases = json.loads((FIXTURE / "diagnostics_snapshot.json").read_text(encoding="utf-8"))["cases"]
    if len(cases) != 40:
        raise AssertionError(f"diagnostics_snapshot.json: expected 40 cases, found {len(cases)}")
    return cases


def only(service: DeliveryService) -> DestinationDiagnostics:
    return service.diagnostics().destinations[0]


class SharedScenariosTest(unittest.TestCase):
    def test_every_scenario_of_the_fixture(self):
        for case in load_cases():
            with self.subTest(case=case["name"]):
                self.run_case(case)

    def run_case(self, case: dict[str, Any]) -> None:
        collector = Collector()
        channel = new_channel(self, [collector])
        sink = ScriptedSink([SendResult(**answer) for answer in case["answers"]])
        service = new_service(
            self,
            [sink],
            channel,
            mode=DeliveryMode(case["mode"]),
            retry_max_attempts=case["retry_max_attempts"],
        )
        for number in range(case["events"]):
            service.enqueue("log", number)
        result = service.flush(5000)
        expected = case["expected"]
        self.assertEqual(
            (result.sent, result.spooled, result.dropped),
            (expected["last_flush"]["sent"], 0, expected["last_flush"]["dropped"]),
        )
        eventually(5.0, lambda: only(service).batches == 1, "the batch ended")
        found = only(service)
        counters = expected["counters"]
        self.assertEqual(
            {
                "attempts": found.attempts,
                "retries": found.retries,
                "batches": found.batches,
                "delivered": found.delivered,
                "dropped": found.dropped,
                "failed_events": found.failed_events,
                "export_failures": found.export_failures,
                "serialization_failures": found.serialization_failures,
                "configuration_errors": found.configuration_errors,
                "last_batch_size": found.last_batch_size,
                "max_batch_size": found.max_batch_size,
                "dropped_by_reason": dict(found.dropped_by_reason),
            },
            counters,
        )
        self.assertEqual(found.last_success_at_ms > 0, expected["has_last_success"])
        self.assertEqual(
            (found.last_flush.done, found.last_flush.sent, found.last_flush.spooled, found.last_flush.dropped),
            (
                True,
                expected["last_flush"]["sent"],
                expected["last_flush"]["spooled"],
                expected["last_flush"]["dropped"],
            ),
        )
        self.assertGreater(found.last_flush.at_ms, 0)
        self.assertEqual(found.accepted, case["events"])
        self.assertEqual((found.spool_events, found.spool_bytes, found.spool_quota_bytes), (0, 0, 0))
        self.assertGreaterEqual(found.last_export_latency_ms, 0)
        self.assertGreaterEqual(found.max_export_latency_ms, found.last_export_latency_ms)
        self.assertGreaterEqual(found.total_export_latency_ms, found.max_export_latency_ms)
        collected = collector.wait_for(len(expected["diagnostics"]))
        self.assertEqual(
            triples(collected),
            [(item["code"], item["reason"], item["count"]) for item in expected["diagnostics"]],
        )
        self.assertEqual([event.suppressed for event in collected], [0] * len(collected))
        self.assertEqual({(event.kind, event.index) for event in collected} - {("log", 0)}, set())
        eventually(5.0, lambda: service.diagnostics().channel.published == len(collected), "the channel counted them")
        self.assertEqual(service.diagnostics().channel.suppressed, 0)

    def test_the_fixture_has_the_cases_that_the_rules_name(self):
        names = {case["name"] for case in load_cases()}
        self.assertIn("retry_limit_reached", names)
        self.assertIn("best_effort_serialization_failed", names)


class SnapshotShapeTest(unittest.TestCase):
    def test_a_service_that_never_sent_has_zero_figures_and_the_queue_figures(self):
        service = new_service(self, [ScriptedSink([ACCEPTED_3])], queue_size=5, start=False)
        for number in range(3):
            service.enqueue("log", number)
        found = only(service)
        self.assertEqual((found.kind, found.index, found.queue_depth, found.queue_capacity), ("log", 0, 3, 5))
        self.assertEqual((found.accepted, found.delivered, found.dropped, found.batches), (3, 0, 0, 0))
        self.assertEqual((found.attempts, found.retries, found.export_failures, found.last_success_at_ms), (0, 0, 0, 0))
        self.assertEqual(found.last_flush, FlushOutcome())
        self.assertEqual(found.last_reason, "")
        self.assertEqual((found.spool_events, found.spool_bytes, found.spool_quota_bytes), (0, 0, 0))

    def test_accepted_means_queued_and_delivered_is_what_the_destination_took(self):
        service = new_service(self, [ScriptedSink([ACCEPTED_3])])
        for number in range(3):
            service.enqueue("log", number)
        before = only(service)
        self.assertEqual((before.accepted, before.delivered), (3, 0))
        service.flush(5000)
        after = only(service)
        self.assertEqual((after.accepted, after.delivered, after.queue_depth), (3, 3, 0))

    def test_the_destinations_are_in_the_order_of_the_kinds_and_the_sinks(self):
        service = new_service(self, [ScriptedSink([ACCEPTED_3]), ScriptedSink([ACCEPTED_3])], start=False)
        found = service.diagnostics()
        self.assertIsInstance(found.destinations, tuple)
        self.assertEqual([(item.kind, item.index) for item in found.destinations], [("log", 0), ("log", 1)])

    def test_a_service_without_a_channel_has_zero_channel_counters_and_still_counts(self):
        service = new_service(self, [ScriptedSink([FAILED_3])], channel=None)
        for number in range(3):
            service.enqueue("log", number)
        service.flush(5000)
        found = service.diagnostics()
        self.assertEqual(dataclasses.astuple(found.channel), (0, 0, 0, 0))
        self.assertEqual(found.destinations[0].export_failures, 1)

    def test_a_service_without_destinations_has_an_empty_snapshot(self):
        service = new_service(self, [], start=False)
        self.assertEqual(service.diagnostics(), DiagnosticsSnapshot())

    def test_the_snapshot_is_a_copy(self):
        service = new_service(self, [ScriptedSink([SendResult(accepted=0, dropped=3, failed=0, reason="status_400")])])
        for number in range(3):
            service.enqueue("log", number)
        service.flush(5000)
        first = service.diagnostics()
        self.assertEqual(dict(first.destinations[0].dropped_by_reason), {"status_400": 3})
        first.destinations[0].dropped_by_reason["injected"] = 99  # type: ignore[index]
        first.destinations[0].dropped_by_reason["status_400"] = 0  # type: ignore[index]
        with self.assertRaises(dataclasses.FrozenInstanceError):
            first.destinations[0].dropped = 0  # type: ignore[misc]
        with self.assertRaises(TypeError):
            first.destinations[0] = DestinationDiagnostics("log", 1)  # type: ignore[index]
        second = service.diagnostics()
        self.assertEqual(dict(second.destinations[0].dropped_by_reason), {"status_400": 3})
        self.assertEqual(second.destinations[0].dropped, 3)

    def test_the_reasons_of_the_snapshot_are_sanitized_and_those_that_become_equal_add_up(self):
        answers = [
            SendResult(accepted=0, dropped=1, failed=0, reason="bad reason://x"),
            SendResult(accepted=0, dropped=2, failed=0, reason="bad reason:__x"),
        ]
        service = new_service(self, [ScriptedSink(answers)], batch_size=1)
        for number in range(2):
            service.enqueue("log", number)
        service.flush(5000)
        eventually(5.0, lambda: only(service).batches == 2, "two batches")
        found = only(service)
        self.assertEqual(dict(found.dropped_by_reason), {"bad_reason:__x": 3})
        self.assertEqual(found.last_reason, "bad_reason:__x")

    def test_a_snapshot_taken_before_is_not_changed_by_what_happens_after(self):
        service = new_service(self, [ScriptedSink([ACCEPTED_3])])
        before = service.diagnostics()
        for number in range(3):
            service.enqueue("log", number)
        service.flush(5000)
        self.assertEqual(before.destinations[0].accepted, 0)
        self.assertEqual(service.diagnostics().destinations[0].accepted, 3)

    def test_the_latency_of_a_slow_call_is_counted_in_whole_milliseconds(self):
        class SlowSink:
            def send(self, events: Sequence[Any]) -> SendResult:
                import time

                time.sleep(0.05)
                return SendResult(accepted=len(events), dropped=0, failed=0)

        service = new_service(self, [SlowSink()], batch_size=2)
        for number in range(4):
            service.enqueue("log", number)
        service.flush(5000)
        eventually(5.0, lambda: only(service).batches == 2, "two batches")
        found = only(service)
        self.assertGreaterEqual(found.last_export_latency_ms, 40)
        self.assertGreaterEqual(found.max_export_latency_ms, found.last_export_latency_ms)
        self.assertGreaterEqual(found.total_export_latency_ms, 80)
        self.assertEqual((found.last_batch_size, found.max_batch_size), (2, 2))

    def test_the_batch_sizes_are_the_last_and_the_largest(self):
        service = new_service(self, [ScriptedSink([ACCEPTED_3])], batch_size=3)
        for number in range(4):
            service.enqueue("log", number)
        service.flush(5000)
        eventually(5.0, lambda: only(service).batches == 2, "two batches")
        found = only(service)
        self.assertEqual((found.last_batch_size, found.max_batch_size), (1, 3))

    def test_a_sink_that_raises_is_an_export_failure_with_the_reason_sink_panicked(self):
        class Raising:
            def send(self, events: Sequence[Any]) -> SendResult:
                raise RuntimeError("the sink broke with a secret message")

        collector = Collector()
        service = new_service(self, [Raising()], new_channel(self, [collector]))
        for number in range(3):
            service.enqueue("log", number)
        service.flush(5000)
        found = only(service)
        self.assertEqual((found.export_failures, found.failed_events, found.last_reason), (1, 3, REASON_SINK_PANICKED))
        self.assertEqual(triples(collector.wait_for(1)), [("export_failed", "sink_panicked", 3)])


class LastFlushTest(unittest.TestCase):
    def test_every_destination_gets_the_outcome_of_the_flush(self):
        service = new_service(
            self, [ScriptedSink([FAILED_3]), ScriptedSink([ACCEPTED_3])], mode=DeliveryMode.BEST_EFFORT
        )
        for number in range(3):
            service.enqueue("log", number)
        result = service.flush(5000)
        self.assertEqual((result.sent, result.dropped), (3, 3))
        outcomes = [item.last_flush for item in service.diagnostics().destinations]
        self.assertTrue(all(outcome.done for outcome in outcomes))
        self.assertEqual(
            [(outcome.sent, outcome.dropped, outcome.spooled) for outcome in outcomes], [(0, 3, 0), (3, 0, 0)]
        )

    def test_a_flush_with_nothing_queued_is_done_with_zero(self):
        service = new_service(self, [ScriptedSink([ACCEPTED_3])])
        service.flush(1000)
        outcome = only(service).last_flush
        self.assertEqual((outcome.done, outcome.sent, outcome.dropped, outcome.spooled), (True, 0, 0, 0))
        self.assertGreater(outcome.at_ms, 0)

    def test_the_last_flush_is_the_one_that_ended_last(self):
        sink = ScriptedSink([ACCEPTED_3])
        service = new_service(self, [sink])
        for number in range(3):
            service.enqueue("log", number)
        service.flush(5000)
        self.assertEqual(only(service).last_flush.sent, 3)
        service.flush(1000)
        self.assertEqual(only(service).last_flush.sent, 0)

    def test_a_shutdown_sets_it_too_and_the_snapshot_still_answers_after_it(self):
        service = new_service(self, [ScriptedSink([ACCEPTED_3])])
        for number in range(3):
            service.enqueue("log", number)
        result = service.shutdown(5000)
        self.assertEqual(result.sent, 3)
        found = only(service)
        self.assertEqual((found.last_flush.done, found.last_flush.sent, found.last_flush.dropped), (True, 3, 0))
        service.enqueue("log", "late")
        self.assertEqual(only(service).dropped_by_reason, {REASON_SHUTDOWN: 1})

    def test_a_flush_that_times_out_counts_what_is_pending_as_dropped(self):
        service = new_service(self, [ScriptedSink([ACCEPTED_3])])
        for number in range(3):
            service.enqueue("log", number)
        service.flush(0)
        outcome = only(service).last_flush
        self.assertEqual((outcome.done, outcome.sent, outcome.dropped), (True, 0, 3))


class DiagnosticCodeTest(unittest.TestCase):
    def test_the_codes_of_the_generated_enum(self):
        self.assertEqual(
            {code.value for code in DiagnosticCode},
            {"export_failed", "events_dropped", "serialization_failed", "configuration_error"},
        )


if __name__ == "__main__":
    unittest.main()
