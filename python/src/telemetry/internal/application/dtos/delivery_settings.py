"""The delivery settings of the configuration."""

from dataclasses import dataclass

from telemetry.internal.domain.rules import (
    DEFAULT_BATCH_SIZE,
    DEFAULT_FLUSH_INTERVAL_MILLISECONDS,
    DEFAULT_QUEUE_SIZE,
    DEFAULT_TIMEOUT_MILLISECONDS,
)
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode


@dataclass(frozen=True, slots=True)
class DeliverySettings:
    """How events leave the process: the mode, the bounded queue, the batch, the flush interval and the timeout.

    The values are the ones already validated by the configuration provider; the intervals are in milliseconds.
    """

    mode: DeliveryMode = DeliveryMode.BEST_EFFORT
    queue_size: int = DEFAULT_QUEUE_SIZE
    batch_size: int = DEFAULT_BATCH_SIZE
    flush_interval_milliseconds: int = DEFAULT_FLUSH_INTERVAL_MILLISECONDS
    timeout_milliseconds: int = DEFAULT_TIMEOUT_MILLISECONDS
