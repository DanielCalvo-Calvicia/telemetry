"""The routing of an event type to the kind of destination that serves it."""

from types import MappingProxyType

DESTINATION_LOG = "log"
DESTINATION_EVENT = "event"
DESTINATION_METRICS = "metrics"
DESTINATION_TRACE = "trace"
DESTINATION_DEVICE_TELEMETRY = "device_telemetry"
DESTINATION_AI_TELEMETRY = "ai_telemetry"

_KIND_OF_FIRST_SEGMENT = MappingProxyType(
    {
        "log": DESTINATION_LOG,
        "metric": DESTINATION_METRICS,
        "trace": DESTINATION_TRACE,
        "device": DESTINATION_DEVICE_TELEMETRY,
        "ai": DESTINATION_AI_TELEMETRY,
    }
)


def destination_kind_of(event_type: str) -> str:
    """Return the kind of destination of an event type, by the first segment of its name (before the first dot).

    ``log`` goes to the log destinations, ``metric`` to metrics, ``trace`` to trace, ``device`` to device telemetry,
    ``ai`` to AI telemetry; every other name, also an empty one, goes to the event destinations. The returned text is
    the name of the matching field of Destinations. It is a pure function: it never raises.
    """
    first_segment = event_type.split(".", 1)[0] if isinstance(event_type, str) else ""
    return _KIND_OF_FIRST_SEGMENT.get(first_segment, DESTINATION_EVENT)
