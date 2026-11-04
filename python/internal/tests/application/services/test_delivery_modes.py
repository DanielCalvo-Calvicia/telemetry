"""Black-box tests of the delivery modes of DeliveryService (T-05.3, REQ-DLV-003): best_effort and retry.

The ten cases of ``integration-tests/fixtures/delivery/mode_scenarios.json`` run through a service with a scripted
destination, a fixed random source and delays of a few milliseconds; the other tests are the edges: a stop during the
wait of a retry, a flush that waits for it, a retrying destination that never delays another one and events that keep
entering the queue while a batch is retried. The time is real and short, and a worker waits on a condition, so a test
waits for the asynchronous fact with ``eventually`` and never for a fixed time.
"""

import json
import pathlib
import time
import unittest
from typing import Any, Optional, Sequence

from internal.tests.application.services.test_delivery_service import Item, ItemDeliveryService, received
from internal.tests.eventually import eventually
from internal.tests.fakes import FakeLogSink, FakeRandom
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.services.delivery_service import (
    REASON_RETRY_LIMIT_REACHED,
    REASON_SHUTDOWN,
    REASON_SINK_PANICKED,
    DeliveryService,
    Destinations,
    DestinationStats,
)
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode
from telemetry.internal.infrastructure.outbound.shared.system_clock.system_clock import SystemClock

FIXTURE = pathlib.Path(__file__).resolve().parents[5] / "integration-tests" / "fixtures" / "delivery"
LONG_DELAY_MS = 60_000
FAILED_503 = SendResult(accepted=0, dropped=0, failed=3, reason="status_503")
ACCEPTED_3 = SendResult(accepted=3, dropped=0, failed=0)


class FailsOnceSink:
    """A sink whose first call fails in a transient way and whose next calls accept the whole batch."""

    def __init__(self) -> None:
        self.sizes: list[int] = []

    def send(self, events: Sequence[Any]) -> SendResult:
        self.sizes.append(len(events))
        if len(self.sizes) == 1:
            return FAILED_503
        return SendResult(accepted=len(events), dropped=0, failed=0)


class ScriptedSink:
    """A sink that answers with the scripted results in order, and the last one again when they are used up.

    It records the size of each batch and the moment of each call (``time.monotonic``, the clock the worker waits by).
    """

    def __init__(self, answers: Sequence[SendResult]) -> None:
        self.answers = tuple(answers)
        self.sizes: list[int] = []
        self.moments: list[float] = []

    @property
    def calls(self) -> int:
        return len(self.sizes)

    def send(self, events: Sequence[Any]) -> SendResult:
        self.moments.append(time.monotonic())
        self.sizes.append(len(events))
        return self.answers[min(len(self.sizes), len(self.answers)) - 1]


def load_cases() -> list[dict[str, Any]]:
    cases = json.loads((FIXTURE / "mode_scenarios.json").read_text(encoding="utf-8"))["cases"]
    if len(cases) < 10:
        raise AssertionError(f"mode_scenarios.json: expected 10 cases, found {len(cases)}")
    return cases


def one_stats(service: DeliveryService, index: int = 0) -> DestinationStats:
    return service.stats()[index]


class ModeTestCase(unittest.TestCase):
    def make(
        self,
        sinks: Sequence[Any],
        mode: DeliveryMode = DeliveryMode.RETRY,
        retry_max_attempts: int = 5,
        retry_base_delay_milliseconds: int = 1,
        retry_max_delay_milliseconds: int = 5,
        batch_size: int = 3,
        flush_interval_milliseconds: int = LONG_DELAY_MS,
        random: Optional[FakeRandom] = None,
    ) -> DeliveryService:
        """A started service over the sinks; the sinks here never block, so a plain stop at the end is enough."""
        settings = DeliverySettings(
            mode=mode,
            queue_size=1000,
            batch_size=batch_size,
            flush_interval_milliseconds=flush_interval_milliseconds,
            retry_max_attempts=retry_max_attempts,
            retry_base_delay_milliseconds=retry_base_delay_milliseconds,
            retry_max_delay_milliseconds=retry_max_delay_milliseconds,
        )
        service = ItemDeliveryService(
            Destinations(log=tuple(sinks)),
            settings,
            SystemClock(),
            new_memory_queue,
            random if random is not None else FakeRandom((0.5,)),
        )
        self.addCleanup(service.stop)
        service.start()
        return service

    def wait_for_batches(self, service: DeliveryService, count: int = 1, index: int = 0) -> DestinationStats:
        eventually(5.0, lambda: one_stats(service, index).batches >= count, f"{count} batches ended")
        return one_stats(service, index)


class SharedScenariosTest(ModeTestCase):
    def test_every_scenario_of_the_fixture(self):
        for case in load_cases():
            with self.subTest(case=case["name"]):
                self.run_case(case)

    def run_case(self, case: dict[str, Any]) -> None:
        sink = ScriptedSink([SendResult(**answer) for answer in case["answers"]])
        service = self.make([sink], mode=DeliveryMode(case["mode"]), retry_max_attempts=case["retry_max_attempts"])
        try:
            for number in range(case["events"]):
                service.enqueue("log", f"event-{number}")
            stats = self.wait_for_batches(service)
            expected = case["expected"]
            self.assertEqual(
                {
                    "attempts": stats.attempts,
                    "retries": stats.retries,
                    "delivered": stats.delivered,
                    "dropped": stats.dropped,
                    "failed": stats.failed,
                    "dropped_by_reason": dict(stats.dropped_by_reason),
                },
                {key: value for key, value in expected.items() if key != "min_elapsed_ms"},
            )
            self.assertEqual(sink.calls, expected["attempts"])
            self.assertEqual(sink.sizes, [case["events"]] * expected["attempts"], "the WHOLE batch is sent again")
            self.assertEqual(stats.delivered + stats.dropped + stats.failed, case["events"], "each event counted once")
            if "min_elapsed_ms" in expected:
                elapsed_ms = (sink.moments[-1] - sink.moments[0]) * 1000
                self.assertGreaterEqual(elapsed_ms, expected["min_elapsed_ms"])
        finally:
            service.stop()

    def test_the_fixture_has_the_cases_that_the_rules_name(self):
        names = {case["name"] for case in load_cases()}
        self.assertIn("retry_gives_up_at_the_limit", names)
        self.assertIn("retry_honors_the_retry_after_hint", names)


class BestEffortTest(ModeTestCase):
    def test_a_stat_that_has_not_sent_anything_has_no_attempts(self):
        service = self.make([ScriptedSink([ACCEPTED_3])])
        stats = one_stats(service)
        self.assertEqual((stats.attempts, stats.retries, stats.delivered, stats.batches), (0, 0, 0, 0))

    def test_every_batch_is_sent_once_whatever_the_retry_settings(self):
        sink = ScriptedSink([FAILED_503])
        service = self.make([sink], mode=DeliveryMode.BEST_EFFORT, retry_max_attempts=9)
        for number in range(6):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service, 2)
        self.assertEqual((stats.attempts, stats.retries, stats.failed, stats.dropped), (2, 0, 6, 0))
        self.assertEqual(sink.sizes, [3, 3])

    def test_an_answer_that_drops_counts_under_its_reason_or_unspecified(self):
        for reason, counted in (("status_400", "status_400"), ("", "unspecified")):
            with self.subTest(reason=reason):
                sink = ScriptedSink([SendResult(accepted=0, dropped=3, failed=0, reason=reason)])
                service = self.make([sink], mode=DeliveryMode.BEST_EFFORT)
                for number in range(3):
                    service.enqueue("log", number)
                stats = self.wait_for_batches(service)
                self.assertEqual(dict(stats.dropped_by_reason), {counted: 3})

    def test_a_sink_that_raises_is_one_attempt_and_a_failed_batch(self):
        class Raising:
            def send(self, events: Sequence[Any]) -> SendResult:
                raise RuntimeError("the sink broke")

        service = self.make([Raising()], mode=DeliveryMode.BEST_EFFORT)
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.attempts, stats.failed, stats.last_reason), (1, 3, REASON_SINK_PANICKED))

    def test_the_durable_mode_is_not_a_retry_mode_in_the_service(self):
        sink = ScriptedSink([FAILED_503])
        service = self.make([sink], mode=DeliveryMode.DURABLE)
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.attempts, stats.failed), (1, 3))


class RetryTest(ModeTestCase):
    def test_the_delays_grow_by_the_backoff_with_the_random_of_the_source(self):
        sink = ScriptedSink([FAILED_503])
        # base 40, ceiling 1000, random 0.5: the waits are 30 ms and 60 ms (equal jitter of 40 and of 80).
        service = self.make(
            [sink],
            retry_max_attempts=3,
            retry_base_delay_milliseconds=40,
            retry_max_delay_milliseconds=1000,
            random=FakeRandom((0.5,)),
        )
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.attempts, stats.retries), (3, 2))
        gaps_ms = [(later - earlier) * 1000 for earlier, later in zip(sink.moments, sink.moments[1:])]
        self.assertGreaterEqual(gaps_ms[0], 30)
        self.assertGreaterEqual(gaps_ms[1], 60)

    def test_the_random_source_is_asked_once_per_failed_answer(self):
        random = FakeRandom((0.0, 1.0))
        service = self.make([ScriptedSink([FAILED_503])], retry_max_attempts=4, random=random)
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.attempts, random.calls), (4, 4))

    def test_a_random_source_that_raises_does_not_stop_the_worker(self):
        random = FakeRandom()
        random.fail_with(RuntimeError("no random"))
        sink = ScriptedSink([FAILED_503, ACCEPTED_3])
        service = self.make([sink], random=random)
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.attempts, stats.retries, stats.delivered), (2, 1, 3))

    def test_a_sink_that_raises_is_a_transient_failure_that_is_retried(self):
        class RaisingOnce:
            calls = 0

            def send(self, events: Sequence[Any]) -> SendResult:
                self.calls += 1
                if self.calls == 1:
                    raise RuntimeError("the sink broke")
                return SendResult(accepted=len(events), dropped=0, failed=0)

        sink = RaisingOnce()
        service = self.make([sink])
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.attempts, stats.retries, stats.delivered, stats.failed), (2, 1, 3, 0))

    def test_a_sink_that_always_raises_ends_at_the_limit(self):
        class Raising:
            def send(self, events: Sequence[Any]) -> SendResult:
                raise RuntimeError("the sink broke")

        service = self.make([Raising()], retry_max_attempts=2)
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.attempts, dict(stats.dropped_by_reason)), (2, {REASON_RETRY_LIMIT_REACHED: 3}))

    def test_at_the_limit_the_accepted_of_the_last_answer_are_delivered_and_the_failed_are_dropped(self):
        partial = SendResult(accepted=1, dropped=0, failed=2, reason="status_503")
        service = self.make([ScriptedSink([partial])], retry_max_attempts=2)
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.delivered, stats.dropped, stats.failed), (1, 2, 0))
        self.assertEqual(dict(stats.dropped_by_reason), {REASON_RETRY_LIMIT_REACHED: 2})
        self.assertEqual(stats.last_reason, REASON_RETRY_LIMIT_REACHED)

    def test_an_answer_with_dropped_and_failed_events_ends_the_batch(self):
        mixed = SendResult(accepted=0, dropped=2, failed=1, reason="status_400")
        sink = ScriptedSink([mixed, ACCEPTED_3])
        service = self.make([sink])
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual((stats.attempts, stats.failed, stats.dropped), (1, 1, 2))
        self.assertEqual(dict(stats.dropped_by_reason), {"status_400": 2})

    def test_a_dropped_answer_without_a_reason_counts_as_unspecified(self):
        service = self.make([ScriptedSink([SendResult(accepted=0, dropped=3, failed=0)])])
        for number in range(3):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service)
        self.assertEqual(dict(stats.dropped_by_reason), {"unspecified": 3})

    def test_a_small_retry_after_never_shortens_the_backoff(self):
        short = SendResult(accepted=0, dropped=0, failed=3, reason="status_429", retry_after_ms=1)
        sink = ScriptedSink([short, ACCEPTED_3])
        service = self.make(
            [sink], retry_base_delay_milliseconds=40, retry_max_delay_milliseconds=40, random=FakeRandom((1.0,))
        )
        for number in range(3):
            service.enqueue("log", number)
        self.wait_for_batches(service)
        self.assertGreaterEqual((sink.moments[1] - sink.moments[0]) * 1000, 40)

    def test_later_batches_are_new_attempts_from_the_first(self):
        sink = ScriptedSink([FAILED_503, ACCEPTED_3, FAILED_503, ACCEPTED_3])
        service = self.make([sink])
        for number in range(6):
            service.enqueue("log", number)
        stats = self.wait_for_batches(service, 2)
        self.assertEqual((stats.attempts, stats.retries, stats.delivered, stats.batches), (4, 2, 6, 2))


class WaitTest(ModeTestCase):
    def test_a_stop_during_the_wait_returns_at_once_and_drops_the_batch_as_shutdown(self):
        sink = ScriptedSink([FAILED_503])
        service = self.make(
            [sink],
            retry_base_delay_milliseconds=LONG_DELAY_MS,
            retry_max_delay_milliseconds=LONG_DELAY_MS,
        )
        for number in range(3):
            service.enqueue("log", number)
        eventually(5.0, lambda: one_stats(service).attempts == 1, "the first attempt")
        started = time.monotonic()
        service.stop()
        self.assertLess(time.monotonic() - started, 2.0)
        stats = one_stats(service)
        self.assertEqual((stats.attempts, stats.batches, dict(stats.dropped_by_reason)), (1, 1, {REASON_SHUTDOWN: 3}))
        self.assertEqual((stats.dropped, stats.delivered, stats.failed, stats.last_reason), (3, 0, 0, REASON_SHUTDOWN))

    def test_shutdown_during_a_wait_returns_within_its_timeout_and_counts_the_batch_as_dropped(self):
        sink = ScriptedSink([FAILED_503])
        service = self.make(
            [sink],
            retry_base_delay_milliseconds=LONG_DELAY_MS,
            retry_max_delay_milliseconds=LONG_DELAY_MS,
        )
        for number in range(3):
            service.enqueue("log", number)
        eventually(5.0, lambda: one_stats(service).attempts == 1, "the first attempt")
        started = time.monotonic()
        result = service.shutdown(300)
        elapsed = time.monotonic() - started
        self.assertGreaterEqual(elapsed, 0.25)
        self.assertLess(elapsed, 2.0)
        self.assertEqual((result.sent, result.spooled, result.dropped), (0, 0, 3))
        eventually(5.0, lambda: dict(one_stats(service).dropped_by_reason) == {REASON_SHUTDOWN: 3}, "the batch dropped")
        self.assertEqual(sink.calls, 1, "no second attempt after the stop")

    def test_the_wait_is_a_wait_on_a_condition_and_not_a_busy_loop(self):
        service = self.make(
            [ScriptedSink([FAILED_503])],
            retry_base_delay_milliseconds=LONG_DELAY_MS,
            retry_max_delay_milliseconds=LONG_DELAY_MS,
        )
        for number in range(3):
            service.enqueue("log", number)
        eventually(5.0, lambda: one_stats(service).attempts == 1, "the first attempt")
        cpu_before = time.process_time()
        time.sleep(0.6)
        self.assertLess(time.process_time() - cpu_before, 0.3)

    def test_a_flush_waits_for_the_retries_of_the_batch_within_its_timeout(self):
        sink = ScriptedSink([FAILED_503, FAILED_503, ACCEPTED_3])
        service = self.make(
            [sink],
            batch_size=100,
            flush_interval_milliseconds=LONG_DELAY_MS,
            retry_base_delay_milliseconds=40,
            retry_max_delay_milliseconds=40,
        )
        for number in range(3):
            service.enqueue("log", number)
        result = service.flush(5000)
        self.assertEqual((result.sent, result.dropped), (3, 0))
        self.assertEqual(one_stats(service).retries, 2)

    def test_a_flush_that_times_out_in_the_wait_counts_the_batch_as_dropped_and_the_service_goes_on(self):
        sink = ScriptedSink([FAILED_503, ACCEPTED_3])
        service = self.make(
            [sink],
            batch_size=100,
            flush_interval_milliseconds=LONG_DELAY_MS,
            retry_base_delay_milliseconds=600,
            retry_max_delay_milliseconds=600,
            random=FakeRandom((1.0,)),
        )
        for number in range(3):
            service.enqueue("log", number)
        started = time.monotonic()
        result = service.flush(150)
        self.assertLess(time.monotonic() - started, 0.5)
        self.assertEqual((result.sent, result.dropped), (0, 3))
        self.assertEqual(one_stats(service).delivered, 0)
        eventually(5.0, lambda: one_stats(service).delivered == 3, "the retry after the flush")


class IndependenceTest(ModeTestCase):
    def test_a_destination_that_retries_never_delays_another_one(self):
        failing = ScriptedSink([FAILED_503])
        healthy = FakeLogSink()
        service = self.make(
            [failing, healthy],
            retry_base_delay_milliseconds=LONG_DELAY_MS,
            retry_max_delay_milliseconds=LONG_DELAY_MS,
        )
        service.enqueue("log", "first")
        service.enqueue("log", "second")
        service.enqueue("log", "third")
        started = time.monotonic()
        eventually(5.0, lambda: len(received(healthy)) == 3, "the healthy destination to deliver")
        self.assertLess(time.monotonic() - started, 2.0)
        eventually(5.0, lambda: one_stats(service, 0).attempts == 1, "the first attempt of the failing one")
        self.assertEqual((one_stats(service, 1).delivered, one_stats(service, 1).retries), (3, 0))
        self.assertEqual((one_stats(service, 0).delivered, one_stats(service, 0).batches), (0, 0))

    def test_new_events_keep_entering_the_queue_while_a_batch_is_retried(self):
        sink = FailsOnceSink()
        service = self.make(
            [sink],
            retry_base_delay_milliseconds=400,
            retry_max_delay_milliseconds=400,
            random=FakeRandom((1.0,)),
        )
        for number in range(3):
            service.enqueue("log", number)
        eventually(5.0, lambda: one_stats(service).attempts == 1, "the first attempt")
        results = [service.enqueue("log", Item(number)) for number in range(10, 15)]
        self.assertTrue(all(result.accepted == 1 and result.dropped == 0 for result in results))
        waiting = one_stats(service)
        self.assertEqual((waiting.queued, waiting.accepted, waiting.batches), (5, 8, 0))
        result = service.flush(5000)
        self.assertEqual((result.sent, result.dropped), (8, 0))
        self.assertEqual((one_stats(service).delivered, one_stats(service).retries), (8, 1))
        self.assertEqual(sink.sizes[:2], [3, 3])


if __name__ == "__main__":
    unittest.main()
