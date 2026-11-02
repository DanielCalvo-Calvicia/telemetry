"""The console destination: events as JSON lines or readable text on the standard output (REQ-LOG-001, REQ-LOG-003).

It satisfies both the log sink port and the event sink port. Each event is one line, written with ONE call of the
writer under a lock, so the lines of concurrent callers never interleave. The destination never raises from ``send``
(ADR-005); ``write_event`` is the way for a caller of the ADAPTER to see the classified error of a failed write.
"""

import threading
from typing import Callable, Protocol, Sequence

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.domain.rules import CONSOLE_FORMAT_TEXT
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.severity import parse_severity, severity_at_least
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.console.error.console_errors import (
    ConsoleEncodingFailedError,
    ConsoleWriteFailedError,
)
from telemetry.internal.infrastructure.outbound.logging.console.helpers.encode_json_line import encode_json_line
from telemetry.internal.infrastructure.outbound.logging.console.helpers.encode_text_line import encode_text_line
from telemetry.internal.infrastructure.outbound.logging.console.settings import Settings, validate_settings

REASON_SERIALIZATION_FAILED = "serialization_failed"
REASON_WRITE_FAILED = "write_failed"
REASON_DESTINATION_FAILED = "destination_failed"


class Writer(Protocol):
    """Anything with ``write(text)``: the standard output, a file or an ``io.StringIO``."""

    def write(self, text: str) -> object:
        """Write the text."""


class ConsoleDestination:
    """ConsoleDestination writes each event it accepts as one line to its writer.

    An event below the minimum severity is not written and it counts as ACCEPTED: the destination took it and its own
    policy decided not to write it (it is not a drop, which is what a destination does when it cannot take the event).
    """

    def __init__(self, settings: Settings, writer: Writer) -> None:
        """Validate the settings (a ConfigurationError holds every problem) and keep the writer."""
        problems = validate_settings(settings)
        if problems:
            raise ConfigurationError(problems)
        self._encode: Callable[[Envelope], str] = (
            encode_text_line if settings.format == CONSOLE_FORMAT_TEXT else encode_json_line
        )
        self._minimum = parse_severity(settings.minimum_severity)
        self._writer = writer
        self._lock = threading.Lock()

    def send(self, events: Sequence[Envelope]) -> SendResult:
        """Write the events and say how many were accepted, dropped or failed; it never raises (ADR-005).

        Each event is counted once. A line that cannot be built fails with ``serialization_failed``, a writer that
        raises fails the event with ``write_failed`` and the next events are still tried; whatever is not counted when
        something unexpected happens fails with ``destination_failed``. The reason is the first one that happened.
        """
        accepted = failed = total = 0
        reason = ""
        try:
            batch = list(events)
            total = len(batch)
            for event in batch:
                if not severity_at_least(event.severity, self._minimum):
                    accepted += 1
                    continue
                try:
                    line = self._encode(event)
                except Exception:
                    failed += 1
                    reason = reason or REASON_SERIALIZATION_FAILED
                    continue
                try:
                    self._write_line(line)
                except Exception:
                    failed += 1
                    reason = reason or REASON_WRITE_FAILED
                    continue
                accepted += 1
        except Exception:
            failed += total - accepted - failed
            reason = reason or REASON_DESTINATION_FAILED
        return SendResult(accepted=accepted, dropped=0, failed=failed, reason=reason)

    def write_event(self, event: Envelope) -> None:
        """Write one event; a filtered event is not written. Both cases return None.

        Unlike ``send`` it raises: a ConsoleEncodingFailedError when the line cannot be built and a
        ConsoleWriteFailedError when the writer fails, each chained from its cause. The error goes to the caller of the
        adapter and never to a log call (ADR-005).
        """
        if not severity_at_least(event.severity, self._minimum):
            return
        try:
            line = self._encode(event)
        except Exception as cause:
            raise ConsoleEncodingFailedError("the event could not be turned into a line") from cause
        try:
            self._write_line(line)
        except Exception as cause:
            raise ConsoleWriteFailedError("the console could not be written") from cause

    def _write_line(self, line: str) -> None:
        with self._lock:
            self._writer.write(line)
