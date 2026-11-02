"""The names under which a consumer can load each setting of the console destination from its own source.

The keys are the property names of contracts/config/console.schema.json, in its order, and the values are its
``x-env`` names. There are NO default values here on purpose (the defaults live in settings.py) and the table is read
only. This package never reads them (ADR-004).
"""

from types import MappingProxyType
from typing import Mapping

CONSOLE_ENVIRONMENT_VARIABLES: Mapping[str, str] = MappingProxyType(
    {
        "format": "TELEMETRY_CONSOLE_FORMAT",
        "minimum_severity": "TELEMETRY_CONSOLE_MINIMUM_SEVERITY",
    }
)
