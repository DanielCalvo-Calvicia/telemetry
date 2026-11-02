"""The handler of the standard library ``logging``: a native record becomes one log call."""

import logging
from typing import Any

from telemetry.internal.application.dtos.log_entry import LogEntry
from telemetry.internal.application.ports.inbound.logger_port import LoggerPort

# Every attribute a LogRecord has by itself; the other keys of its __dict__ come from ``extra``.
STANDARD_RECORD_ATTRIBUTES = frozenset(logging.makeLogRecord({}).__dict__) | {"message", "asctime", "taskName"}
CATEGORY_KEY = "category"
FUNCTION_KEY = "function"


def severity_method_name(level: int) -> str:
    """Return the name of the LoggerPort method for a native level.

    Below 10 is trace, 10 to 19 debug, 20 to 29 info, 30 to 39 warn, 40 to 49 error and 50 and above critical.
    """
    if level < logging.DEBUG:
        return "trace"
    if level < logging.INFO:
        return "debug"
    if level < logging.WARNING:
        return "info"
    if level < logging.ERROR:
        return "warn"
    if level < logging.CRITICAL:
        return "error"
    return "critical"


class LoggingHandler(logging.Handler):
    """LoggingHandler sends the records of the standard ``logging`` to a LoggerPort.

    The severity comes from the level of the record (see ``severity_method_name``) and the message is
    ``record.getMessage()``, so the %-arguments are formatted. The ``extra`` attributes of the record are the fields,
    except ``category`` (a string that sets the category) and ``function`` (a string that sets the function), which are
    not fields; without a ``category`` the default category is used. Choices, on purpose: the function is NOT taken
    from ``record.funcName`` nor from the source, the time is NOT ``record.created`` (the event is stamped with the
    time of the call by the clock port), and ``exc_info`` and the traceback are NOT captured (REQ-LOG-006): a project
    that wants the error passes it in ``extra``. ``emit`` never raises and never calls ``handleError`` (ADR-005).
    """

    def __init__(self, logger: LoggerPort, default_category: str = "", level: int = logging.NOTSET) -> None:
        """Keep the logger and the category used when a record gives none; ``level`` filters like any handler."""
        super().__init__(level)
        self._logger = logger
        self._default_category = default_category

    def emit(self, record: logging.LogRecord) -> None:
        """Turn the record into one log call; never raise."""
        try:
            category = self._default_category
            function = ""
            fields: dict[str, Any] = {}
            for key, value in record.__dict__.items():
                if key in STANDARD_RECORD_ATTRIBUTES:
                    continue
                if key == CATEGORY_KEY:
                    if isinstance(value, str):
                        category = value
                elif key == FUNCTION_KEY:
                    if isinstance(value, str):
                        function = value
                else:
                    fields[key] = value
            log_entry = LogEntry(category=category, message=record.getMessage(), function=function, fields=fields)
            getattr(self._logger, severity_method_name(record.levelno))(log_entry)
        except Exception:  # a failing log call never reaches the program, and handleError is never called
            return
