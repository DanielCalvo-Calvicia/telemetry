"""The errors of the domain operations, each one with the code of the catalog (contracts/errors/catalog.yaml).

The full error model arrives with T-03.7; these are small on purpose and easy to move.
"""

from telemetry.internal.domain.value_objects.error_codes_generated import (
    ERROR_CODE_EVENT_SCHEMA_INVALID,
    ERROR_CODE_EVENT_TYPE_UNKNOWN,
)


class ClassifiedError(Exception):
    """An error with a code of the catalog of error codes."""

    code: str = ERROR_CODE_EVENT_SCHEMA_INVALID

    def __init__(self, message: str, code: str | None = None) -> None:
        """Keep the message and, when it is given, a code other than the default of the class."""
        super().__init__(message)
        if code is not None:
            self.code = code


class NumberOutOfRangeError(ClassifiedError):
    """A number has no exact canonical form: NaN, an infinity or an integer beyond 2^53 - 1."""


class UnsupportedValueError(ClassifiedError):
    """A value has no canonical JSON form, such as a set or a string with a lone surrogate."""


class NonUtcTimeError(ClassifiedError):
    """An instant is not in UTC, or has no zone, or is outside the years 0001 to 9999."""


class EventSchemaInvalidError(ClassifiedError):
    """The data of an event does not match the schema of its event type (code ``event_schema_invalid``)."""

    code = ERROR_CODE_EVENT_SCHEMA_INVALID


class EventTypeUnknownError(ClassifiedError):
    """An event names an event type that is not in the registry (code ``event_type_unknown``)."""

    code = ERROR_CODE_EVENT_TYPE_UNKNOWN
