"""The dependencies of the system clock adapter."""

from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.infrastructure.outbound.shared.system_clock.system_clock import SystemClock


def new_system_clock() -> ClockPort:
    """Create the system clock, the default ClockPort."""
    return SystemClock()
