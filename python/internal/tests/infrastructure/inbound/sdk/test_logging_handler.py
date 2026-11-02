"""Black-box tests of LoggingHandler: the mapping of a native record to a log call."""

import dataclasses
import logging
import threading
import unittest
from typing import Any, Optional

import telemetry
from internal.tests.fakes import FakeClock, FakeIdGenerator, FakeLogSink
from telemetry.internal.application.dtos.log_entry import LogEntry
from telemetry.internal.infrastructure.inbound.sdk.logging_handler import (
    STANDARD_RECORD_ATTRIBUTES,
    severity_method_name,
)


class RecordingLogger:
    """A LoggerPort that keeps the calls as (method, entry); it can be told to raise."""

    def __init__(self, error: Optional[Exception] = None) -> None:
        self.calls: list[tuple[str, LogEntry]] = []
        self.failure = error
        self._lock = threading.Lock()

    def _record(self, method: str, log_entry: LogEntry) -> None:
        with self._lock:
            self.calls.append((method, log_entry))
        if self.failure is not None:
            raise self.failure

    def trace(self, log_entry: LogEntry) -> None:
        self._record("trace", log_entry)

    def debug(self, log_entry: LogEntry) -> None:
        self._record("debug", log_entry)

    def info(self, log_entry: LogEntry) -> None:
        self._record("info", log_entry)

    def warn(self, log_entry: LogEntry) -> None:
        self._record("warn", log_entry)

    def error(self, log_entry: LogEntry) -> None:
        self._record("error", log_entry)

    def critical(self, log_entry: LogEntry) -> None:
        self._record("critical", log_entry)


def new_record(level: int = logging.INFO, message: Any = "m", args: Any = None, **extra: Any) -> logging.LogRecord:
    record = logging.LogRecord("t", level, __file__, 10, message, args, None, func="somewhere")
    record.__dict__.update(extra)
    return record


def attach(test: unittest.TestCase, name: str, handler: logging.Handler, level: int = logging.DEBUG) -> logging.Logger:
    """Add the handler to a private logger of the standard library for the length of one test."""
    logger = logging.getLogger(name)
    logger.propagate = False
    logger.setLevel(level)
    logger.addHandler(handler)
    test.addCleanup(logger.removeHandler, handler)
    return logger


class SeverityTest(unittest.TestCase):
    def test_the_level_table(self):
        table = (
            (0, "trace"),
            (9, "trace"),
            (10, "debug"),
            (19, "debug"),
            (20, "info"),
            (29, "info"),
            (30, "warn"),
            (39, "warn"),
            (40, "error"),
            (49, "error"),
            (50, "critical"),
            (100, "critical"),
        )
        for level, method in table:
            with self.subTest(level=level):
                self.assertEqual(severity_method_name(level), method)


class LoggingHandlerTest(unittest.TestCase):
    def test_a_record_becomes_a_call_with_the_default_category(self):
        recording = RecordingLogger()
        telemetry.LoggingHandler(recording, "app").handle(new_record(logging.WARNING, "disk low"))
        expected = LogEntry(category="app", message="disk low", function="", fields={})
        self.assertEqual(recording.calls, [("warn", expected)])

    def test_the_arguments_of_the_message_are_formatted(self):
        recording = RecordingLogger()
        telemetry.LoggingHandler(recording).handle(new_record(message="user %s has %d items", args=("bob", 3)))
        self.assertEqual(recording.calls[0][1].message, "user bob has 3 items")

    def test_category_and_function_extras_are_not_fields(self):
        recording = RecordingLogger()
        telemetry.LoggingHandler(recording, "app").handle(new_record(category="storage", function="check", order=7))
        log_entry = recording.calls[0][1]
        self.assertEqual((log_entry.category, log_entry.function), ("storage", "check"))
        self.assertEqual(dict(log_entry.fields), {"order": 7})

    def test_a_category_or_function_that_is_not_text_is_ignored_and_is_not_a_field(self):
        recording = RecordingLogger()
        telemetry.LoggingHandler(recording, "app").handle(new_record(category=5, function=None, order=7))
        log_entry = recording.calls[0][1]
        self.assertEqual((log_entry.category, log_entry.function, dict(log_entry.fields)), ("app", "", {"order": 7}))

    def test_the_standard_attributes_of_a_record_are_not_fields(self):
        recording = RecordingLogger()
        record = new_record(message="m")
        record.message = "formatted"
        record.asctime = "now"
        telemetry.LoggingHandler(recording).handle(record)
        self.assertEqual(dict(recording.calls[0][1].fields), {})
        for name in ("name", "msg", "args", "levelno", "pathname", "funcName", "created", "message", "asctime"):
            self.assertIn(name, STANDARD_RECORD_ATTRIBUTES)
        self.assertIn("taskName", STANDARD_RECORD_ATTRIBUTES)

    def test_the_function_name_of_the_record_is_not_used(self):
        recording = RecordingLogger()
        telemetry.LoggingHandler(recording).handle(new_record())
        self.assertEqual(recording.calls[0][1].function, "")

    def test_the_level_of_the_handler_filters(self):
        recording = RecordingLogger()
        handler = telemetry.LoggingHandler(recording, level=logging.ERROR)
        self.assertEqual(handler.level, logging.ERROR)
        logger = attach(self, "test.handler.level", handler)
        logger.warning("low")
        logger.error("high")
        self.assertEqual([call[0] for call in recording.calls], ["error"])

    def test_no_exception_or_traceback_is_captured(self):
        recording = RecordingLogger()
        logger = attach(self, "test.handler.exc", telemetry.LoggingHandler(recording))
        try:
            raise ValueError("secret detail")
        except ValueError:
            logger.exception("failed")
        log_entry = recording.calls[0][1]
        self.assertEqual(log_entry.message, "failed")
        self.assertEqual(dict(log_entry.fields), {})

    def test_it_works_through_logging_get_logger(self):
        recording = RecordingLogger()
        logger = attach(self, "test.handler.get_logger", telemetry.LoggingHandler(recording, "app"), level=1)
        logger.log(5, "tiny")
        logger.info("order %d", 7, extra={"order": 7, "category": "shop"})
        self.assertEqual(recording.calls[0][0], "trace")
        method, log_entry = recording.calls[1]
        self.assertEqual((method, log_entry.category, log_entry.message), ("info", "shop", "order 7"))
        self.assertEqual(dict(log_entry.fields), {"order": 7})

    def test_a_failing_logger_does_not_raise_and_handle_error_is_not_called(self):
        handler = telemetry.LoggingHandler(RecordingLogger(error=RuntimeError("down")))
        handler.handleError = lambda record: self.fail("handleError was called")  # type: ignore[assignment]
        handler.handle(new_record())

    def test_a_message_that_cannot_be_formatted_does_not_raise(self):
        recording = RecordingLogger()
        handler = telemetry.LoggingHandler(recording)
        handler.handleError = lambda record: self.fail("handleError was called")  # type: ignore[assignment]
        handler.handle(new_record(message="%d", args=("not a number",)))
        self.assertEqual(recording.calls, [])

    def test_sixteen_threads_through_one_handler(self):
        recording = RecordingLogger()
        handler = telemetry.LoggingHandler(recording, "app")

        def work() -> None:
            for index in range(50):
                handler.handle(new_record(order=index))

        threads = [threading.Thread(target=work) for _ in range(16)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        self.assertEqual(len(recording.calls), 16 * 50)


class SecretThroughTheWholeCoreTest(unittest.TestCase):
    def test_a_field_named_password_never_reaches_the_sink(self):
        sink = FakeLogSink()
        base = telemetry.microservice()
        resource = dataclasses.replace(base.resource, service_name="sensor-hub", instance_id="hub-01")
        core = telemetry.new(
            dataclasses.replace(base, resource=resource),
            telemetry.with_log_sink(sink),
            telemetry.with_clock(FakeClock()),
            telemetry.with_id_generator(FakeIdGenerator()),
        )
        core.logger.info(telemetry.LogEntry(category="auth", message="login", fields={"password": "hunter2"}))
        telemetry.LoggingHandler(core.logger, "auth").handle(new_record(password="hunter3"))
        self.assertEqual(sink.calls, 2)
        for batch in sink.batches:
            for envelope in batch:
                self.assertNotIn("hunter", str(envelope))
                self.assertEqual(envelope.attributes["password"], "*****MASKED*****")


if __name__ == "__main__":
    unittest.main()
