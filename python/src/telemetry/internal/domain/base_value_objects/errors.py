"""The errors of the base value objects, and the root of every error of the library.

They do not import the errors of the domain (that would be a cycle); the value objects of the domain wrap them in the
classified errors of ``telemetry.internal.domain.errors``. A base value object error is a validation error: its code is
``event_schema_invalid`` (``error_code_of``).
"""


class TelemetryError(Exception):
    """The root of every exception of the library.

    Every layer derives its errors from it. The hot path never raises one: the services RETURN it in a result
    (BuildResult, MaskResult, LimitsResult) and it is raised only by construction and start (ADR-005, REQ-CFG-002).
    A message never holds a value passed by the caller.
    """


class ValueObjectError(TelemetryError):
    """The base of every error of a base value object (raised by the domain layer, at construction)."""


class IdentifierBlankError(ValueObjectError):
    """An identifier is not text, or has no character that is not whitespace (an empty text is blank)."""


class IdentifierHasControlCharactersError(ValueObjectError):
    """An identifier holds a Unicode control character (category Cc), such as a tab or a newline."""


class HexDigitsInvalidError(ValueObjectError):
    """A hex text is not lowercase hexadecimal of the exact width, or the width is not a positive integer."""
