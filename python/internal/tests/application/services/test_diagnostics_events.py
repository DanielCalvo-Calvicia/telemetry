"""Black-box tests of the diagnostics DeliveryService publishes: drops of a full queue, shutdown, panics and the rest.

The shared scenarios (answers of a destination) are in ``test_diagnostics_snapshot.py``. Here the other sources of a
diagnostic are checked one by one, and that a diagnostic holds a code, a position, a reason and a count and nothing
of an event.
"""

import json
import threading
import time
import unittest
from typing import Any, Sequence

from internal.tests.application.services.diagnostics_helpers import Collector, new_channel, new_service, triples
from internal.tests.application.services.test_delivery_modes import ScriptedSink
from internal.tests.application.services.test_delivery_service import Item
from internal.tests.eventually import eventually
from internal.tests.fakes import FakeLogSink
from telemetry.internal.application.dtos.channel_diagnostics import ChannelDiagnostics
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.services import delivery_service
from telemetry.internal.domain.value_objects.enums_generated import (
    DeliveryMode,
    DropReason,
    EventClass,
    QueuePolicy,
    Severity,
)

FAILED_3 = SendResult(accepted=0, dropped=0, failed=3, reason="status_503")
ALERT = "alert.raised"


class QueueDropsTest(unittest.TestCase):
    def test_a_full_queue_publishes_one_diagnostic_per_dropped_event(self):
        collector = Collector()
        channel = new_channel(self, [collector])
        service = new_service(self, [FakeLogSink()], channel, queue_size=2, start=False)
        channel.start()
        for number in range(5):
            service.enqueue("log", number)
        found = collector.wait_for(3)
        self.assertEqual(triples(found), [("events_dropped", "queue_full_newest", 1)] * 3)
        self.assertEqual([(event.kind, event.index) for event in found], [("log", 0)] * 3)
        self.assertTrue(all(event.at_ms > 0 for event in found))

    def test_drop_oldest_publishes_the_displaced_event(self):
        collector = Collector()
        channel = new_channel(self, [collector])
        service = new_service(
            self, [FakeLogSink()], channel, queue_size=1, queue_policy=QueuePolicy.DROP_OLDEST, start=False
        )
        channel.start()
        service.enqueue("log", 0)
        service.enqueue("log", 1)
        self.assertEqual(triples(collector.wait_for(1)), [("events_dropped", "queue_full_oldest", 1)])

    def test_a_critical_event_that_displaces_publishes_displaced_by_critical(self):
        collector = Collector()
        channel = new_channel(self, [collector])
        service = new_service(
            self, [FakeLogSink()], channel, queue_size=1, event_classes={ALERT: EventClass.CRITICAL}, start=False
        )
        channel.start()
        service.enqueue("log", Item(0, "log.entry.recorded", Severity.DEBUG))
        service.enqueue("log", Item(1, ALERT, Severity.INFO))
        self.assertEqual(triples(collector.wait_for(1)), [("events_dropped", "displaced_by_critical", 1)])
        self.assertEqual(service.diagnostics().destinations[0].accepted, 2)

    def test_an_event_rejected_because_the_service_is_closed_publishes_shutdown_for_each_destination(self):
        collector = Collector()
        service = new_service(self, [FakeLogSink(), FakeLogSink()], new_channel(self, [collector]))
        result = service.reject_closed("log")
        self.assertEqual((result.dropped, result.reason), (1, "shutdown"))
        found = collector.wait_for(2)
        self.assertEqual(triples(found), [("events_dropped", "shutdown", 1)] * 2)
        self.assertEqual([(event.kind, event.index) for event in found], [("log", 0), ("log", 1)])

    def test_after_the_shutdown_the_drops_are_counted_but_the_channel_is_closed(self):
        collector = Collector()
        service = new_service(self, [FakeLogSink()], new_channel(self, [collector]))
        service.shutdown(1000)
        service.enqueue("log", "late")
        self.assertEqual(service.diagnostics().destinations[0].dropped_by_reason, {"shutdown": 1})
        self.assertEqual(service.diagnostics().channel.published, 0)
        self.assertEqual(collector.events, [])

    def test_the_block_timeout_publishes_its_reason(self):
        collector = Collector()
        channel = new_channel(self, [collector])
        service = new_service(
            self,
            [FakeLogSink()],
            channel,
            queue_size=1,
            queue_policy=QueuePolicy.BLOCK_WITH_TIMEOUT,
            block_timeout_milliseconds=20,
            start=False,
        )
        channel.start()
        service.enqueue("log", 0)
        service.enqueue("log", 1)
        self.assertEqual(triples(collector.wait_for(1)), [("events_dropped", "block_timeout", 1)])


class BatchPublishedDiagnosticsTest(unittest.TestCase):
    def test_the_events_of_a_batch_abandoned_while_it_waited_for_a_retry_are_dropped_as_shutdown(self):
        collector = Collector()
        sink = ScriptedSink([FAILED_3])
        service = new_service(
            self,
            [sink],
            new_channel(self, [collector]),
            mode=DeliveryMode.RETRY,
            batch_size=3,
            retry_base_delay_milliseconds=60_000,
            retry_max_delay_milliseconds=60_000,
        )
        for number in range(3):
            service.enqueue("log", number)
        eventually(5.0, lambda: sink.calls == 1, "the first attempt")
        service.stop()
        found = collector.wait_for(2)
        self.assertEqual(triples(found), [("export_failed", "status_503", 3), ("events_dropped", "shutdown", 3)])

    def test_a_sink_that_raises_publishes_export_failed_with_sink_panicked(self):
        class Raising:
            def send(self, events: Sequence[Any]) -> SendResult:
                raise RuntimeError("secret value inside the exception")

        collector = Collector()
        service = new_service(self, [Raising()], new_channel(self, [collector]))
        for number in range(2):
            service.enqueue("log", number)
        service.flush(5000)
        found = collector.wait_for(1)
        self.assertEqual(triples(found), [("export_failed", "sink_panicked", 2)])
        self.assertNotIn("secret", json.dumps(service.diagnostics().destinations[0].last_reason))

    def test_a_channel_that_raises_never_fails_the_delivery(self):
        class BrokenChannel:
            def publish(self, event: Any) -> None:
                raise RuntimeError("broken")

            def start(self) -> None:
                return None

            def stop(self) -> None:
                return None

            def counters(self) -> Any:
                raise AssertionError("not used")

        sink = ScriptedSink([FAILED_3])
        service = new_service(self, [sink], BrokenChannel())  # type: ignore[arg-type]
        for number in range(3):
            service.enqueue("log", number)
        result = service.flush(5000)
        self.assertEqual((result.sent, result.dropped), (0, 3))
        self.assertEqual(service.stats()[0].failed, 3)

    def test_the_diagnostics_do_not_pass_through_the_destinations(self):
        sink = ScriptedSink([FAILED_3])
        collector = Collector()
        service = new_service(self, [sink], new_channel(self, [collector]))
        for number in range(3):
            service.enqueue("log", number)
        service.flush(5000)
        collector.wait_for(1)
        self.assertEqual(sink.sizes, [3], "the sink received the three events and nothing else")


class PassivityTest(unittest.TestCase):
    def test_constructing_the_service_starts_nothing_and_start_and_stop_drive_the_dispatcher(self):
        baseline = set(threading.enumerate())

        def mine() -> list[threading.Thread]:
            return [thread for thread in threading.enumerate() if thread not in baseline and thread.is_alive()]

        service = new_service(self, [FakeLogSink()], new_channel(self, [Collector()]), start=False)
        self.assertEqual(mine(), [])
        service.start()
        names = sorted(thread.name for thread in mine())
        self.assertEqual(names, ["telemetry-delivery-log-0", "telemetry-diagnostics"])
        service.stop()
        eventually(5.0, lambda: mine() == [], "every thread to end")

    def test_the_dispatcher_ends_at_the_shutdown(self):
        baseline = set(threading.enumerate())
        service = new_service(self, [FakeLogSink()], new_channel(self, [Collector()]))
        service.shutdown(1000)
        eventually(5.0, lambda: [t for t in threading.enumerate() if t not in baseline and t.is_alive()] == [])

    def test_a_service_without_a_channel_starts_and_stops(self):
        service = new_service(self, [FakeLogSink()], channel=None)
        service.stop()
        self.assertEqual(service.diagnostics().channel, ChannelDiagnostics())


class ReasonsTest(unittest.TestCase):
    def test_the_drop_reasons_of_the_service_are_the_values_of_the_generated_enum(self):
        enum_values = {reason.value for reason in DropReason}
        names = [name for name in delivery_service.__all__ if name.startswith("REASON_")]
        constants = {getattr(delivery_service, name) for name in names}
        extra = {delivery_service.REASON_SERIALIZATION_FAILED, delivery_service.REASON_CONFIGURATION_ERROR}
        self.assertEqual(constants - extra, enum_values)

    def test_a_reason_with_a_space_and_a_url_is_sanitized(self):
        collector = Collector()
        answer = SendResult(accepted=0, dropped=0, failed=1, reason="bad token abc://host/path?key=SECRET " + "x" * 100)
        service = new_service(self, [ScriptedSink([answer])], new_channel(self, [collector]))
        service.enqueue("log", 0)
        service.flush(5000)
        reason = collector.wait_for(1)[0].reason
        self.assertRegex(reason, r"^[A-Za-z0-9_.:-]{1,64}$")
        self.assertEqual(len(reason), 64)
        self.assertTrue(reason.startswith("bad_token_abc:__host_path_key_SECRET_"))


class TimingTest(unittest.TestCase):
    def test_publishing_does_not_wait_for_a_slow_callback(self):
        def slow(event: Any) -> None:
            time.sleep(0.3)

        sink = ScriptedSink([FAILED_3])
        service = new_service(self, [sink], new_channel(self, [slow]), batch_size=1)
        started = time.monotonic()
        for number in range(5):
            service.enqueue("log", number)
        service.flush(5000)
        self.assertLess(time.monotonic() - started, 1.2)


if __name__ == "__main__":
    unittest.main()
