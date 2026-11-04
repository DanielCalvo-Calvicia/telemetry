"""The dependencies of the system random adapter."""

from telemetry.internal.application.ports.outbound.random_port import RandomPort
from telemetry.internal.infrastructure.outbound.shared.system_random.system_random import SystemRandom


def new_system_random() -> RandomPort:
    """Create the system random source, the RandomPort of the delivery."""
    return SystemRandom()
