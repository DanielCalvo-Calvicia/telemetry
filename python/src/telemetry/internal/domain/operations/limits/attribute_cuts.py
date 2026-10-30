"""The cuts of the attributes of an event: keys beyond the limit, long keys and long values."""

from typing import Any, Optional

from telemetry.internal.domain.operations.canonical_json.canonical_json import utf16_sort_key
from telemetry.internal.domain.operations.limits.report import LimitsReport
from telemetry.internal.domain.value_objects.limits import Limits


def cut_attributes(
    attributes: Optional[dict[str, Any]], limits: Limits
) -> tuple[Optional[dict[str, Any]], LimitsReport]:
    """Return a limited COPY of the attributes and the counters; the input is never modified.

    The keys are processed in the sorted order of the canonical form (UTF-16 code units of the original key), so the
    result does not depend on the order in which they were given. The keys beyond the first ``max_attribute_keys`` are
    dropped. A kept key over ``max_attribute_key_chars`` characters is cut; when the cut key equals a key already kept
    the key is dropped (counted only as dropped). Characters are code points. When no key remains, None is returned.
    """
    if not attributes:
        return None, LimitsReport()
    dropped = 0
    keys_truncated = 0
    values_truncated = 0
    kept: dict[str, Any] = {}
    for position, key in enumerate(sorted(attributes, key=utf16_sort_key)):
        if position >= limits.max_attribute_keys:
            dropped += 1
            continue
        cut_key = key[: limits.max_attribute_key_chars]
        if cut_key in kept:
            dropped += 1
            continue
        if cut_key != key:
            keys_truncated += 1
        value, cuts = _cut_value(attributes[key], limits.max_attribute_value_chars)
        values_truncated += cuts
        kept[cut_key] = value
    report = LimitsReport(
        attribute_keys_dropped=dropped,
        attribute_keys_truncated=keys_truncated,
        attribute_values_truncated=values_truncated,
    )
    return kept, report


def _cut_value(value: Any, max_chars: int) -> tuple[Any, int]:
    """Cut a text, or each text of a list or tuple, to max_chars; numbers, booleans and None are never touched."""
    if isinstance(value, str):
        return (value[:max_chars], 1) if len(value) > max_chars else (value, 0)
    if isinstance(value, (list, tuple)):
        elements = [_cut_value(element, max_chars) for element in value]
        return type(value)(element for element, _ in elements), sum(cuts for _, cuts in elements)
    return value, 0
