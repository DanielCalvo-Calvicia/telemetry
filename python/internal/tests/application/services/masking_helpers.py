"""Helpers of the masking service tests: the envelope from a fixture and the masker from its options."""

from typing import Any

from telemetry.internal.domain.operations.masking.sensitive_data_masker import MaskerOptions, SensitiveDataMasker
from telemetry.internal.domain.value_objects.enums_generated import ActorKind, Severity
from telemetry.internal.domain.value_objects.envelope_generated import Actor, Context, Envelope, Resource


def envelope_from_json(event: dict[str, Any]) -> Envelope:
    """Build the generated Envelope from the JSON object of a fixture."""
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


def options_from_json(options: dict[str, Any]) -> MaskerOptions:
    """Build the masker options from the snake_case object of a fixture; an absent key is the default."""
    return MaskerOptions(**options)


def masker_from_json(options: dict[str, Any]) -> SensitiveDataMasker:
    """Create the masker of a fixture (it raises a ClassifiedError for an invalid option)."""
    return SensitiveDataMasker(options_from_json(options))
