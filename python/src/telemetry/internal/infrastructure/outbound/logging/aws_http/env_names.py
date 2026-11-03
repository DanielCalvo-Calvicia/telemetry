"""The names under which a consumer can load each setting of the AWS HTTP adapter from its own source.

The keys are the property names of contracts/config/aws_http.schema.json, in its order, and the values are its
``x-env`` names. There are NO default values here on purpose (the defaults live in settings.py) and the table is read
only. This package never reads them (ADR-004).
"""

from types import MappingProxyType
from typing import Mapping

AWS_HTTP_ENVIRONMENT_VARIABLES: Mapping[str, str] = MappingProxyType(
    {
        "host": "TELEMETRY_AWS_HTTP_HOST",
        "port": "TELEMETRY_AWS_HTTP_PORT",
        "path": "TELEMETRY_AWS_HTTP_PATH",
        "log_group_name": "TELEMETRY_AWS_HTTP_LOG_GROUP_NAME",
        "log_stream_name": "TELEMETRY_AWS_HTTP_LOG_STREAM_NAME",
        "timeout_ms": "TELEMETRY_AWS_HTTP_TIMEOUT_MS",
        "skip_tls_verification": "TELEMETRY_AWS_HTTP_SKIP_TLS_VERIFICATION",
        "bearer_token": "TELEMETRY_AWS_HTTP_BEARER_TOKEN",
        "api_key_header": "TELEMETRY_AWS_HTTP_API_KEY_HEADER",
        "api_key_value": "TELEMETRY_AWS_HTTP_API_KEY_VALUE",
        "max_attempts": "TELEMETRY_AWS_HTTP_MAX_ATTEMPTS",
        "retry_base_delay_ms": "TELEMETRY_AWS_HTTP_RETRY_BASE_DELAY_MS",
        "retry_max_delay_ms": "TELEMETRY_AWS_HTTP_RETRY_MAX_DELAY_MS",
    }
)
