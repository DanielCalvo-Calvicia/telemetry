"""The delivery settings of the configuration."""

from dataclasses import dataclass, field
from typing import Mapping

from telemetry.internal.domain.rules import (
    DEFAULT_BATCH_SIZE,
    DEFAULT_BLOCK_TIMEOUT_MILLISECONDS,
    DEFAULT_FLUSH_INTERVAL_MILLISECONDS,
    DEFAULT_QUEUE_SIZE,
    DEFAULT_RETRY_BASE_DELAY_MILLISECONDS,
    DEFAULT_RETRY_MAX_ATTEMPTS,
    DEFAULT_RETRY_MAX_DELAY_MILLISECONDS,
    DEFAULT_TIMEOUT_MILLISECONDS,
)
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode, EventClass, QueuePolicy


@dataclass(frozen=True, slots=True)
class DeliverySettings:
    """How events leave the process: the mode, the bounded queue, the batch, the intervals and the full-queue rules.

    The values are the ones already validated by the configuration provider; the intervals are in milliseconds.
    ``queue_policy`` is what a full queue does, ``block_timeout_milliseconds`` how long a call may wait for room with
    ``block_with_timeout`` and ``event_classes`` the class assigned to an event type (an event type that is not in it
    gets the default of ``classify_event``). With the mode ``retry`` a transient failure is sent again, at most
    ``retry_max_attempts`` attempts for a batch (the first included), after a backoff with jitter that starts at
    ``retry_base_delay_milliseconds`` and never passes ``retry_max_delay_milliseconds``; ``best_effort`` sends once.
    """

    mode: DeliveryMode = DeliveryMode.BEST_EFFORT
    queue_size: int = DEFAULT_QUEUE_SIZE
    batch_size: int = DEFAULT_BATCH_SIZE
    flush_interval_milliseconds: int = DEFAULT_FLUSH_INTERVAL_MILLISECONDS
    timeout_milliseconds: int = DEFAULT_TIMEOUT_MILLISECONDS
    queue_policy: QueuePolicy = QueuePolicy.DROP_NEWEST
    block_timeout_milliseconds: int = DEFAULT_BLOCK_TIMEOUT_MILLISECONDS
    event_classes: Mapping[str, EventClass] = field(default_factory=dict)
    retry_max_attempts: int = DEFAULT_RETRY_MAX_ATTEMPTS
    retry_base_delay_milliseconds: int = DEFAULT_RETRY_BASE_DELAY_MILLISECONDS
    retry_max_delay_milliseconds: int = DEFAULT_RETRY_MAX_DELAY_MILLISECONDS
