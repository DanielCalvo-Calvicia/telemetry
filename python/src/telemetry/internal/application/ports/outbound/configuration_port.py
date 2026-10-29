"""The configuration port."""

from typing import Protocol, runtime_checkable

from telemetry.internal.application.dtos.configuration import Configuration


@runtime_checkable
class ConfigurationPort(Protocol):
    """ConfigurationPort hands the core the configuration the consumer passed in; it reads no file and no environment.

    The library never reads the environment (REQ-ARC-007, ADR-004): the consumer loads its configuration.
    """

    def configuration(self) -> Configuration:
        """Return the configuration the consumer passed in."""
