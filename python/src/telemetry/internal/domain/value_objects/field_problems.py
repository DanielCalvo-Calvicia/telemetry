"""The collector of the failed fields of a composite value object, so one error can report all of them."""

from typing import Optional

from telemetry.internal.domain.base_value_objects.errors import ValueObjectError
from telemetry.internal.domain.base_value_objects.hex_digits import HexDigits
from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.errors import InvalidValueError


class FieldProblems:
    """Collects what is missing and what is invalid in the fields of one composite, then raises one error.

    ``None`` is an absent field: an error when the field is required, fine when it is optional. A present field
    that is empty or malformed is always invalid.
    """

    def __init__(self) -> None:
        """Start without problems."""
        self.missing: list[str] = []
        self.invalid: list[str] = []

    def identifier(self, field_name: str, value: Optional[str], required: bool) -> None:
        """Check one identifier field."""
        if value is None:
            if required:
                self.missing.append(field_name)
            return
        try:
            Identifier(value)
        except ValueObjectError as error:
            self.invalid.append(f"{field_name}: {error}")

    def hex_digits(self, field_name: str, value: Optional[str], width: int) -> None:
        """Check one optional lowercase hex field of an exact width."""
        if value is None:
            return
        try:
            HexDigits(value, width)
        except ValueObjectError as error:
            self.invalid.append(f"{field_name}: {error}")

    def add_invalid(self, field_name: str, error: Exception) -> None:
        """Record a field that failed its own check."""
        self.invalid.append(f"{field_name}: {error}")

    def raise_if_any(
        self,
        subject: str,
        missing_error: type[InvalidValueError],
        invalid_error: type[InvalidValueError],
    ) -> None:
        """Raise one error with every problem; ``missing_error`` wins when a required field is missing."""
        if not self.missing and not self.invalid:
            return
        parts = []
        if self.missing:
            parts.append("missing: " + ", ".join(self.missing))
        parts.extend(self.invalid)
        message = f"{subject} is not valid ({'; '.join(parts)})"
        raise (missing_error if self.missing else invalid_error)(message)
