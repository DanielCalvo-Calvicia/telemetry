"""The defaults of the configuration, in one place."""

from telemetry.internal.domain.rules import (
    DEFAULT_BATCH_SIZE,
    DEFAULT_FLUSH_INTERVAL_MILLISECONDS,
    DEFAULT_MAX_ATTRIBUTE_KEY_CHARS,
    DEFAULT_MAX_ATTRIBUTE_KEYS,
    DEFAULT_MAX_ATTRIBUTE_VALUE_CHARS,
    DEFAULT_MAX_BATCH_BYTES,
    DEFAULT_MAX_EVENT_BYTES,
    DEFAULT_QUEUE_SIZE,
    DEFAULT_SPOOL_QUOTA_BYTES,
    DEFAULT_TIMEOUT_MILLISECONDS,
)
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode
from telemetry.internal.infrastructure.config.config import (
    Config,
    DeliveryConfig,
    LimitsConfig,
    MaskingConfig,
    ResourceConfig,
    SpoolConfig,
)


def defaults() -> Config:
    """Return the default configuration: the delivery mode best_effort, the defaults of PR-001 and nothing else set.

    It is not valid by itself: the service name is required and the consumer gives it.
    """
    return Config(
        resource=ResourceConfig(),
        delivery=DeliveryConfig(
            mode=DeliveryMode.BEST_EFFORT.value,
            queue_size=DEFAULT_QUEUE_SIZE,
            batch_size=DEFAULT_BATCH_SIZE,
            flush_interval_ms=DEFAULT_FLUSH_INTERVAL_MILLISECONDS,
            timeout_ms=DEFAULT_TIMEOUT_MILLISECONDS,
        ),
        limits=LimitsConfig(
            max_attribute_keys=DEFAULT_MAX_ATTRIBUTE_KEYS,
            max_attribute_key_chars=DEFAULT_MAX_ATTRIBUTE_KEY_CHARS,
            max_attribute_value_chars=DEFAULT_MAX_ATTRIBUTE_VALUE_CHARS,
            max_event_bytes=DEFAULT_MAX_EVENT_BYTES,
            max_batch_bytes=DEFAULT_MAX_BATCH_BYTES,
        ),
        spool=SpoolConfig(quota_bytes=DEFAULT_SPOOL_QUOTA_BYTES),
        masking=MaskingConfig(),
    )
