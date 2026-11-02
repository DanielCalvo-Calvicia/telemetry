"""The readable line of an event: time, severity, category, function, message and the attributes.

The line is a CONTRACT shared with Go: ``occurred_at``, a space, the severity padded on the right to 8 characters, the
body and the attributes, then a line feed. A ``log.entry.recorded`` body is ``[category] function: message`` (the
function only when it is not empty); any other event type prints its type and the canonical JSON of its data. Each
attribute is `` key=canonical-json-of-the-value``, the keys sorted by UTF-16 code units. The line never holds more than
the (already masked) event holds.
"""

from types import MappingProxyType
from typing import Any, Mapping

from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json, utf16_sort_key
from telemetry.internal.domain.value_objects.envelope_generated import Envelope

EVENT_TYPE_LOG_ENTRY_RECORDED = "log.entry.recorded"
SEVERITY_WIDTH = 8
_NAMED_ESCAPES: Mapping[str, str] = MappingProxyType({"\n": "\\n", "\r": "\\r", "\t": "\\t"})


def escape_text(text: str) -> str:
    """Make a text safe for one line.

    LF, CR and TAB become the two characters ``\\n``, ``\\r`` and ``\\t``; any other character below U+0020 and U+007F
    become ``\\u`` and four lower case hexadecimal digits; every other character (non-ASCII included) is kept.
    """
    pieces = []
    for character in text:
        if character in _NAMED_ESCAPES:
            pieces.append(_NAMED_ESCAPES[character])
        elif character < " " or character == "\x7f":
            pieces.append(f"\\u{ord(character):04x}")
        else:
            pieces.append(character)
    return "".join(pieces)


def _text_of(data: dict[str, Any], name: str) -> str:
    """The member of the data when it is a text, else an empty text."""
    value = data.get(name)
    return value if isinstance(value, str) else ""


def _body(event: Envelope) -> str:
    if event.event_type == EVENT_TYPE_LOG_ENTRY_RECORDED:
        function_name = _text_of(event.data, "function")
        function_part = f" {escape_text(function_name)}:" if function_name else ""
        category = escape_text(_text_of(event.data, "category"))
        message = escape_text(_text_of(event.data, "message"))
        return f" [{category}]{function_part} {message}"
    return f" {escape_text(event.event_type)} {canonical_json(event.data).decode('utf-8')}"


def _attributes(event: Envelope) -> str:
    attributes = event.attributes or {}
    return "".join(
        f" {escape_text(key)}={canonical_json(attributes[key]).decode('utf-8')}"
        for key in sorted(attributes, key=utf16_sort_key)
    )


def encode_text_line(event: Envelope) -> str:
    """Return the readable line of the event, with its line feed.

    It raises a ClassifiedError for a value that has no canonical JSON form (NaN, an integer beyond 2^53 - 1, a lone
    surrogate).
    """
    severity = str(event.severity.value).ljust(SEVERITY_WIDTH)
    return f"{event.occurred_at} {severity}{_body(event)}{_attributes(event)}\n"
