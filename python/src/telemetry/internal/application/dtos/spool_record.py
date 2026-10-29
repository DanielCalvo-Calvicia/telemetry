"""One event held by the local spool."""

from dataclasses import dataclass

from telemetry.internal.domain.value_objects.envelope_generated import Envelope


@dataclass(frozen=True, slots=True)
class SpoolRecord:
    """SpoolRecord is an event read from the local spool, with the id used to acknowledge it."""

    id: str
    event: Envelope
