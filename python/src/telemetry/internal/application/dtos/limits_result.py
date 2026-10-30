"""The result of applying the size limits to an event."""

from dataclasses import dataclass
from typing import Optional

from telemetry.internal.domain.operations.limits.report import LimitsReport
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@dataclass(frozen=True, slots=True)
class LimitsResult:
    """LimitsResult holds the limited envelope or the error, exactly one of them, and always the counters."""

    envelope: Optional[Envelope] = None
    report: LimitsReport = LimitsReport()
    error: Optional[Exception] = None
