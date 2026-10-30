"""The base profiles (REQ-CFG-004): functions that return a configuration.

A profile is only defaults: the delivery mode. The consumer overrides fields afterwards (``dataclasses.replace``).
The precedence, lowest to highest, is the defaults, then the profile, then the options given in code (REQ-CFG-003).
"""

import dataclasses

from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode
from telemetry.internal.infrastructure.config.config import Config
from telemetry.internal.infrastructure.config.defaults import defaults


def _with_mode(mode: DeliveryMode) -> Config:
    base = defaults()
    return dataclasses.replace(base, delivery=dataclasses.replace(base.delivery, mode=mode.value))


def microservice() -> Config:
    """The profile of a microservice: the delivery mode best_effort."""
    return _with_mode(DeliveryMode.BEST_EFFORT)


def cronjob() -> Config:
    """The profile of a cron job: the delivery mode retry."""
    return _with_mode(DeliveryMode.RETRY)


def worker() -> Config:
    """The profile of a worker: the delivery mode retry."""
    return _with_mode(DeliveryMode.RETRY)


def robotics() -> Config:
    """The profile of a robot: the delivery mode durable (it needs a spool directory)."""
    return _with_mode(DeliveryMode.DURABLE)


def offline_device() -> Config:
    """The profile of a device that is often offline: the delivery mode durable (it needs a spool directory)."""
    return _with_mode(DeliveryMode.DURABLE)
