"""The names under which a consumer can load each setting of the HTTP destination from its own source.

The keys are the property names of contracts/config/http_client.schema.json followed by the one of
contracts/config/http_destination.schema.json, and the values are their ``x-env`` names. There are NO default values
here on purpose (the defaults live in settings.py) and the table is read only. This package never reads them (ADR-004).
"""

from types import MappingProxyType
from typing import Mapping

from telemetry.internal.infrastructure.outbound.shared.http_client.env_names import HTTP_ENVIRONMENT_VARIABLES

HTTP_DESTINATION_ENVIRONMENT_VARIABLES: Mapping[str, str] = MappingProxyType(
    {
        **HTTP_ENVIRONMENT_VARIABLES,
        "minimum_severity": "TELEMETRY_HTTP_MINIMUM_SEVERITY",
    }
)
