"""The JSON line of an event: its canonical JSON and a line feed, as bytes."""

from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


def encode_json_line(event: Envelope) -> bytes:
    """Return the canonical JSON of the event (RFC 8785) followed by a line feed: the same bytes as Go writes.

    It raises a ClassifiedError when the event has no canonical form (NaN, an integer beyond 2^53 - 1, a lone
    surrogate).
    """
    return canonical_json(event) + b"\n"
