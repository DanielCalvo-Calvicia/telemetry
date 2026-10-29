"""Fakes of the six sink ports: one private base, one class per capability."""

import threading
import time
from typing import Optional, Sequence

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


class _FakeSink:
    """Record every batch received as a copy, wait the set delay and return the set result; never raise."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._batches: list = []
        self._result: Optional[SendResult] = None
        self._delay_seconds = 0.0

    @property
    def batches(self) -> list:
        """A copy of the batches received, each one a copy too."""
        with self._lock:
            return [list(batch) for batch in self._batches]

    @property
    def calls(self) -> int:
        """The number of calls to send."""
        with self._lock:
            return len(self._batches)

    def set_result(self, result: SendResult) -> None:
        """Set the result that send returns; the default accepts every event."""
        with self._lock:
            self._result = result

    def set_delay(self, seconds: float) -> None:
        """Make send wait this long before it returns."""
        with self._lock:
            self._delay_seconds = seconds

    def send(self, events: Sequence[Envelope]) -> SendResult:
        """Record a copy of the batch, wait, and return the result."""
        with self._lock:
            self._batches.append(list(events))
            result = self._result
            delay_seconds = self._delay_seconds
        if delay_seconds > 0:
            time.sleep(delay_seconds)
        if result is not None:
            return result
        return SendResult(accepted=len(events), dropped=0, failed=0)


class FakeLogSink(_FakeSink):
    """FakeLogSink is a fake of the log sink port."""


class FakeEventSink(_FakeSink):
    """FakeEventSink is a fake of the event sink port."""


class FakeMetricsSink(_FakeSink):
    """FakeMetricsSink is a fake of the metrics sink port."""


class FakeTraceSink(_FakeSink):
    """FakeTraceSink is a fake of the trace sink port."""


class FakeDeviceTelemetrySink(_FakeSink):
    """FakeDeviceTelemetrySink is a fake of the device telemetry sink port."""


class FakeAiTelemetrySink(_FakeSink):
    """FakeAiTelemetrySink is a fake of the AI telemetry sink port."""
