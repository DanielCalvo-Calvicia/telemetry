"""Map the validated value objects to the generated Envelope, Resource, Actor and Context.

No business logic lives here. The null-versus-empty rules, the same in Go and Python:

* ``data`` is never None: None becomes an empty dict and a top-level copy (a dict) of the caller's mapping is kept.
* ``attributes`` is None when it is empty.
* ``actor`` is None when it is absent; ``context`` is None when absent or when it has no field at all.
"""

from typing import Any, Mapping, Optional

from telemetry.internal.domain.value_objects.actor_identity import ActorIdentity
from telemetry.internal.domain.value_objects.correlation_context import CorrelationContext
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.envelope_generated import Actor, Context, Envelope, Resource
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity


def to_resource(resource: ResourceIdentity) -> Resource:
    """Map the resource identity to the generated Resource."""
    return Resource(
        instance_id=resource.instance_id,
        service_name=resource.service_name,
        environment=resource.environment,
        host=resource.host,
        service_version=resource.service_version,
        time_zone=resource.time_zone,
    )


def to_actor(actor: Optional[ActorIdentity]) -> Optional[Actor]:
    """Map the actor identity to the generated Actor; None stays None."""
    if actor is None:
        return None
    return Actor(id=actor.id, kind=actor.kind, type=actor.type, version=actor.version)


def to_context(context: Optional[CorrelationContext]) -> Optional[Context]:
    """Map the correlation context to the generated Context; None, or a context with no field, gives None."""
    if context is None:
        return None
    fields = (context.trace_id, context.span_id, context.execution_id, context.correlation_id)
    if all(value is None for value in fields):
        return None
    return Context(
        correlation_id=context.correlation_id,
        execution_id=context.execution_id,
        span_id=context.span_id,
        trace_id=context.trace_id,
    )


def to_envelope(
    schema_version: str,
    event_id: str,
    event_type: str,
    occurred_at: str,
    recorded_at: str,
    severity: Severity,
    resource: Resource,
    actor: Optional[Actor],
    context: Optional[Context],
    attributes: Mapping[str, Any],
    data: Optional[Mapping[str, Any]],
) -> Envelope:
    """Assemble the Envelope, applying the null-versus-empty rules of this module."""
    return Envelope(
        data=dict(data) if data is not None else {},
        event_id=event_id,
        event_type=event_type,
        occurred_at=occurred_at,
        recorded_at=recorded_at,
        resource=resource,
        schema_version=schema_version,
        severity=severity,
        actor=actor,
        attributes=dict(attributes) if attributes else None,
        context=context,
    )
