"""Helpers of the canonical JSON tests: the shared fixture tables and the typed envelope."""

import json
import pathlib
from typing import Any

from telemetry.internal.domain.value_objects.enums_generated import ActorKind, Severity
from telemetry.internal.domain.value_objects.envelope_generated import Actor, Context, Envelope, Resource

# The tables are shared with the Go tests: both languages read the SAME files, so they cannot drift apart. T-02.8
# moves them to integration-tests/fixtures/.
REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
FIXTURES_DIRECTORY = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "canonical_json"


def load_cases(file_name: str) -> list[dict[str, Any]]:
    """Read the cases of a shared fixture file; an empty or missing file fails the test."""
    content = json.loads((FIXTURES_DIRECTORY / file_name).read_text(encoding="utf-8"))
    cases = content["cases"]
    if not cases:
        raise AssertionError(f"the fixture {file_name} has no case")
    return cases


def envelope_from_dict(event: dict[str, Any]) -> Envelope:
    """Build the generated Envelope dataclass from the JSON object of a fixture."""
    actor = event.get("actor")
    context = event.get("context")
    return Envelope(
        data=event["data"],
        event_id=event["event_id"],
        event_type=event["event_type"],
        occurred_at=event["occurred_at"],
        recorded_at=event["recorded_at"],
        resource=Resource(**event["resource"]),
        schema_version=event["schema_version"],
        severity=Severity(event["severity"]),
        actor=None if actor is None else Actor(**{**actor, "kind": ActorKind(actor["kind"])}),
        attributes=event.get("attributes"),
        context=None if context is None else Context(**context),
    )
