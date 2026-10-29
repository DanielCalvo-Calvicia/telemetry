"""Lowercase hexadecimal text of an exact width (a W3C trace id has 32 digits, a span id 16)."""

from dataclasses import dataclass

from telemetry.internal.domain.base_value_objects.errors import HexDigitsInvalidError

LOWERCASE_HEX_DIGITS = frozenset("0123456789abcdef")


@dataclass(frozen=True, slots=True)
class HexDigits:
    """A validated text of lowercase hexadecimal digits that has exactly ``width`` characters."""

    value: str
    width: int

    def __post_init__(self) -> None:
        """Validate the width and the text; raise HexDigitsInvalidError."""
        if isinstance(self.width, bool) or not isinstance(self.width, int) or self.width <= 0:
            raise HexDigitsInvalidError("the width of a hex text must be a positive integer")
        if not isinstance(self.value, str) or len(self.value) != self.width:
            raise HexDigitsInvalidError(f"a hex text must have exactly {self.width} characters")
        if not set(self.value) <= LOWERCASE_HEX_DIGITS:
            raise HexDigitsInvalidError("a hex text must have only lowercase hexadecimal digits")

    def __str__(self) -> str:
        """Return the raw text."""
        return self.value
