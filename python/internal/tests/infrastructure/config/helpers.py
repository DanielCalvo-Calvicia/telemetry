"""Helpers of the configuration tests: the profiles by name and the building of a Config from a merged mapping."""

import dataclasses

from telemetry.internal.infrastructure.config import profiles
from telemetry.internal.infrastructure.config.config import (
    Config,
    DeliveryConfig,
    LimitsConfig,
    MaskingConfig,
    ResourceConfig,
    SpoolConfig,
)

PROFILES = {
    "microservice": profiles.microservice,
    "cronjob": profiles.cronjob,
    "worker": profiles.worker,
    "robotics": profiles.robotics,
    "offline_device": profiles.offline_device,
}
MAP_FIELDS = ("field_classes", "class_rules")


def merged_config(profile_name: str, overrides: dict) -> Config:
    """The profile with the overrides on top: sections and the two maps merge key by key, the rest replaces."""
    merged = dataclasses.asdict(PROFILES[profile_name]())
    for section_name, section_overrides in overrides.items():
        section = merged[section_name]
        for field_name, value in section_overrides.items():
            if field_name in MAP_FIELDS:
                section[field_name] = {**section[field_name], **value}
            elif isinstance(value, list):
                section[field_name] = tuple(value)
            else:
                section[field_name] = value
    return Config(
        resource=ResourceConfig(**merged["resource"]),
        delivery=DeliveryConfig(**merged["delivery"]),
        limits=LimitsConfig(**merged["limits"]),
        spool=SpoolConfig(**merged["spool"]),
        masking=MaskingConfig(**merged["masking"]),
    )
