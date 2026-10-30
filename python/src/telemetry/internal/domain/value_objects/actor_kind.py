"""The kind of an actor: the generated enum, and the parsing of a text into it."""

from telemetry.internal.domain.errors import ActorKindInvalidError
from telemetry.internal.domain.value_objects.enums_generated import ActorKind


def parse_actor_kind(text: str) -> ActorKind:
    """Return the ActorKind whose value is exactly ``text``; raise ActorKindInvalidError otherwise."""
    for member in ActorKind:
        if member.value == text:
            return member
    raise ActorKindInvalidError("the actor kind is not one of the values of the contract")
