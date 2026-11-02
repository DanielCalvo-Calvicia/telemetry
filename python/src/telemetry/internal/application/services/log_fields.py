"""The rule that turns the extra fields of a log call into the attributes of the event (shared with Go)."""

from typing import Any, Mapping, Optional

from telemetry.internal.application.dtos.log_entry import LogEntry
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes

MAX_EXACT_INTEGER = 2**53 - 1
UNSUPPORTED_VALUE_TEXT = "[unsupported value]"
NOT_A_NUMBER_TEXT = "NaN"
POSITIVE_INFINITY_TEXT = "+Inf"
NEGATIVE_INFINITY_TEXT = "-Inf"
_DROPPED = object()


def _number_value(value: Any) -> Any:
    """Return the number as it is kept, or the text that replaces it; the caller checked it is an int or a float."""
    if isinstance(value, int):
        if -MAX_EXACT_INTEGER <= value <= MAX_EXACT_INTEGER:
            return value
        return str(value)
    if value != value:  # NaN is the only float that differs from itself
        return NOT_A_NUMBER_TEXT
    if value == float("inf"):
        return POSITIVE_INFINITY_TEXT
    if value == float("-inf"):
        return NEGATIVE_INFINITY_TEXT
    return value


def _is_kept_scalar(value: Any) -> bool:
    """A string, a boolean, or a number that is finite and within +-(2^53 - 1)."""
    if isinstance(value, (str, bool)):
        return True
    if isinstance(value, (int, float)):
        return not isinstance(_number_value(value), str)
    return False


def attribute_value_of(value: Any) -> Any:
    """Return the attribute value of one field value, or None when the value is dropped.

    The rules, in this order:

    - None is dropped.
    - A string is kept; a bool is kept (it is checked before int).
    - An int or a float is kept as a number, EXCEPT: an int beyond +-(2^53 - 1) becomes its decimal text, and NaN and
      the infinities become exactly ``NaN``, ``+Inf`` and ``-Inf``. An integral float stays a float here; its
      canonical JSON prints it without the fraction (7.0 prints as 7), the same bytes as Go.
    - An exception (BaseException) becomes ``str(error)``; its traceback is not captured.
    - A list or a tuple is kept as a list only when EVERY element is a scalar by these same rules.
    - Anything else, and a list that holds something that is not a scalar, becomes as a WHOLE the text of its
      canonical JSON; when that fails the text is exactly ``[unsupported value]``.
    """
    converted = _convert(value)
    return None if converted is _DROPPED else converted


def _convert(value: Any) -> Any:
    if value is None:
        return _DROPPED
    if isinstance(value, (str, bool)):
        return value
    if isinstance(value, (int, float)):
        return _number_value(value)
    if isinstance(value, BaseException):
        return str(value)
    if isinstance(value, (list, tuple)) and all(_is_kept_scalar(element) for element in value):
        return list(value)
    return _canonical_text(value)


def _canonical_text(value: Any) -> str:
    try:
        return canonical_json(value).decode("utf-8")
    except Exception:  # a value without a JSON form: the text says so, the call is not lost
        return UNSUPPORTED_VALUE_TEXT


def fields_to_attribute_map(fields: Optional[Mapping[str, Any]]) -> dict[str, Any]:
    """Apply ``attribute_value_of`` to every pair of the fields and return the plain dict of what is kept.

    An empty key, a key that is not a string and a None value are dropped. Nothing else is dropped.
    """
    if not isinstance(fields, Mapping):
        return {}
    attribute_map: dict[str, Any] = {}
    for key, value in fields.items():
        if not isinstance(key, str) or key == "":
            continue
        converted = _convert(value)
        if converted is not _DROPPED:
            attribute_map[key] = converted
    return attribute_map


def fields_to_attributes(fields: Optional[Mapping[str, Any]]) -> Optional[EventAttributes]:
    """Return the EventAttributes of the fields (see ``attribute_value_of``), or None when there are none.

    When the attributes cannot be built the whole attributes of the call are dropped, never the event.
    """
    attribute_map = fields_to_attribute_map(fields)
    if not attribute_map:
        return None
    try:
        return EventAttributes(attribute_map)
    except Exception:  # drop the attributes of this call rather than the event
        return None


def log_entry_data(log_entry: LogEntry) -> dict[str, Any]:
    """Return the ``data`` of a log.entry.recorded event: category and message, and function only when it is given."""
    data: dict[str, Any] = {"category": log_entry.category, "message": log_entry.message}
    if log_entry.function:
        data["function"] = log_entry.function
    return data
