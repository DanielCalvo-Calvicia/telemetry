"""Start the call context of a new request, the migration of the old ``NewRequestContext``."""

from telemetry.internal.application.call_context.new_call_context import new_call_context
from telemetry.internal.application.dtos.call_context_result import CallContextResult
from telemetry.internal.application.errors import CallContextIdFailedError
from telemetry.internal.application.ports.outbound.id_generator_port import IDGeneratorPort


def start_request(id_generator: IDGeneratorPort, application_name: str, instance_id: str) -> CallContextResult:
    """Return a new call context for a request; the caller puts it in ``use_call_context``.

    It carries the given application and instance id, a new correlation id and the starting indentation. The old
    process id (one per request) is the correlation id here. The id is generated first, as the text of a new event id
    from the port (a UUIDv7 is a fine unique request id); the port may get its own method later. When the generator
    fails the result has a CallContextIdFailedError chained to the cause (``__cause__``) and no context. Then the
    context is built with ``new_call_context``, so an invalid application or instance id gives its
    CallContextFieldInvalidError (code ``event_schema_invalid``, no context, no value in the message). The caller
    decides whether the request goes on without a context. This function never raises.
    """
    try:
        correlation_id = str(id_generator.new_event_id())
    except Exception as cause:  # a port may raise anything; the result carries it, the hot path never raises
        error = CallContextIdFailedError("the id of the request could not be generated")
        error.__cause__ = cause
        return CallContextResult(error=error)
    return new_call_context(application=application_name, instance_id=instance_id, correlation_id=correlation_id)
