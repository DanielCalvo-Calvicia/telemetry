"""Canonical JSON by RFC 8785 (JCS): the same logical event gives the same bytes in Go and in Python."""

import dataclasses
import datetime
import enum
from typing import Any

from telemetry.internal.domain.errors import UnsupportedValueError
from telemetry.internal.domain.operations.canonical_json.number_format import format_float, format_int
from telemetry.internal.domain.operations.canonical_json.time_format import format_utc_time


def canonical_json(value: Any) -> bytes:
    """Return the canonical JSON (RFC 8785) of a value, as UTF-8 bytes.

    It accepts the generated Envelope (a dataclass: the fields that are None are omitted and an enum prints as its
    value, like Go does with omitempty) and plain dict, list, tuple, str, int, float, bool and None. A bool is never an
    int. A datetime prints as the string of format_utc_time. It raises a ClassifiedError for NaN, an infinity, an
    integer beyond +-(2^53 - 1), a string that cannot be UTF-8 and a value that has no JSON form.
    """
    return _serialize(value).encode("utf-8")


def _serialize(value: Any) -> str:
    if value is None:
        return "null"
    if isinstance(value, bool):  # before int: a bool is an int in Python
        return "true" if value else "false"
    if isinstance(value, enum.Enum):
        return _serialize(value.value)
    if isinstance(value, str):
        return _serialize_string(value)
    if isinstance(value, int):
        return format_int(int(value))
    if isinstance(value, float):
        return format_float(float(value))
    if isinstance(value, datetime.datetime):
        return _serialize_string(format_utc_time(value))
    if dataclasses.is_dataclass(value) and not isinstance(value, type):
        members = {field.name: getattr(value, field.name) for field in dataclasses.fields(value)}
        return _serialize_object({name: member for name, member in members.items() if member is not None})
    if isinstance(value, dict):
        return _serialize_object(value)
    if isinstance(value, (list, tuple)):
        return "[" + ",".join(_serialize(element) for element in value) + "]"
    raise UnsupportedValueError(f"the type {type(value).__name__} has no JSON form")


def _serialize_object(members: dict[Any, Any]) -> str:
    for name in members:
        if not isinstance(name, str):
            raise UnsupportedValueError("an object needs string keys")
    # RFC 8785 section 3.2.3: sort by UTF-16 code units. The UTF-16 big-endian bytes compare in the same order.
    ordered_names = sorted(members, key=_utf16_sort_key)
    return "{" + ",".join(f"{_serialize_string(name)}:{_serialize(members[name])}" for name in ordered_names) + "}"


def _utf16_sort_key(text: str) -> bytes:
    try:
        return text.encode("utf-16-be")
    except UnicodeEncodeError as encode_error:
        raise UnsupportedValueError("a key has a lone surrogate") from encode_error


_ESCAPES = {'"': '\\"', "\\": "\\\\", "\b": "\\b", "\t": "\\t", "\n": "\\n", "\f": "\\f", "\r": "\\r"}


def _serialize_string(text: str) -> str:
    try:
        text.encode("utf-8")
    except UnicodeEncodeError as encode_error:
        raise UnsupportedValueError("a string has a lone surrogate") from encode_error
    pieces = []
    for character in text:
        if character in _ESCAPES:
            pieces.append(_ESCAPES[character])
        elif character < " ":
            pieces.append(f"\\u{ord(character):04x}")
        else:
            pieces.append(character)
    return '"' + "".join(pieces) + '"'
