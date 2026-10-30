"""The errors of the application layer, each one with a code of the catalog.

The services RETURN them in a result (BuildResult, MaskResult, LimitsResult) and the hot path never raises them
(ADR-005); only the start can raise one (ConfigurationInstanceIdFailedError). The cause is chained with ``from``, the
role of ``errors.Join`` in Go, and a message never holds a value of the caller.
"""

from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.value_objects.error_codes_generated import (
    ERROR_CODE_EVENT_BUILD_FAILED,
    ERROR_CODE_INSTANCE_ID_UNAVAILABLE,
    ERROR_CODE_LIMITS_FAILED,
    ERROR_CODE_MASKING_FAILED,
)


class EventBuildFailedError(ClassifiedError):
    """The event could not be built: a port failed or something unexpected happened (code ``event_build_failed``)."""

    code = ERROR_CODE_EVENT_BUILD_FAILED


class EnvelopeMaskingFailedError(ClassifiedError):
    """The event could not be masked (code ``masking_failed``); the event is dropped, never sent unmasked (ADR-006)."""

    code = ERROR_CODE_MASKING_FAILED


class EnvelopeLimitsFailedError(ClassifiedError):
    """The size limits could not be applied (code ``limits_failed``); the event is dropped, never sent unchecked."""

    code = ERROR_CODE_LIMITS_FAILED


class ConfigurationInstanceIdFailedError(ClassifiedError):
    """The instance id could not be generated at start (code ``instance_id_unavailable``); raised at start only."""

    code = ERROR_CODE_INSTANCE_ID_UNAVAILABLE
