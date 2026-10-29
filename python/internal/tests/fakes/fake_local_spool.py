"""A fake of the local spool port, in memory."""

import threading
from typing import Optional, Sequence

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.dtos.spool_cleanup import SpoolCleanup
from telemetry.internal.application.dtos.spool_record import SpoolRecord
from telemetry.internal.application.dtos.spool_usage import SpoolUsage
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


def approximate_size(event: Envelope) -> int:
    """A stable size for an event, for the usage numbers."""
    return len(repr(event))


class FakeLocalSpool:
    """FakeLocalSpool keeps records in memory; fail_with makes append, read_batch, acknowledge and cleanup raise."""

    def __init__(self, quota_bytes: int = 1_000_000) -> None:
        self._lock = threading.Lock()
        self._records: list = []
        self._acknowledged: set = set()
        self._next_id = 0
        self._quota_bytes = quota_bytes
        self._failure: Optional[Exception] = None
        self._append_result: Optional[SendResult] = None
        self.append_calls = 0
        self.read_calls = 0
        self.acknowledge_calls = 0

    def fail_with(self, error: Optional[Exception]) -> None:
        """Make the methods that may raise raise this error; None clears it."""
        with self._lock:
            self._failure = error

    def set_append_result(self, result: Optional[SendResult]) -> None:
        """Override what append returns; None restores the default."""
        with self._lock:
            self._append_result = result

    def _raise_if_failing(self) -> None:
        if self._failure is not None:
            raise self._failure

    def _pending(self) -> list:
        return [record for record in self._records if record.id not in self._acknowledged]

    def append(self, events: Sequence[Envelope]) -> SendResult:
        """Store the events, or return the overridden result."""
        with self._lock:
            self.append_calls += 1
            self._raise_if_failing()
            if self._append_result is not None:
                return self._append_result
            for event in events:
                self._next_id += 1
                self._records.append(SpoolRecord(id=f"record-{self._next_id}", event=event))
            return SendResult(accepted=len(events), dropped=0, failed=0)

    def read_batch(self, maximum_events: int) -> list:
        """Return up to maximum_events records that are not acknowledged."""
        with self._lock:
            self.read_calls += 1
            self._raise_if_failing()
            return self._pending()[: max(maximum_events, 0)]

    def acknowledge(self, record_ids: Sequence[str]) -> None:
        """Mark the records as delivered."""
        with self._lock:
            self.acknowledge_calls += 1
            self._raise_if_failing()
            self._acknowledged.update(record_ids)

    def usage(self) -> SpoolUsage:
        """Count the records not acknowledged and their size."""
        with self._lock:
            pending = self._pending()
            return SpoolUsage(
                events=len(pending),
                bytes=sum(approximate_size(record.event) for record in pending),
                quota_bytes=self._quota_bytes,
            )

    def cleanup(self) -> SpoolCleanup:
        """Remove the acknowledged records."""
        with self._lock:
            self._raise_if_failing()
            removed = [record for record in self._records if record.id in self._acknowledged]
            self._records = self._pending()
            self._acknowledged = set()
            return SpoolCleanup(
                removed_events=len(removed), freed_bytes=sum(approximate_size(record.event) for record in removed)
            )
