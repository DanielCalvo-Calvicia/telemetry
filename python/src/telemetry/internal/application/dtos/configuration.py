"""The configuration the consumer passed to the core."""

from dataclasses import dataclass

from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity


@dataclass(frozen=True, slots=True)
class Configuration:
    """Configuration is what the consumer passed in; T-03.6 completes it with the rest of the settings."""

    resource: ResourceIdentity
