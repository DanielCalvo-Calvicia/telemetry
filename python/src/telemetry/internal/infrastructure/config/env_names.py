"""The names under which a consumer can load each configuration value from its own source.

The keys are the dotted paths of the properties of contracts/config/config.schema.json and the values are its
``x-env`` names. There are NO default values here on purpose: a default would override the profile when the consumer
loads the values after choosing the profile. The defaults live in one place, defaults.py. This package never reads
them (ADR-004).
"""

from typing import Mapping

ENVIRONMENT_VARIABLES: Mapping[str, str] = {
    "resource.service_name": "TELEMETRY_SERVICE_NAME",
    "resource.instance_id": "TELEMETRY_INSTANCE_ID",
    "resource.environment": "TELEMETRY_ENVIRONMENT",
    "resource.service_version": "TELEMETRY_SERVICE_VERSION",
    "resource.host": "TELEMETRY_HOST",
    "resource.time_zone": "TELEMETRY_TIME_ZONE",
    "delivery.mode": "TELEMETRY_DELIVERY_MODE",
    "delivery.queue_size": "TELEMETRY_QUEUE_SIZE",
    "delivery.batch_size": "TELEMETRY_BATCH_SIZE",
    "delivery.flush_interval_ms": "TELEMETRY_FLUSH_INTERVAL_MS",
    "delivery.timeout_ms": "TELEMETRY_TIMEOUT_MS",
    "limits.max_attribute_keys": "TELEMETRY_MAX_ATTRIBUTE_KEYS",
    "limits.max_attribute_key_chars": "TELEMETRY_MAX_ATTRIBUTE_KEY_CHARS",
    "limits.max_attribute_value_chars": "TELEMETRY_MAX_ATTRIBUTE_VALUE_CHARS",
    "limits.max_event_bytes": "TELEMETRY_MAX_EVENT_BYTES",
    "limits.max_batch_bytes": "TELEMETRY_MAX_BATCH_BYTES",
    "spool.directory": "TELEMETRY_SPOOL_DIRECTORY",
    "spool.quota_bytes": "TELEMETRY_SPOOL_QUOTA_BYTES",
    "masking.extra_patterns": "TELEMETRY_MASK_EXTRA_PATTERNS",
    "masking.field_names": "TELEMETRY_MASK_FIELD_NAMES",
    "masking.extra_field_names": "TELEMETRY_MASK_EXTRA_FIELD_NAMES",
    "masking.field_classes": "TELEMETRY_MASK_FIELD_CLASSES",
    "masking.class_rules": "TELEMETRY_MASK_CLASS_RULES",
}
