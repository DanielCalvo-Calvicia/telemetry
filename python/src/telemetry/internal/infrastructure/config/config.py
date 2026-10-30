"""The configuration model: the Python form of contracts/config/config.schema.json.

The consumer loads these values with its own tool, using the names of ``ENVIRONMENT_VARIABLES``, and passes them in.
THIS PACKAGE NEVER READS THE ENVIRONMENT OR A FILE (ADR-004, REQ-ARC-007). The attribute names are exactly the
property names of the schema, so ``dataclasses.asdict(config)`` has the shape of the schema. An empty text means
'not set'. No field is a secret (REQ-CFG-005).
"""

from dataclasses import dataclass, field
from typing import Mapping, Optional

from telemetry.internal.domain.rules import DEFAULT_SPOOL_QUOTA_BYTES


@dataclass(frozen=True)
class ResourceConfig:
    """The identity of the emitting process; an empty text is 'not set' (an empty instance_id is generated)."""

    service_name: str = ""
    instance_id: str = ""
    environment: str = ""
    service_version: str = ""
    host: str = ""
    time_zone: str = ""


@dataclass(frozen=True)
class DeliveryConfig:
    """How events leave the process; ``mode`` is the text best_effort, retry or durable."""

    mode: str
    queue_size: int
    batch_size: int
    flush_interval_ms: int
    timeout_ms: int


@dataclass(frozen=True)
class LimitsConfig:
    """The size limits of an event and of a batch."""

    max_attribute_keys: int
    max_attribute_key_chars: int
    max_attribute_value_chars: int
    max_event_bytes: int
    max_batch_bytes: int


@dataclass(frozen=True)
class SpoolConfig:
    """The local spool of the durable mode; the directory is required then and has no default."""

    directory: str = ""
    quota_bytes: int = DEFAULT_SPOOL_QUOTA_BYTES


@dataclass(frozen=True)
class MaskingConfig:
    """The masking options. ``field_names`` None means 'not set' (the default list); any tuple, even empty, REPLACES."""

    extra_patterns: tuple[str, ...] = ()
    field_names: Optional[tuple[str, ...]] = None
    extra_field_names: tuple[str, ...] = ()
    field_classes: Mapping[str, str] = field(default_factory=dict)
    class_rules: Mapping[str, str] = field(default_factory=dict)


@dataclass(frozen=True)
class Config:
    """The whole configuration: one section per object of the schema."""

    resource: ResourceConfig
    delivery: DeliveryConfig
    limits: LimitsConfig
    spool: SpoolConfig
    masking: MaskingConfig
