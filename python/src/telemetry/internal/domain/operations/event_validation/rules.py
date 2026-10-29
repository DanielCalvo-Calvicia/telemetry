"""The rules that the generated validators call (the twin of rules.go).

A check returns the first ``Problem`` it finds, or ``None``. A ``Problem`` says where a value breaks its schema and why,
and never carries the value: a value may be a secret. No check raises, whatever the value is.
"""

import dataclasses
import math
from collections.abc import Callable
from typing import Any

REASON_MISSING = "required field is missing"
REASON_UNKNOWN = "unknown field"
REASON_NOT_OBJECT = "expected object"
REASON_NOT_STRING = "expected string"
REASON_NOT_NUMBER = "expected number"
REASON_NOT_INTEGER = "expected integer"
REASON_NOT_BOOLEAN = "expected boolean"
REASON_NOT_FINITE = "number is not finite"
REASON_NOT_ALLOWED = "not one of the allowed values"
REASON_NOT_SCALAR = "expected a string, a number or a boolean"
REASON_NOT_SCALAR_LIST = "expected a scalar or a list of scalars"
REASON_TOO_MANY = "too many keys"
REASON_BAD_KEY = "the key has an invalid length"

# The longest text of a name that a path or a message shows. A longer one is cut, so a hostile name cannot fill a log.
DISPLAY_LIMIT = 64


@dataclasses.dataclass(frozen=True)
class Problem:
    """Where a value breaks its schema and why: a field path such as ``data.message`` and a short fixed reason."""

    path: str
    reason: str


Validator = Callable[[Any, str], Problem | None]
"""A check of one value found at a field path: it returns the first problem or ``None``."""


@dataclasses.dataclass(frozen=True)
class FieldRule:
    """A property an object declares and the check of its value."""

    name: str
    check: Validator


@dataclasses.dataclass(frozen=True)
class ObjectRules:
    """The rules of an object schema.

    The generated code lists ``required`` and ``fields`` in alphabetical order, so Go and Python report the same first
    problem. ``closed`` is true when a property that is not in ``fields`` is invalid (additionalProperties false).
    """

    required: tuple[str, ...] = ()
    fields: tuple[FieldRule, ...] = ()
    closed: bool = False


@dataclasses.dataclass(frozen=True)
class AttributesLimits:
    """The limits of the attributes schema; a zero means no limit."""

    max_properties: int = 0
    min_key_length: int = 0
    max_key_length: int = 0
    max_string_length: int = 0


def display_text(text: str) -> str:
    """Return a name safe to put in a path or a message: at most 64 characters and no ASCII control character.

    A control character becomes ``?`` and a name that was cut ends with ``...``. It is meant for names (a field, an
    event type), never for values.
    """
    shown = "".join(
        "?" if ord(character) < 0x20 or ord(character) == 0x7F else character for character in text[:DISPLAY_LIMIT]
    )
    return shown + "..." if len(text) > DISPLAY_LIMIT else shown


def _join_path(path: str, name: str) -> str:
    return f"{path}.{display_text(name)}"


def _is_object(value: Any) -> bool:
    """Tell if a value is a dict whose keys are all strings, the only kind of JSON object."""
    return isinstance(value, dict) and all(isinstance(key, str) for key in value)


def validate_object(value: Any, path: str, rules: ObjectRules) -> Problem | None:
    """Check that a value is an object and apply the rules.

    The order is: every missing required field, then the declared fields that are present in the order of the rules and,
    when the object is closed, the smallest unknown field. It returns the first problem, or ``None``.
    """
    if not _is_object(value):
        return Problem(path, REASON_NOT_OBJECT)
    for name in rules.required:
        if name not in value:
            return Problem(_join_path(path, name), REASON_MISSING)
    for rule in rules.fields:
        if rule.name in value:
            problem = rule.check(value[rule.name], _join_path(path, rule.name))
            if problem is not None:
                return problem
    if rules.closed:
        declared = {rule.name for rule in rules.fields}
        unknown = [name for name in value if name not in declared]
        if unknown:
            return Problem(_join_path(path, min(unknown)), REASON_UNKNOWN)
    return None


def check_object(value: Any, path: str) -> Problem | None:
    """Accept any object: the values are not checked. It is the check of a property of type object with no property."""
    return None if _is_object(value) else Problem(path, REASON_NOT_OBJECT)


def check_string(min_length: int, max_length: int) -> Validator:
    """Return the check of a string whose length, in characters, is at least ``min_length`` and at most ``max_length``.

    A zero means no limit.
    """

    def check(value: Any, path: str) -> Problem | None:
        if not isinstance(value, str):
            return Problem(path, REASON_NOT_STRING)
        if len(value) < min_length:
            return Problem(path, f"string is shorter than {min_length} characters")
        if 0 < max_length < len(value):
            return Problem(path, f"string is longer than {max_length} characters")
        return None

    return check


def check_boolean(value: Any, path: str) -> Problem | None:
    """Accept a bool and nothing else."""
    return None if isinstance(value, bool) else Problem(path, REASON_NOT_BOOLEAN)


# What _classify_number says about a value.
_NUMBER_NONE = "none"
_NUMBER_NOT_FINITE = "not finite"
_NUMBER_FRACTION = "fraction"
_NUMBER_WHOLE = "whole"


def _classify_number(value: Any) -> str:
    """Tell whether a value is a number and which kind. A bool is not a number."""
    if isinstance(value, bool):
        return _NUMBER_NONE
    if isinstance(value, int):
        return _NUMBER_WHOLE
    if isinstance(value, float):
        if not math.isfinite(value):
            return _NUMBER_NOT_FINITE
        return _NUMBER_WHOLE if value.is_integer() else _NUMBER_FRACTION
    return _NUMBER_NONE


def check_number(value: Any, path: str) -> Problem | None:
    """Accept a finite number: an int or a finite float. A bool is not a number; NaN and the infinities are invalid."""
    kind = _classify_number(value)
    if kind == _NUMBER_NONE:
        return Problem(path, REASON_NOT_NUMBER)
    if kind == _NUMBER_NOT_FINITE:
        return Problem(path, REASON_NOT_FINITE)
    return None


def check_integer(value: Any, path: str) -> Problem | None:
    """Accept a whole number: an int or a float with no fraction. 1.5 is invalid, a bool is not an integer."""
    kind = _classify_number(value)
    if kind in (_NUMBER_NONE, _NUMBER_FRACTION):
        return Problem(path, REASON_NOT_INTEGER)
    if kind == _NUMBER_NOT_FINITE:
        return Problem(path, REASON_NOT_FINITE)
    return None


def check_enum(allowed: tuple[str, ...]) -> Validator:
    """Return the check of a string that is one of the allowed values. The message never names the value."""

    def check(value: Any, path: str) -> Problem | None:
        if not isinstance(value, str):
            return Problem(path, REASON_NOT_STRING)
        return None if value in allowed else Problem(path, REASON_NOT_ALLOWED)

    return check


def check_attributes(limits: AttributesLimits) -> Validator:
    """Return the check of the attributes.

    They are an object whose values are scalars (a string, a number or a boolean) or lists of scalars, within the
    limits. The keys are checked in alphabetical order.
    """

    def check(value: Any, path: str) -> Problem | None:
        if not _is_object(value):
            return Problem(path, REASON_NOT_OBJECT)
        if 0 < limits.max_properties < len(value):
            return Problem(path, REASON_TOO_MANY)
        for key in sorted(value):
            key_length = len(key)
            if key_length < limits.min_key_length or 0 < limits.max_key_length < key_length:
                return Problem(_join_path(path, key), REASON_BAD_KEY)
            problem = _check_attribute_value(value[key], _join_path(path, key), limits)
            if problem is not None:
                return problem
        return None

    return check


def _check_attribute_value(value: Any, path: str, limits: AttributesLimits) -> Problem | None:
    """Check the value of one attribute: a scalar or a list of scalars."""
    if not isinstance(value, list):
        return _check_scalar(value, path, limits, REASON_NOT_SCALAR_LIST)
    for position, item in enumerate(value):
        problem = _check_scalar(item, f"{path}[{position}]", limits, REASON_NOT_SCALAR)
        if problem is not None:
            return problem
    return None


def _check_scalar(value: Any, path: str, limits: AttributesLimits, not_scalar: str) -> Problem | None:
    """Check a string within the limit, a boolean or a finite number; ``not_scalar`` is the reason otherwise."""
    if isinstance(value, str):
        if 0 < limits.max_string_length < len(value):
            return Problem(path, f"string is longer than {limits.max_string_length} characters")
        return None
    if isinstance(value, bool):
        return None
    kind = _classify_number(value)
    if kind == _NUMBER_NONE:
        return Problem(path, not_scalar)
    if kind == _NUMBER_NOT_FINITE:
        return Problem(path, REASON_NOT_FINITE)
    return None
