"""The logger port: the way a project records a log entry."""

from typing import Protocol, runtime_checkable

from telemetry.internal.application.dtos.log_entry import LogEntry


@runtime_checkable
class LoggerPort(Protocol):
    """LoggerPort is what a project calls to record a log entry; it has one method per severity of the contract.

    Each call builds one ``log.entry.recorded`` event and hands it to the log sinks. A log call NEVER raises and
    returns nothing: an event that is rejected (not valid, or without a destination) is not an error of the caller; it
    is counted by the diagnostics of T-05.5 (ADR-005, REQ-DLV-002). A log call does not stop the program, not even at
    CRITICAL. No request body, payload or traceback is recorded unless the caller passes it in ``fields``
    (REQ-LOG-006). The public name is ``Logger``.
    """

    def trace(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity TRACE; never raise."""

    def debug(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity DEBUG; never raise."""

    def info(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity INFO; never raise."""

    def warn(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity WARN; never raise."""

    def error(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity ERROR; never raise."""

    def critical(self, log_entry: LogEntry) -> None:
        """Record the entry with the severity CRITICAL; never raise, never stop the program."""
