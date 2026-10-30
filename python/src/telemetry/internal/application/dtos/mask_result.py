"""The result of masking an event."""

from dataclasses import dataclass
from typing import Optional

from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@dataclass(frozen=True, slots=True)
class MaskResult:
    """MaskResult holds the masked envelope or the error, exactly one of them: masking never raises (ADR-005)."""

    envelope: Optional[Envelope] = None
    error: Optional[Exception] = None
