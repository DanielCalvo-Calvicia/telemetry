"""Map a call context to the correlation context of the envelope."""

from typing import Optional

from telemetry.internal.application.call_context.call_context import CallContext
from telemetry.internal.domain.errors import TelemetryError
from telemetry.internal.domain.value_objects.correlation_context import CorrelationContext


def to_correlation_context(call_context: CallContext) -> tuple[Optional[CorrelationContext], Optional[Exception]]:
    """Return ``(context, None)`` with the ids that are set, ``(None, None)`` when none is, or ``(None, error)``.

    Only ``trace_id``, ``span_id``, ``execution_id`` and ``correlation_id`` go to the envelope; an empty text is
    absent (None). ``application`` and ``instance_id`` are not part of the envelope context (the resource comes from
    the configuration); they stay in the call context for the migration of the old consumers and for later
    per-request use. The indentation is not part of it either (a presentation matter of the console adapter). A
    field that does not pass the value object is returned as the error, never raised.
    """
    fields = {
        "trace_id": call_context.trace_id or None,
        "span_id": call_context.span_id or None,
        "execution_id": call_context.execution_id or None,
        "correlation_id": call_context.correlation_id or None,
    }
    if all(value is None for value in fields.values()):
        return None, None
    try:
        return CorrelationContext(**fields), None
    except TelemetryError as error:
        return None, error
