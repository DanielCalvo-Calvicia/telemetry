"""The actor of an event: who or what acted."""

from dataclasses import dataclass
from typing import Optional

from telemetry.internal.domain.errors import ActorFieldInvalidError, ActorKindInvalidError
from telemetry.internal.domain.value_objects.actor_kind import parse_actor_kind
from telemetry.internal.domain.value_objects.enums_generated import ActorKind
from telemetry.internal.domain.value_objects.field_problems import FieldProblems


@dataclass(frozen=True, slots=True)
class ActorIdentity:
    """A known kind and an id are required; type and version are optional identifiers (``None`` is absent).

    ``kind`` is an ActorKind or the text of one; it is kept as an ActorKind. Every failed field is reported in one
    ActorFieldInvalidError.
    """

    kind: ActorKind
    id: str
    type: Optional[str] = None
    version: Optional[str] = None

    def __post_init__(self) -> None:
        """Validate every field and report all the failures together."""
        problems = FieldProblems()
        try:
            object.__setattr__(self, "kind", parse_actor_kind(self.kind))
        except ActorKindInvalidError as error:
            problems.add_invalid("kind", error)
        problems.identifier("id", self.id, required=True)
        problems.identifier("type", self.type, required=False)
        problems.identifier("version", self.version, required=False)
        problems.raise_if_any("the actor", ActorFieldInvalidError, ActorFieldInvalidError)
