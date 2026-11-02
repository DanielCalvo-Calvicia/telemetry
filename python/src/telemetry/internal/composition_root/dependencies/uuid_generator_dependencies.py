"""The dependencies of the UUID generator adapter."""

from telemetry.internal.application.ports.outbound.id_generator_port import IDGeneratorPort
from telemetry.internal.infrastructure.outbound.shared.uuid_generator.uuid_generator import UuidGenerator


def new_uuid_generator() -> IDGeneratorPort:
    """Create the UUIDv7 generator, the default IDGeneratorPort."""
    return UuidGenerator()
