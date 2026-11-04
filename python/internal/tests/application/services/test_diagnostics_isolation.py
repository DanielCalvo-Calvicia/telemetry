"""Black-box tests that the diagnostics never disturb the delivery: callbacks that raise or hang, and a storm.

REQ-DGN-003: a failure of telemetry never creates an event that can fail again, so a callback that fails produces
nothing, one that hangs blocks only the thread of the channel, and the rate limit holds under a storm of failures.
"""

import threading
import time
import unittest
from typing import Any

from internal.tests.application.services.diagnostics_helpers import Collector, new_channel, new_service, triples
from internal.tests.application.services.test_delivery_modes import ScriptedSink
from internal.tests.eventually import eventually
from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.infrastructure.outbound.shared.diagnostics_channel.diagnostics_channel import THREAD_NAME

FAILED_1 = SendResult(accepted=0, dropped=0, failed=1, reason="status_503")
BATCHES = 20


class Raising:
    """A callback that always raises and counts its calls."""

    def __init__(self) -> None:
        self.calls = 0

    def __call__(self, event: DiagnosticEvent) -> None:
        self.calls += 1
        raise RuntimeError("secret inside the exception")


class Hanging:
    """A callback that blocks until released."""

    def __init__(self) -> None:
        self.release = threading.Event()
        self.entered = threading.Event()

    def __call__(self, event: DiagnosticEvent) -> None:
        self.entered.set()
        self.release.wait(30)


class MisbehavingCallbacksTest(unittest.TestCase):
    def test_a_callback_that_raises_does_not_touch_the_delivery_of_20_batches(self):
        raising = Raising()
        collector = Collector()
        sink = ScriptedSink([FAILED_1])
        service = new_service(self, [sink], new_channel(self, [raising, collector]), batch_size=1)
        for number in range(BATCHES):
            service.enqueue("log", number)
        result = service.flush(5000)
        self.assertEqual((result.sent, result.dropped), (0, BATCHES))
        self.assertEqual(sink.sizes, [1] * BATCHES, "the sink received the 20 events and not one more")
        collector.wait_for(BATCHES)
        eventually(5.0, lambda: service.diagnostics().channel.callback_errors == BATCHES, "the errors counted")
        self.assertEqual(raising.calls, BATCHES)
        self.assertEqual(service.diagnostics().destinations[0].export_failures, BATCHES)
        self.assertEqual(
            triples(collector.events), [("export_failed", "status_503", 1)] * BATCHES, "no diagnostic about the error"
        )

    def test_a_callback_that_hangs_affects_neither_the_delivery_nor_flush_nor_shutdown(self):
        baseline = set(threading.enumerate())
        hanging = Hanging()
        self.addCleanup(hanging.release.set)
        sink = ScriptedSink([FAILED_1])
        channel = new_channel(self, [hanging], queue_size=64)
        service = new_service(self, [sink], channel, batch_size=1, queue_size=2000)
        service.enqueue("log", 0)
        self.assertTrue(hanging.entered.wait(5), "the first diagnostic reaches the callback, which hangs")
        events = 150
        for number in range(1, events):
            service.enqueue("log", number)
        started = time.monotonic()
        flushed = service.flush(5000)
        self.assertLess(time.monotonic() - started, 4.0)
        self.assertEqual((flushed.sent, flushed.dropped), (0, events - 1))
        self.assertEqual(len(sink.sizes), events, "every event reached the sink")
        eventually(
            5.0, lambda: service.diagnostics().channel.published + service.diagnostics().channel.dropped == events
        )
        counters = service.diagnostics().channel
        self.assertEqual((counters.published, counters.dropped), (65, events - 65))
        started = time.monotonic()
        service.shutdown(1000)
        self.assertLess(time.monotonic() - started, 2.0)
        dispatchers = [
            thread for thread in threading.enumerate() if thread.name == THREAD_NAME and thread not in baseline
        ]
        self.assertEqual(len(dispatchers), 1, "the hanging dispatcher is abandoned, not waited for")
        hanging.release.set()
        eventually(5.0, lambda: not dispatchers[0].is_alive(), "the dispatcher to end once released")

    def test_a_writer_that_raises_does_not_touch_the_delivery(self):
        class Broken:
            def write(self, text: str) -> int:
                raise OSError("secret inside the exception")

        sink = ScriptedSink([FAILED_1])
        service = new_service(self, [sink], new_channel(self, writer=Broken()), batch_size=1)
        for number in range(BATCHES):
            service.enqueue("log", number)
        service.flush(5000)
        eventually(5.0, lambda: service.diagnostics().channel.callback_errors == BATCHES, "the errors counted")
        self.assertEqual(sink.sizes, [1] * BATCHES)


class StormTest(unittest.TestCase):
    def test_a_storm_of_1000_failing_batches_in_one_window_lets_10_reach_the_callback(self):
        class FakeMilliseconds:
            value = 0

            def __call__(self) -> int:
                return self.value

        clock = FakeMilliseconds()
        collector = Collector()
        channel = new_channel(self, [collector], read_monotonic_milliseconds=clock, limit=10)
        sink = ScriptedSink([FAILED_1])
        service = new_service(self, [sink], channel, batch_size=1, queue_size=2000)
        for number in range(1000):
            service.enqueue("log", number)
        service.flush(10_000)
        eventually(10.0, lambda: service.diagnostics().channel.suppressed == 990, "990 suppressed")
        collector.wait_for(10)
        counters = service.diagnostics().channel
        self.assertEqual((counters.published, counters.suppressed, counters.dropped), (10, 990, 0))
        self.assertEqual(len(collector.events), 10)
        clock.value = 1000
        service.enqueue("log", "after the window")
        service.flush(5000)
        eventually(5.0, lambda: len(collector.events) == 11, "the first diagnostic of the next window")
        self.assertEqual(collector.events[10].suppressed, 990)
        self.assertEqual(len(sink.sizes), 1001, "the storm never touched the delivery")


class ShapeOfDiagnosticsTest(unittest.TestCase):
    def test_a_diagnostic_is_not_passed_to_any_destination_and_has_only_counters_and_identifiers(self):
        collector = Collector()
        received: list[Any] = []

        class Recording(ScriptedSink):
            def send(self, events: Any) -> SendResult:
                received.extend(events)
                return super().send(events)

        service = new_service(self, [Recording([FAILED_1])], new_channel(self, [collector]), batch_size=1)
        service.enqueue("log", "payload")
        service.flush(5000)
        found = collector.wait_for(1)[0]
        self.assertEqual(len(received), 1)
        self.assertNotIn("payload", repr(found))
        self.assertEqual(sorted(found.__slots__), ["at_ms", "code", "count", "index", "kind", "reason", "suppressed"])


if __name__ == "__main__":
    unittest.main()
