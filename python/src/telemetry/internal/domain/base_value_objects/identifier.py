"""The identifier: a text with a visible character and no control characters."""

import unicodedata
from dataclasses import dataclass

from telemetry.internal.domain.base_value_objects.errors import (
    IdentifierBlankError,
    IdentifierHasControlCharactersError,
)

CONTROL_CHARACTER_CATEGORY = "Cc"


@dataclass(frozen=True, slots=True)
class Identifier:
    """A validated text: at least one non-whitespace character and no Unicode control character.

    The text is never trimmed or changed. Absent is modelled by the caller (``None``); an empty text is invalid.
    """

    value: str

    def __post_init__(self) -> None:
        """Validate the text; raise IdentifierBlankError or IdentifierHasControlCharactersError."""
        if not isinstance(self.value, str) or not self.value.strip():
            raise IdentifierBlankError("an identifier must be text with at least one non-whitespace character")
        if any(unicodedata.category(character) == CONTROL_CHARACTER_CATEGORY for character in self.value):
            raise IdentifierHasControlCharactersError("an identifier must not hold control characters")

    def __str__(self) -> str:
        """Return the raw text."""
        return self.value
