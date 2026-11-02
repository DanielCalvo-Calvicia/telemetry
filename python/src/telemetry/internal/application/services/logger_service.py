"""The service behind the logger: one method per severity, each one a log.entry.recorded event."""

from typing import Protocol

from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.log_entry import LogEntry
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.services.log_fields import fields_to_attributes, log_entry_data
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.event_types_generated import EVENT_TYPE_LOG_ENTRY_RECORDED


class EventEmitter(Protocol):
    """EventEmitter is what the logger needs of the core: turn an input into a delivered event."""

    def emit(self, new_event: NewEventInput) -> EmitResult:
        """Run the input through the chain and deliver the event; EventPipelineService satisfies this."""


class LoggerService:
    """LoggerService implements LoggerPort over an emitter (the event pipeline).

    Each method builds a NewEventInput of the type ``log.entry.recorded`` with the severity of the method and hands it
    to the emitter; it NEVER raises and returns nothing (ADR-005, REQ-DLV-002). The ``data`` has the category and the
    message, and the function only when it is given. The fields become the attributes under the rules of
    ``fields_to_attributes``. The call context of the thread is attached by the pipeline, not here, and the
    application name is not per call: it is ``resource.service_name`` of the configuration. Nothing is captured
    automatically: no request body, no payload (REQ-LOG-006) and no exception or traceback; only what the caller
    passes in the fields. It holds no mutable state, so one instance serves many threads.
    """

    def __init__(self, emitter: EventEmitter) -> None:
        """Keep the emitter; the composition root passes the event pipeline."""
        self._emitter = emitter

    def trace(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity TRACE; never raise."""
        self._log(Severity.TRACE, log_entry)

    def debug(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity DEBUG; never raise."""
        self._log(Severity.DEBUG, log_entry)

    def info(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity INFO; never raise."""
        self._log(Severity.INFO, log_entry)

    def warn(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity WARN; never raise."""
        self._log(Severity.WARN, log_entry)

    def error(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity ERROR; never raise."""
        self._log(Severity.ERROR, log_entry)

    def critical(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity CRITICAL; never raise and never stop the program."""
        self._log(Severity.CRITICAL, log_entry)

    def _log(self, severity: Severity, log_entry: LogEntry) -> None:
        try:
            self._emitter.emit(
                NewEventInput(
                    event_type=EVENT_TYPE_LOG_ENTRY_RECORDED,
                    severity=severity,
                    data=log_entry_data(log_entry),
                    attributes=fields_to_attributes(log_entry.fields),
                )
            )
        except Exception:  # the last resort of ADR-005: a log call never fails its caller
            return
