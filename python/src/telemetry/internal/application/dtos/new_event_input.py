"""What a capability gives the core to build an event."""

from dataclasses import dataclass
from typing import Any, Mapping, Optional

from telemetry.internal.domain.value_objects.actor_identity import ActorIdentity
from telemetry.internal.domain.value_objects.correlation_context import CorrelationContext
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant


@dataclass(frozen=True, slots=True)
class NewEventInput:
    """NewEventInput is the input of EventBuilderService.build.

    There is NO recorded_at field, on purpose: only the library sets it, from the clock port, so a caller cannot
    supply it (REQ-CTR-008, ADR-012). ``occurred_at`` is optional and defaults to the time of the call. ``data`` may be
    None, which means an empty object.
    """

    event_type: str
    severity: Severity
    data: Optional[Mapping[str, Any]]
    occurred_at: Optional[UtcInstant] = None
    actor: Optional[ActorIdentity] = None
    context: Optional[CorrelationContext] = None
    attributes: Optional[EventAttributes] = None
