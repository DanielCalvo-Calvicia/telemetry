"""The attributes of an event: flat key-value pairs of scalars (REQ-CTR-010). The size limits are T-03.5."""

from dataclasses import dataclass
from types import MappingProxyType
from typing import Any, Mapping, Union

from telemetry.internal.domain.errors import AttributeKeyInvalidError, AttributeValueInvalidError

AttributeScalar = Union[str, bool, int, float]
AttributeValue = Union[AttributeScalar, tuple]
SCALAR_TYPES = (str, bool, int, float)  # bool is a subclass of int; it is a valid scalar of its own


def _normalized_value(value: Any) -> Any:
    """Return the scalar, or a tuple of scalars for a flat list or tuple; return None when the value is not valid."""
    if isinstance(value, SCALAR_TYPES):
        return value
    if isinstance(value, (list, tuple)) and all(isinstance(item, SCALAR_TYPES) for item in value):
        return tuple(value)
    return None


@dataclass(frozen=True, slots=True, eq=False)
class EventAttributes:
    """A read-only copy of a mapping of non-empty text keys to a string, boolean, number or flat list of them.

    A null, an object, a nested list or a list that holds one of those is rejected: AttributeKeyInvalidError for a key
    (or for input that is not a mapping) and AttributeValueInvalidError for a value. Every failure is reported in one
    error. The input is copied: a later change by the caller does not change this object, and ``entries`` is a
    read-only mapping whose lists are tuples. NaN and the infinities are not special here.
    """

    entries: Mapping[str, AttributeValue]

    def __post_init__(self) -> None:
        """Validate and copy the entries."""
        if not isinstance(self.entries, Mapping):
            raise AttributeKeyInvalidError("the attributes must be a mapping of text keys to values")
        copied: dict[str, AttributeValue] = {}
        key_problems: list[str] = []
        value_problems: list[str] = []
        for key, value in self.entries.items():
            if not isinstance(key, str) or not key:
                key_problems.append(f"the key {key!r} is not a non-empty text")
                continue
            normalized = _normalized_value(value)
            if normalized is None:
                value_problems.append(f"the value of {key!r} is not a scalar or a flat list of scalars")
                continue
            copied[key] = normalized
        if key_problems or value_problems:
            message = "the attributes are not valid (" + "; ".join(key_problems + value_problems) + ")"
            raise (AttributeKeyInvalidError if key_problems else AttributeValueInvalidError)(message)
        object.__setattr__(self, "entries", MappingProxyType(copied))

    def as_dict(self) -> dict[str, Any]:
        """Return a new plain dict with the lists as lists, ready for JSON; changing it changes nothing here."""
        return {key: list(value) if isinstance(value, tuple) else value for key, value in self.entries.items()}

    def __eq__(self, other: object) -> bool:
        """Equal when the other is an EventAttributes with the same entries."""
        if not isinstance(other, EventAttributes):
            return NotImplemented
        return dict(self.entries) == dict(other.entries)

    def __hash__(self) -> int:
        """Hash the entries; the lists are tuples, so they hash."""
        return hash(frozenset(self.entries.items()))
