"""The id generator port."""

from typing import Protocol, runtime_checkable

from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.value_objects.event_id import EventId


@runtime_checkable
class IDGeneratorPort(Protocol):
    """IDGeneratorPort is what the application requires to create ids. Both may raise; the service handles it."""

    def new_event_id(self) -> EventId:
        """Return a new event id, a UUIDv7."""

    def new_instance_id(self) -> Identifier:
        """Return the instance id of a resource when the consumer gives none."""
