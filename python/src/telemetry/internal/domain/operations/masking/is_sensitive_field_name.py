"""The rule of the sensitive field names: which fields have their value always hidden."""

import re
from typing import Iterable, Optional, Sequence

from telemetry.internal.domain.rules import FIELD_NAME_CAMEL_BOUNDARY_PATTERN, SENSITIVE_FIELD_NAMES

_CAMEL_BOUNDARY = re.compile(FIELD_NAME_CAMEL_BOUNDARY_PATTERN)
_SEPARATORS = str.maketrans({"-": "_", " ": "_", ".": "_"})


def normalize_field_name(field_name: str) -> str:
    """Read a field name in snake case: accessKey, Access-Key and access_key are the same name."""
    spaced_name = _CAMEL_BOUNDARY.sub(r"\1_\2", field_name)
    return spaced_name.translate(_SEPARATORS).lower()


def normalize_field_names(field_names: Iterable[str]) -> tuple[str, ...]:
    """Normalize a list of names, without the empty ones (an empty name would match nothing useful)."""
    normalized_names = (normalize_field_name(field_name) for field_name in field_names)
    return tuple(name for name in normalized_names if name)


DEFAULT_SENSITIVE_NAMES = normalize_field_names(SENSITIVE_FIELD_NAMES)


def is_sensitive_field_name(field_name: str, sensitive_names: Optional[Sequence[str]] = None) -> bool:
    """Report whether the value of a field with this name must always be hidden.

    The name is read in snake case and matches when it is equal to one of the sensitive names or ends with "_" and
    one of them. ``sensitive_names`` (already normalized) defaults to the list of domain/rules.py.
    """
    names = DEFAULT_SENSITIVE_NAMES if sensitive_names is None else sensitive_names
    normalized_name = normalize_field_name(field_name)
    for sensitive_name in names:
        if normalized_name == sensitive_name or normalized_name.endswith("_" + sensitive_name):
            return True
    return False
