"""The body of an HTTP batch (REQ-OUT-010): one JSON object in the canonical form, the same bytes in Go and Python."""

from typing import Sequence

from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.rules import SCHEMA_VERSION
from telemetry.internal.domain.value_objects.envelope_generated import Envelope


def build_batch_body(events: Sequence[Envelope]) -> bytes:
    """Return ``{"events": [...], "schema_version": "0.1"}`` as canonical JSON bytes; no events give ``[]``."""
    return canonical_json({"schema_version": SCHEMA_VERSION, "events": list(events)})
