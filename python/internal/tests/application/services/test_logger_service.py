"""Black-box tests of LoggerService with a fake emitter."""

import threading
import unittest
from typing import Optional

from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.log_entry import LogEntry
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.ports.inbound.logger_port import LoggerPort
from telemetry.internal.application.services.logger_service import LoggerService
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes
from telemetry.internal.domain.value_objects.event_types_generated import EVENT_TYPE_LOG_ENTRY_RECORDED


class FakeEmitter:
    """Record every input; raise when told to."""

    def __init__(self, error: Optional[Exception] = None) -> None:
        self.inputs: list[NewEventInput] = []
        self.error = error
        self._lock = threading.Lock()

    def emit(self, new_event: NewEventInput) -> EmitResult:
        with self._lock:
            self.inputs.append(new_event)
        if self.error is not None:
            raise self.error
        return EmitResult(accepted=1)


METHOD_SEVERITIES = (
    ("trace", Severity.TRACE),
    ("debug", Severity.DEBUG),
    ("info", Severity.INFO),
    ("warn", Severity.WARN),
    ("error", Severity.ERROR),
    ("critical", Severity.CRITICAL),
)


class LoggerServiceTest(unittest.TestCase):
    def test_it_satisfies_the_port(self):
        self.assertIsInstance(LoggerService(FakeEmitter()), LoggerPort)

    def test_each_method_emits_one_log_entry_recorded_with_its_severity(self):
        for method, severity in METHOD_SEVERITIES:
            with self.subTest(method):
                emitter = FakeEmitter()
                returned = getattr(LoggerService(emitter), method)(LogEntry(category="c", message="m"))
                self.assertIsNone(returned)
                self.assertEqual(len(emitter.inputs), 1)
                self.assertEqual(emitter.inputs[0].event_type, EVENT_TYPE_LOG_ENTRY_RECORDED)
                self.assertEqual(emitter.inputs[0].severity, severity)

    def test_data_keys_are_exactly_category_and_message_and_there_are_no_attributes(self):
        emitter = FakeEmitter()
        LoggerService(emitter).info(LogEntry(category="http", message="request done"))
        new_event = emitter.inputs[0]
        self.assertEqual(dict(new_event.data), {"category": "http", "message": "request done"})
        self.assertIsNone(new_event.attributes)
        self.assertIsNone(new_event.context)
        self.assertIsNone(new_event.occurred_at)

    def test_function_is_in_data_only_when_given(self):
        emitter = FakeEmitter()
        LoggerService(emitter).warn(LogEntry(category="c", message="m", function="serve"))
        self.assertEqual(dict(emitter.inputs[0].data), {"category": "c", "message": "m", "function": "serve"})

    def test_fields_become_attributes(self):
        emitter = FakeEmitter()
        LoggerService(emitter).info(LogEntry(category="c", message="m", fields={"order": 7, "gone": None, "": 1}))
        self.assertEqual(emitter.inputs[0].attributes, EventAttributes({"order": 7}))

    def test_an_emitter_that_raises_does_not_raise_to_the_caller(self):
        emitter = FakeEmitter(error=RuntimeError("sink down"))
        service = LoggerService(emitter)
        for method, _ in METHOD_SEVERITIES:
            with self.subTest(method):
                self.assertIsNone(getattr(service, method)(LogEntry(category="c", message="m")))
        self.assertEqual(len(emitter.inputs), 6)

    def test_a_bad_argument_does_not_raise_either(self):
        self.assertIsNone(LoggerService(FakeEmitter()).info(None))  # type: ignore[arg-type]

    def test_no_body_is_captured_by_a_call_with_only_category_and_message(self):
        emitter = FakeEmitter()
        LoggerService(emitter).error(LogEntry(category="c", message="m"))
        self.assertEqual(sorted(emitter.inputs[0].data), ["category", "message"])
        self.assertIsNone(emitter.inputs[0].attributes)

    def test_sixteen_threads_share_one_service(self):
        emitter = FakeEmitter()
        service = LoggerService(emitter)

        def work() -> None:
            for _ in range(50):
                service.info(LogEntry(category="c", message="m"))

        threads = [threading.Thread(target=work) for _ in range(16)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        self.assertEqual(len(emitter.inputs), 16 * 50)


if __name__ == "__main__":
    unittest.main()
