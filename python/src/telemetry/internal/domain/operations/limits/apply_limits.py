"""Apply the size limits to an envelope (REQ-CTR-010)."""

import dataclasses

from telemetry.internal.domain.errors import EventTooLargeError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.limits.attribute_cuts import cut_attributes
from telemetry.internal.domain.operations.limits.report import LimitsOutcome
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.limits import Limits


def apply_limits(envelope: Envelope, limits: Limits) -> LimitsOutcome:
    """Return a LIMITED COPY of the envelope with the counters of what was cut; the input is never modified.

    The policy (REQ-CTR-010), in this order:

    a. The attribute keys are processed in the sorted order of the canonical form (UTF-16 code units); the keys beyond
       the first ``max_attribute_keys`` are dropped and counted in ``attribute_keys_dropped``.
    b. A kept key longer than ``max_attribute_key_chars`` characters is cut and counted in
       ``attribute_keys_truncated``; if the cut key equals a key already kept, the key is dropped and counted only in
       ``attribute_keys_dropped``.
    c. A text value over ``max_attribute_value_chars`` characters is cut and counted in
       ``attribute_values_truncated``; each text of a list is limited and counted on its own. Numbers, booleans and
       None are never cut.
    d. Characters are Unicode code points (never bytes, never UTF-16 units).
    e. When no key remains, ``attributes`` becomes None.
    f. After the cuts, the size is the number of BYTES of the canonical JSON of the envelope; over
       ``max_event_bytes`` the event is rejected (``events_rejected`` is 1 and the other counters are kept); exactly
       ``max_event_bytes`` is accepted.

    Only ``attributes`` is cut; ``data`` and every other field are untouched. A too-large event does not raise: the
    outcome carries an EventTooLargeError (code ``limit_exceeded``, numbers only, never the content) and no envelope.
    The application service turns every other failure into a result too (ADR-005).
    """
    attributes, report = cut_attributes(envelope.attributes, limits)
    limited = dataclasses.replace(envelope, attributes=attributes or None)
    size = len(canonical_json(limited))
    if size > limits.max_event_bytes:
        rejected = dataclasses.replace(report, events_rejected=1)
        error = EventTooLargeError(f"the event has {size} bytes and the limit is {limits.max_event_bytes}")
        return LimitsOutcome(envelope=None, report=rejected, error=error)
    return LimitsOutcome(envelope=limited, report=report)
