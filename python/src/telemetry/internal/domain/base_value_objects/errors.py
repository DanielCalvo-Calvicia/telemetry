"""The errors of the base value objects.

They do not import the errors of the domain (that would be a cycle); the value objects of the domain wrap them in the
classified errors of ``telemetry.internal.domain.errors``. The mapping to the codes of the catalog is T-03.7.
"""


class ValueObjectError(Exception):
    """The base of every error of a base value object."""


class IdentifierBlankError(ValueObjectError):
    """An identifier is not text, or has no character that is not whitespace (an empty text is blank)."""


class IdentifierHasControlCharactersError(ValueObjectError):
    """An identifier holds a Unicode control character (category Cc), such as a tab or a newline."""


class HexDigitsInvalidError(ValueObjectError):
    """A hex text is not lowercase hexadecimal of the exact width, or the width is not a positive integer."""
