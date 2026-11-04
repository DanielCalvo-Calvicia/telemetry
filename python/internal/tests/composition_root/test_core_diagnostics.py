"""Black-box tests of the diagnostics of the core built by ``telemetry.new`` (T-05.5, REQ-DGN-001 to REQ-DGN-003).

The options are ``with_diagnostics_callback`` (can be given several times) and ``with_diagnostics_writer``; the
snapshot is ``core.diagnostics()``. The time is real and the bounds are generous.
"""

import dataclasses
import inspect
import io
import json
import threading
import unittest
from typing import Any

import telemetry
from internal.tests.composition_root.test_core import build, config_for, log_event
from internal.tests.eventually import eventually
from internal.tests.fakes import FakeLogSink
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.composition_root.config.options import Options
from telemetry.internal.composition_root.containers.core_container import Core
from telemetry.internal.infrastructure.outbound.shared.diagnostics_channel.diagnostics_channel import THREAD_NAME

SECRET = "SECRET-VALUE-9f8e7d"
MESSAGE = "the message of the event that must never leave"


class FailingSink:
    def __init__(self, reason: str = "status_503") -> None:
        self.reason = reason
        self.batches: list = []

    def send(self, events: Any) -> SendResult:
        self.batches.append(list(events))
        return SendResult(accepted=0, dropped=0, failed=len(events), reason=self.reason)


def dispatchers() -> list[threading.Thread]:
    return [thread for thread in threading.enumerate() if thread.name == THREAD_NAME and thread.is_alive()]


class OptionsTest(unittest.TestCase):
    def test_a_callback_is_added_each_time_and_none_is_ignored(self):
        options = Options()
        first, second = (lambda event: None), (lambda event: None)
        for option in (
            telemetry.with_diagnostics_callback(first),
            telemetry.with_diagnostics_callback(None),
            telemetry.with_diagnostics_callback(second),
        ):
            option(options)
        self.assertEqual(options.diagnostics_callbacks, [first, second])

    def test_the_writer_is_the_last_one_given_and_none_is_ignored(self):
        options = Options()
        first, second = io.StringIO(), io.StringIO()
        for option in (
            telemetry.with_diagnostics_writer(first),
            telemetry.with_diagnostics_writer(None),
            telemetry.with_diagnostics_writer(second),
        ):
            option(options)
        self.assertIs(options.diagnostics_writer, second)
        self.assertEqual(Options().diagnostics_callbacks, [])
        self.assertIsNone(Options().diagnostics_writer)

    def test_the_names_are_public(self):
        for name in (
            "with_diagnostics_callback",
            "with_diagnostics_writer",
            "DiagnosticEvent",
            "DiagnosticCode",
            "DiagnosticsSnapshot",
            "DestinationDiagnostics",
            "ChannelDiagnostics",
            "FlushOutcome",
        ):
            with self.subTest(name=name):
                self.assertIn(name, telemetry.__all__)
                self.assertTrue(inspect.getdoc(getattr(telemetry, name)))
        self.assertTrue(inspect.getdoc(Core.diagnostics))


class SnapshotOfTheCoreTest(unittest.TestCase):
    def test_a_core_without_sinks_has_an_empty_snapshot(self):
        core = build(self, config_for())
        self.assertEqual(core.diagnostics(), telemetry.DiagnosticsSnapshot())

    def test_the_snapshot_shows_the_delivery_and_still_answers_after_the_shutdown(self):
        sink = FakeLogSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink))
        for number in range(3):
            core.emit(log_event(str(number)))
        core.flush(2000)
        found = core.diagnostics()
        self.assertIsInstance(found, telemetry.DiagnosticsSnapshot)
        destination = found.destinations[0]
        self.assertEqual((destination.kind, destination.index), ("log", 0))
        self.assertEqual((destination.accepted, destination.delivered, destination.dropped), (3, 3, 0))
        self.assertEqual((destination.queue_depth, destination.queue_capacity), (0, config_for().delivery.queue_size))
        self.assertTrue(destination.last_flush.done)
        self.assertGreater(destination.last_success_at_ms, 0)
        core.shutdown(1000)
        self.assertEqual(core.diagnostics().destinations[0].delivered, 3)

    def test_the_snapshot_is_a_copy(self):
        sink = FailingSink("status_400")
        sink.send = lambda events: SendResult(accepted=0, dropped=len(events), failed=0, reason="status_400")  # type: ignore[method-assign]
        core = build(self, config_for(), telemetry.with_log_sink(sink))
        core.emit(log_event())
        core.flush(2000)
        first = core.diagnostics()
        first.destinations[0].dropped_by_reason["status_400"] = 100  # type: ignore[index]
        self.assertEqual(dict(core.diagnostics().destinations[0].dropped_by_reason), {"status_400": 1})

    def test_diagnostics_never_raises_even_if_the_service_does(self):
        class Broken:
            def diagnostics(self) -> Any:
                raise RuntimeError("broken")

        core = Core(object(), object(), Broken())  # type: ignore[arg-type]
        self.assertEqual(core.diagnostics(), telemetry.DiagnosticsSnapshot())


class CallbacksOfTheCoreTest(unittest.TestCase):
    def test_a_failing_destination_reaches_the_callbacks_and_the_writer(self):
        received: list[telemetry.DiagnosticEvent] = []
        buffer = io.StringIO()
        sink = FailingSink("status_503")
        core = build(
            self,
            config_for(),
            telemetry.with_log_sink(sink),
            telemetry.with_diagnostics_callback(received.append),
            telemetry.with_diagnostics_writer(buffer),
        )
        core.emit(log_event())
        core.flush(2000)
        eventually(5.0, lambda: len(received) == 1 and buffer.getvalue() != "", "the diagnostic")
        event = received[0]
        self.assertEqual(
            (event.code, event.kind, event.index, event.reason, event.count, event.suppressed),
            (telemetry.DiagnosticCode.EXPORT_FAILED, "log", 0, "status_503", 1, 0),
        )
        self.assertGreater(event.at_ms, 0)
        self.assertEqual(
            buffer.getvalue(),
            "telemetry diagnostics: code=export_failed destination=log/0 reason=status_503 count=1 suppressed=0\n",
        )
        self.assertEqual(len(sink.batches), 1, "the diagnostic is not an event: the sink received only the event")
        self.assertEqual(core.diagnostics().channel.published, 1)

    def test_a_core_without_receivers_starts_no_dispatcher(self):
        before = set(dispatchers())
        core = build(self, config_for(), telemetry.with_log_sink(FailingSink()))
        self.assertEqual(set(dispatchers()) - before, set())
        core.emit(log_event())
        core.flush(2000)
        eventually(5.0, lambda: core.diagnostics().channel.published == 1, "the diagnostic counted")
        self.assertEqual(set(dispatchers()) - before, set())

    def test_the_dispatcher_starts_with_the_core_and_ends_at_the_shutdown(self):
        before = set(dispatchers())
        core = build(
            self, config_for(), telemetry.with_log_sink(FailingSink()), telemetry.with_diagnostics_callback(print)
        )
        mine = set(dispatchers()) - before
        self.assertEqual(len(mine), 1)
        core.shutdown(1000)
        eventually(5.0, lambda: all(not thread.is_alive() for thread in mine), "the dispatcher to end")

    def test_a_callback_that_raises_does_not_break_the_core(self):
        def raising(event: telemetry.DiagnosticEvent) -> None:
            raise RuntimeError("callback bug")

        sink = FailingSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink), telemetry.with_diagnostics_callback(raising))
        for number in range(3):
            core.emit(log_event(str(number)))
        result = core.flush(2000)
        self.assertEqual((result.sent, result.dropped), (0, 3))
        eventually(5.0, lambda: core.diagnostics().channel.callback_errors >= 1, "the error counted")
        self.assertEqual(sum(len(batch) for batch in sink.batches), 3)


class SecretsTest(unittest.TestCase):
    def test_no_diagnostic_holds_the_message_a_field_value_or_an_unsafe_reason(self):
        received: list[telemetry.DiagnosticEvent] = []
        buffer = io.StringIO()
        sink = FailingSink("bad reason://host.example/path with a space")
        core = build(
            self,
            config_for(),
            telemetry.with_log_sink(sink),
            telemetry.with_diagnostics_callback(received.append),
            telemetry.with_diagnostics_writer(buffer),
        )
        core.logger.error(
            telemetry.LogEntry(category="http", message=MESSAGE, fields={"password": SECRET, "order": "ord-" + SECRET})
        )
        core.flush(2000)
        eventually(5.0, lambda: len(received) == 1 and buffer.getvalue() != "", "the diagnostic")
        core.shutdown(1000)
        snapshot_json = json.dumps(dataclasses.asdict(core.diagnostics()), default=str)
        everything = snapshot_json + json.dumps([dataclasses.asdict(event) for event in received], default=str)
        everything += buffer.getvalue()
        self.assertNotIn(SECRET, everything)
        self.assertNotIn(MESSAGE, everything)
        self.assertNotIn("://", everything)
        self.assertRegex(received[0].reason, r"^[A-Za-z0-9_.:-]{1,64}$")
        self.assertEqual(received[0].reason, "bad_reason:__host.example_path_with_a_space")


if __name__ == "__main__":
    unittest.main()
