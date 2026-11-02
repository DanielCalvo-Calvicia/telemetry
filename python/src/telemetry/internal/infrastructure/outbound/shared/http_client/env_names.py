"""The names under which a consumer can load each setting of the shared HTTP client from its own source.

The keys are the property names of contracts/config/http_client.schema.json, in its order, and the values are its
``x-env`` names. There are NO default values here on purpose (the defaults live in settings.py) and the table is read
only. This package never reads them (ADR-004).
"""

from types import MappingProxyType
from typing import Mapping

HTTP_ENVIRONMENT_VARIABLES: Mapping[str, str] = MappingProxyType(
    {
        "url": "TELEMETRY_HTTP_URL",
        "method": "TELEMETRY_HTTP_METHOD",
        "timeout_ms": "TELEMETRY_HTTP_TIMEOUT_MS",
        "compress": "TELEMETRY_HTTP_COMPRESS",
        "skip_tls_verification": "TELEMETRY_HTTP_SKIP_TLS_VERIFICATION",
        "bearer_token": "TELEMETRY_HTTP_BEARER_TOKEN",
        "api_key_header": "TELEMETRY_HTTP_API_KEY_HEADER",
        "api_key_value": "TELEMETRY_HTTP_API_KEY_VALUE",
        "max_attempts": "TELEMETRY_HTTP_MAX_ATTEMPTS",
        "retry_base_delay_ms": "TELEMETRY_HTTP_RETRY_BASE_DELAY_MS",
        "retry_max_delay_ms": "TELEMETRY_HTTP_RETRY_MAX_DELAY_MS",
    }
)
