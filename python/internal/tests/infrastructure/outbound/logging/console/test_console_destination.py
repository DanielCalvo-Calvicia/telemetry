"""Black-box tests of the console destination: the filter, the failures, the concurrency, the ports and the core."""

import dataclasses
import io
import threading
import unittest
from typing import Any

import telemetry
from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.fakes import FakeClock, FakeIdGenerator
from internal.tests.infrastructure.config.helpers import merged_config
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.console.console_destination import ConsoleDestination
from telemetry.internal.infrastructure.outbound.logging.console.error.console_errors import (
    ConsoleEncodingFailedError,
    ConsoleWriteFailedError,
)
from telemetry.internal.infrastructure.outbound.logging.console.settings import Settings
from telemetry.logging import console


def event(severity: str = "INFO", message: str = "started", attributes: Any = None):
    """A log entry envelope."""
    fields: dict[str, Any] = {
        "schema_version": "0.1",
        "event_id": "01900000-0000-7000-8000-000000000001",
        "event_type": "log.entry.recorded",
        "occurred_at": "2026-10-28T10:00:00.123456Z",
        "recorded_at": "2026-10-28T10:00:00.123456Z",
        "severity": severity,
        "resource": {"service_name": "sensor-hub", "instance_id": "hub-01"},
        "data": {"category": "http", "message": message},
    }
    if attributes is not None:
        fields["attributes"] = attributes
    return envelope_from_json(fields)


class FailingWriter:
    """A writer that raises on the calls whose number (from 1) is in ``failing``, and keeps the other lines."""

    def __init__(self, failing: set[int], error: BaseException | None = None) -> None:
        self.failing = failing
        self.error = error or OSError("disk full: placeholder-secret-not-real")
        self.calls = 0
        self.lines: list[str] = []

    def write(self, text: str) -> int:
        self.calls += 1
        if self.calls in self.failing:
            raise self.error
        self.lines.append(text)
        return len(text)


class ConstructionTest(unittest.TestCase):
    def test_invalid_settings_raise_a_configuration_error_with_every_problem(self):
        with self.assertRaises(ConfigurationError) as raised:
            ConsoleDestination(Settings(format="xml", minimum_severity="info"), io.StringIO())
        self.assertEqual([problem.field for problem in raised.exception.problems], ["format", "minimum_severity"])
        self.assertNotIn("xml", str(raised.exception))

    def test_a_value_that_is_not_a_text_is_a_problem_not_an_exception_of_another_kind(self):
        for settings in (Settings(format=None), Settings(minimum_severity=3), Settings(format=b"json")):  # type: ignore[arg-type]
            with self.subTest(settings=settings):
                with self.assertRaises(ConfigurationError):
                    ConsoleDestination(settings, io.StringIO())

    def test_it_serves_both_ports(self):
        destination = ConsoleDestination(Settings(), io.StringIO())
        self.assertIsInstance(destination, LogSinkPort)
        self.assertIsInstance(destination, EventSinkPort)


class SendTest(unittest.TestCase):
    def test_an_event_below_the_minimum_is_not_written_and_counts_as_accepted(self):
        writer = io.StringIO()
        result = ConsoleDestination(Settings(minimum_severity="WARN"), writer).send(
            [event("DEBUG"), event("INFO"), event("WARN"), event("CRITICAL")]
        )
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (4, 0, 0, ""))
        self.assertEqual(writer.getvalue().count("\n"), 2)

    def test_an_empty_batch_writes_nothing(self):
        writer = io.StringIO()
        result = ConsoleDestination(Settings(), writer).send([])
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (0, 0, 0, ""))
        self.assertEqual(writer.getvalue(), "")

    def test_each_line_is_one_call_of_the_writer(self):
        writer = FailingWriter(set())
        ConsoleDestination(Settings(format="text"), writer).send([event(message="one"), event(message="two")])
        self.assertEqual(writer.calls, 2)
        self.assertTrue(all(line.endswith("\n") and line.count("\n") == 1 for line in writer.lines))

    def test_a_failing_write_fails_that_event_and_the_next_ones_are_still_written(self):
        writer = FailingWriter({2, 4})
        events = [event(message=f"m{number}") for number in range(1, 6)]
        result = ConsoleDestination(Settings(format="text"), writer).send(events)
        self.assertEqual((result.accepted, result.dropped, result.failed), (3, 0, 2))
        self.assertEqual(result.reason, "write_failed")
        self.assertEqual([line.rsplit(" ", 1)[1].strip() for line in writer.lines], ["m1", "m3", "m5"])
        self.assertNotIn("placeholder-secret-not-real", result.reason)

    def test_an_event_that_cannot_be_encoded_fails_and_the_others_are_written(self):
        for format_name in ("json", "text"):
            with self.subTest(format=format_name):
                writer = io.StringIO()
                events = [event(message="before"), event(attributes={"ratio": float("nan")}), event(message="after")]
                result = ConsoleDestination(Settings(format=format_name), writer).send(events)
                self.assertEqual((result.accepted, result.dropped, result.failed), (2, 0, 1))
                self.assertEqual(result.reason, "serialization_failed")
                self.assertIn("before", writer.getvalue())
                self.assertIn("after", writer.getvalue())

    def test_the_reason_is_the_first_one_that_happened(self):
        writer = FailingWriter({1})
        events = [event(), event(attributes={"ratio": float("nan")})]
        result = ConsoleDestination(Settings(), writer).send(events)
        self.assertEqual((result.failed, result.reason), (2, "write_failed"))

    def test_something_unexpected_fails_the_events_not_yet_counted(self):
        writer = io.StringIO()
        result = ConsoleDestination(Settings(), writer).send([event(), None, event()])  # type: ignore[list-item]
        self.assertEqual((result.accepted, result.dropped, result.failed), (1, 0, 2))
        self.assertEqual(result.reason, "destination_failed")
        self.assertEqual(writer.getvalue().count("\n"), 1)

    def test_a_batch_that_cannot_be_read_does_not_raise(self):
        def broken_batch():
            raise RuntimeError("no batch")
            yield  # pragma: no cover

        result = ConsoleDestination(Settings(), io.StringIO()).send(broken_batch())  # type: ignore[arg-type]
        self.assertEqual((result.accepted, result.dropped, result.failed), (0, 0, 0))
        self.assertEqual(result.reason, "destination_failed")

    def test_a_keyboard_interrupt_of_the_writer_is_not_swallowed(self):
        writer = FailingWriter({1}, KeyboardInterrupt())
        with self.assertRaises(KeyboardInterrupt):
            ConsoleDestination(Settings(), writer).send([event()])

    def test_the_send_result_is_a_send_result(self):
        result = ConsoleDestination(Settings(), io.StringIO()).send([event()])
        self.assertIsInstance(result, telemetry.SendResult)


class WriteEventTest(unittest.TestCase):
    def test_it_writes_one_line_and_returns_none(self):
        writer = io.StringIO()
        self.assertIsNone(ConsoleDestination(Settings(), writer).write_event(event()))
        self.assertEqual(writer.getvalue().count("\n"), 1)

    def test_a_filtered_event_is_not_written_and_returns_none(self):
        writer = io.StringIO()
        self.assertIsNone(ConsoleDestination(Settings(minimum_severity="ERROR"), writer).write_event(event("INFO")))
        self.assertEqual(writer.getvalue(), "")

    def test_a_failing_writer_raises_a_classified_error_chained_from_its_cause(self):
        cause = OSError("disk full")
        writer = FailingWriter({1}, cause)
        with self.assertRaises(ConsoleWriteFailedError) as raised:
            ConsoleDestination(Settings(), writer).write_event(event())
        self.assertIs(raised.exception.__cause__, cause)
        self.assertIsInstance(raised.exception, ClassifiedError)
        self.assertNotIn("disk full", str(raised.exception))

    def test_an_event_that_cannot_be_encoded_raises_a_classified_error_chained_from_its_cause(self):
        with self.assertRaises(ConsoleEncodingFailedError) as raised:
            ConsoleDestination(Settings(), io.StringIO()).write_event(event(attributes={"ratio": float("nan")}))
        self.assertIsNotNone(raised.exception.__cause__)


class ConcurrencyTest(unittest.TestCase):
    def test_the_lines_of_sixteen_threads_never_interleave(self):
        writer = io.StringIO()
        destination = ConsoleDestination(Settings(), writer)
        per_thread = 50
        results: list[Any] = []

        def work(number: int) -> None:
            batch = [event(message=f"thread {number} line {line} " + "x" * 200) for line in range(per_thread)]
            results.append(destination.send(batch))

        threads = [threading.Thread(target=work, args=(number,)) for number in range(16)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        lines = writer.getvalue().split("\n")
        self.assertEqual(lines.pop(), "")
        self.assertEqual(len(lines), 16 * per_thread)
        for line in lines:
            self.assertTrue(line.startswith("{") and line.endswith("}"), line[:80])
        self.assertEqual(len(set(lines)), 16 * per_thread)
        self.assertEqual(sum(result.accepted for result in results), 16 * per_thread)


class PublicPackageTest(unittest.TestCase):
    def test_the_defaults(self):
        self.assertEqual(dataclasses.asdict(console.default_settings()), {"format": "json", "minimum_severity": "INFO"})

    def test_a_destination_with_a_writer_and_invalid_settings(self):
        with self.assertRaises(ConfigurationError):
            console.new_destination_with_writer(console.Settings(format="xml"), io.StringIO())
        with self.assertRaises(ConfigurationError):
            console.new_destination(console.Settings(minimum_severity="nope"))

    def test_the_standard_library_logging_still_imports(self):
        import logging

        self.assertTrue(hasattr(logging, "getLogger"))
        self.assertEqual(console.__name__, "telemetry.logging.console")

    def test_the_whole_core_writes_the_canonical_json_of_the_masked_event(self):
        stream = io.StringIO()
        sink = console.new_destination_with_writer(console.Settings(), stream)
        options = [
            telemetry.with_log_sink(sink),
            telemetry.with_clock(FakeClock()),
            telemetry.with_id_generator(FakeIdGenerator([EventId("01900000-0000-7000-8000-000000000001")])),
        ]
        core = telemetry.new(merged_config("microservice", {"resource": {"service_name": "sensor-hub"}}), *options)
        core.logger.info(
            telemetry.LogEntry(
                category="http", message="login password=placeholder-secret-not-real", fields={"order": 7}
            )
        )
        core.shutdown()
        lines = stream.getvalue().split("\n")
        self.assertEqual(len(lines), 2)
        self.assertEqual(lines[1], "")
        self.assertIn("login password=*****MASKED*****", lines[0])
        self.assertNotIn("placeholder-secret-not-real", lines[0])
        self.assertIn('"event_type":"log.entry.recorded"', lines[0])
        self.assertEqual(lines[0], canonical_json(_parse(lines[0])).decode("utf-8"))


def _parse(line: str) -> Any:
    import json

    return json.loads(line)
