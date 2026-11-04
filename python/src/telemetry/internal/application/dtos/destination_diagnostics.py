"""The diagnostics of one destination (REQ-DGN-001)."""

from dataclasses import dataclass, field
from typing import Mapping

from telemetry.internal.application.dtos.flush_outcome import FlushOutcome


@dataclass(frozen=True, slots=True)
class DestinationDiagnostics:
    """DestinationDiagnostics is a COPY of the figures of one destination, taken at one moment.

    ``kind`` and ``index`` name the destination. ``queue_depth`` is how many events wait now and ``queue_capacity``
    how many fit. ``accepted`` counts the events that were queued (queued, not delivered), ``delivered`` the events the
    destination accepted in the answer that ended their batch, ``dropped`` the events lost (full queue, answers that
    dropped, retry limit, shutdown) and ``dropped_by_reason`` splits them by reason. ``failed_events`` are the events
    a batch without retries failed. ``export_failures`` counts the calls whose answer had failed or dropped events,
    ``attempts`` all the calls, ``retries`` the calls beyond the first of a batch and ``batches`` the batches that
    ended. ``last_batch_size`` and ``max_batch_size`` are in events; the three latencies are the time of the calls in
    whole milliseconds (last, largest and the sum over all the attempts). ``last_success_at_ms`` is the unix
    millisecond of the last batch whose ending answer accepted events, the ones counted as ``delivered`` (0 means
    never). ``configuration_errors`` and
    ``serialization_failures`` add the failed and dropped events of every answer with that reason. The three spool
    figures are 0 until the spool exists (T-06.5). ``last_flush`` is the outcome of the last flush or shutdown and
    ``last_reason`` the reason of the last drop or failure (empty when none).
    """

    kind: str
    index: int
    queue_depth: int = 0
    queue_capacity: int = 0
    accepted: int = 0
    delivered: int = 0
    dropped: int = 0
    dropped_by_reason: Mapping[str, int] = field(default_factory=dict)
    failed_events: int = 0
    export_failures: int = 0
    attempts: int = 0
    retries: int = 0
    batches: int = 0
    last_batch_size: int = 0
    max_batch_size: int = 0
    last_export_latency_ms: int = 0
    max_export_latency_ms: int = 0
    total_export_latency_ms: int = 0
    last_success_at_ms: int = 0
    configuration_errors: int = 0
    serialization_failures: int = 0
    spool_events: int = 0
    spool_bytes: int = 0
    spool_quota_bytes: int = 0
    last_flush: FlushOutcome = field(default_factory=FlushOutcome)
    last_reason: str = ""
