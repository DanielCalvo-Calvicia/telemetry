"""The errors of the domain operations, each one with the code of the catalog (contracts/errors/catalog.yaml).

The full error model arrives with T-03.7; these are small on purpose and easy to move.
"""

from telemetry.internal.domain.base_value_objects.errors import HexDigitsInvalidError as HexDigitsInvalidError
from telemetry.internal.domain.base_value_objects.errors import (
    IdentifierBlankError as IdentifierBlankError,
)
from telemetry.internal.domain.base_value_objects.errors import (
    IdentifierHasControlCharactersError as IdentifierHasControlCharactersError,
)
from telemetry.internal.domain.base_value_objects.errors import ValueObjectError as ValueObjectError
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


# The invalid values (T-03.1). Each class states one rule and keeps the default code of ClassifiedError
# (event_schema_invalid); the exact mapping of each rule to a code of the catalog is T-03.7.
class InvalidValueError(ClassifiedError):
    """The base of the errors of a value object that is not valid at construction."""


class EventIdInvalidError(InvalidValueError):
    """An event id is not a UUID of version 7 in lowercase, hyphenated, with the RFC 4122 variant."""


class SeverityInvalidError(InvalidValueError):
    """A severity is not exactly one of the values of the generated Severity enum."""


class ActorKindInvalidError(InvalidValueError):
    """An actor kind is not exactly one of the values of the generated ActorKind enum."""


class TimeZoneInvalidError(InvalidValueError):
    """A time zone name is empty or does not have the shape of an IANA name (letters, digits, _ + - and /)."""


class ResourceFieldMissingError(InvalidValueError):
    """A resource lacks service_name or instance_id (None means absent)."""


class ResourceFieldInvalidError(InvalidValueError):
    """A field of a resource that is present is not valid: an identifier that is empty or has control characters."""


class ActorFieldInvalidError(InvalidValueError):
    """An actor has no id or an unknown kind, or its id, type or version is present but not a valid identifier."""


class ContextFieldInvalidError(InvalidValueError):
    """A field of a context that is present is not valid: a trace id, a span id or an identifier."""


class AttributeKeyInvalidError(InvalidValueError):
    """The key of an attribute is not a non-empty text, or the attributes are not a mapping."""


class AttributeValueInvalidError(InvalidValueError):
    """The value of an attribute is not a string, a boolean, a number or a flat list of them (null, object, nesting)."""
