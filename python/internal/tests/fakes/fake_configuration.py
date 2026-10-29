"""A fake of the configuration port."""

import threading
from typing import Optional

from telemetry.internal.application.dtos.configuration import Configuration
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity


class FakeConfiguration:
    """FakeConfiguration hands out a settable configuration and counts the calls."""

    def __init__(self, configuration: Optional[Configuration] = None) -> None:
        self._lock = threading.Lock()
        default = Configuration(resource=ResourceIdentity(service_name="fake-service", instance_id="fake-instance"))
        self._configuration = configuration if configuration is not None else default
        self._failure: Optional[Exception] = None
        self.calls = 0

    def fail_with(self, error: Optional[Exception]) -> None:
        """Make configuration() raise this error; None clears it."""
        with self._lock:
            self._failure = error

    def set_configuration(self, configuration: Configuration) -> None:
        """Set what configuration() returns."""
        with self._lock:
            self._configuration = configuration

    def configuration(self) -> Configuration:
        """Return the settable configuration."""
        with self._lock:
            self.calls += 1
            if self._failure is not None:
                raise self._failure
            return self._configuration
