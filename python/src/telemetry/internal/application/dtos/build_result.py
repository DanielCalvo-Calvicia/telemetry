"""The result of building an event."""

from dataclasses import dataclass
from typing import Optional

from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@dataclass(frozen=True, slots=True)
class BuildResult:
    """BuildResult holds the envelope or the error, exactly one of them: the builder never raises (ADR-005)."""

    envelope: Optional[Envelope] = None
    error: Optional[Exception] = None
