"""Build a call context from incoming values: validate them, never raise."""

from telemetry.internal.application.call_context.call_context import CallContext
from telemetry.internal.application.dtos.call_context_result import CallContextResult
from telemetry.internal.application.errors import CallContextFieldInvalidError, CallContextIdAllZerosError
from telemetry.internal.domain.base_value_objects.errors import ValueObjectError
from telemetry.internal.domain.base_value_objects.hex_digits import HexDigits
from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.value_objects.correlation_context import SPAN_ID_WIDTH, TRACE_ID_WIDTH


def _identifier_problem(field_name: str, value: str) -> str:
    """Return the problem of an identifier field, or an empty text; an empty value is not set."""
    if value == "":
        return ""
    try:
        Identifier(value)
    except ValueObjectError as error:
        return f"{field_name}: {error}"
    return ""


def _trace_id_problem(field_name: str, value: str, width: int) -> str:
    """Return the problem of a W3C id field (lowercase hex of ``width`` digits, not all zeros), or an empty text."""
    if value == "":
        return ""
    try:
        HexDigits(value, width)
    except ValueObjectError as error:
        return f"{field_name}: {error}"
    if set(value) == {"0"}:
        return f"{field_name}: {CallContextIdAllZerosError('an id must not be all zeros (W3C Trace Context)')}"
    return ""


def new_call_context(
    application: str = "",
    instance_id: str = "",
    execution_id: str = "",
    correlation_id: str = "",
    trace_id: str = "",
    span_id: str = "",
) -> CallContextResult:
    """Return a call context with the starting indentation, or one error that names every invalid field.

    An empty text means not set. ``application``, ``instance_id``, ``execution_id`` and ``correlation_id`` must be
    identifiers (not blank, no control characters; never trimmed). ``trace_id`` is 32 and ``span_id`` 16 lowercase
    hexadecimal digits and neither may be all zeros (W3C Trace Context). An incoming id is data, not authorization
    (REQ-TRC-003): it is only validated. The problems come in the order of the arguments, in one
    CallContextFieldInvalidError (code ``event_schema_invalid``) whose message names the fields and never the values.
    This function never raises.
    """
    problems = [
        _identifier_problem("application", application),
        _identifier_problem("instance_id", instance_id),
        _identifier_problem("execution_id", execution_id),
        _identifier_problem("correlation_id", correlation_id),
        _trace_id_problem("trace_id", trace_id, TRACE_ID_WIDTH),
        _trace_id_problem("span_id", span_id, SPAN_ID_WIDTH),
    ]
    found = [problem for problem in problems if problem]
    if found:
        return CallContextResult(
            error=CallContextFieldInvalidError(f"the call context is not valid ({'; '.join(found)})")
        )
    return CallContextResult(
        call_context=CallContext(
            application=application,
            instance_id=instance_id,
            execution_id=execution_id,
            correlation_id=correlation_id,
            trace_id=trace_id,
            span_id=span_id,
        )
    )
