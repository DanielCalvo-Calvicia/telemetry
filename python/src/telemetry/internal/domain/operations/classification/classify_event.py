"""The rule that gives an event its priority class (REQ-DLV-009)."""

from typing import Mapping

from telemetry.internal.domain.value_objects.enums_generated import EventClass, Severity

LOG_EVENT_TYPE_PREFIX = "log."


def classify_event(event_type: str, severity: str, overrides: Mapping[str, str]) -> EventClass:
    """Return the class of an event: the assignment of its exact event type, else by its type and severity.

    An assignment for the EXACT ``event_type`` in ``overrides`` wins (its value is a class or the text of one).
    Otherwise an event type that starts with ``log.`` is ``bulk`` at TRACE and DEBUG and ``normal`` at every other
    severity, and any other event type is ``normal``. The class is a priority for a full queue and never a delivery
    guarantee. It is pure: it reads nothing and changes nothing.
    """
    override = overrides.get(event_type)
    if override is not None:
        return EventClass(override)
    if event_type.startswith(LOG_EVENT_TYPE_PREFIX) and severity in (Severity.TRACE, Severity.DEBUG):
        return EventClass.BULK
    return EventClass.NORMAL
