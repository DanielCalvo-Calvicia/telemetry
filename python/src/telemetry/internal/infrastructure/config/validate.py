"""The validation of a configuration at start (REQ-CFG-002).

All the problems are reported together, in the order of the properties of contracts/config/config.schema.json. A reason
is a fixed text: it never holds a configured value (a configuration can hold anything the consumer loaded).
"""

from typing import Mapping

from telemetry.internal.domain.base_value_objects.errors import ValueObjectError
from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.errors import ClassifiedError, DeliveryModeUnknownError, TimeZoneInvalidError
from telemetry.internal.domain.operations.masking.sensitive_data_masker import MaskerOptions, SensitiveDataMasker
from telemetry.internal.domain.rules import MASK_RULES
from telemetry.internal.domain.value_objects.delivery_mode import parse_delivery_mode
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode
from telemetry.internal.domain.value_objects.error_codes_generated import (
    ERROR_CODE_CONFIG_FIELD_INVALID,
    ERROR_CODE_DELIVERY_MODE_UNKNOWN,
)
from telemetry.internal.domain.value_objects.resource_identity import validate_time_zone
from telemetry.internal.infrastructure.config.config import Config
from telemetry.internal.infrastructure.config.errors import ConfigProblem

_LIMIT_FIELDS = (
    "max_attribute_keys",
    "max_attribute_key_chars",
    "max_attribute_value_chars",
    "max_event_bytes",
    "max_batch_bytes",
)


def _positive(value: object) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value > 0


def _identifier(value: object) -> bool:
    try:
        Identifier(value)  # type: ignore[arg-type]
    except ValueObjectError:
        return False
    return True


def _time_zone(value: object) -> bool:
    try:
        validate_time_zone(value)  # type: ignore[arg-type]
    except TimeZoneInvalidError:
        return False
    return True


def _delivery_mode(value: object) -> bool:
    try:
        parse_delivery_mode(value)  # type: ignore[arg-type]
    except DeliveryModeUnknownError:
        return False
    return True


def _patterns_compile(patterns: object) -> bool:
    if not isinstance(patterns, (tuple, list)) or not all(isinstance(pattern, str) for pattern in patterns):
        return False
    try:
        SensitiveDataMasker(MaskerOptions(extra_patterns=tuple(patterns)))
    except ClassifiedError:
        return False
    return True


def _rules_known(class_rules: object) -> bool:
    return isinstance(class_rules, Mapping) and all(rule in MASK_RULES for rule in class_rules.values())


def validate_config(config: Config) -> tuple[ConfigProblem, ...]:
    """Return every problem of the configuration, in the order of the schema; an empty tuple means it is valid."""
    problems: list[ConfigProblem] = []

    def invalid(field_name: str, reason: str) -> None:
        problems.append(ConfigProblem(field_name, ERROR_CODE_CONFIG_FIELD_INVALID, reason))

    resource = config.resource
    if not _identifier(resource.service_name):
        invalid("resource.service_name", "the service name is required: text with a visible character")
    for field_name in ("instance_id", "environment", "service_version", "host"):
        value = getattr(resource, field_name)
        if value != "" and not _identifier(value):
            invalid(f"resource.{field_name}", "when it is set it must be text with no control characters")
    if resource.time_zone != "" and not _time_zone(resource.time_zone):
        invalid("resource.time_zone", "when it is set it must look like an IANA name, for example Europe/Madrid")

    delivery = config.delivery
    mode_valid = _delivery_mode(delivery.mode)
    if not mode_valid:
        problems.append(
            ConfigProblem(
                "delivery.mode",
                ERROR_CODE_DELIVERY_MODE_UNKNOWN,
                "the delivery mode must be best_effort, retry or durable",
            )
        )
    queue_valid = _positive(delivery.queue_size)
    if not queue_valid:
        invalid("delivery.queue_size", "it must be an integer greater than zero")
    if not _positive(delivery.batch_size):
        invalid("delivery.batch_size", "it must be an integer greater than zero")
    elif queue_valid and delivery.batch_size > delivery.queue_size:
        invalid("delivery.batch_size", "it must not be greater than the queue size")
    if not _positive(delivery.flush_interval_ms):
        invalid("delivery.flush_interval_ms", "it must be an integer greater than zero")
    if not _positive(delivery.timeout_ms):
        invalid("delivery.timeout_ms", "it must be an integer greater than zero")

    limits = config.limits
    limits_valid = {}
    for field_name in _LIMIT_FIELDS:
        limits_valid[field_name] = _positive(getattr(limits, field_name))
        if not limits_valid[field_name]:
            invalid(f"limits.{field_name}", "it must be an integer greater than zero")
    if (
        limits_valid["max_event_bytes"]
        and limits_valid["max_batch_bytes"]
        and limits.max_batch_bytes < limits.max_event_bytes
    ):
        invalid("limits.max_batch_bytes", "it must not be less than the event limit")

    spool = config.spool
    if mode_valid and parse_delivery_mode(delivery.mode) is DeliveryMode.DURABLE:
        if not isinstance(spool.directory, str) or spool.directory == "":
            invalid("spool.directory", "it is required when the delivery mode is durable")
    if not _positive(spool.quota_bytes):
        invalid("spool.quota_bytes", "it must be an integer greater than zero")

    masking = config.masking
    if not _patterns_compile(masking.extra_patterns):
        invalid("masking.extra_patterns", "every extra pattern must be a valid regular expression")
    if not _rules_known(masking.class_rules):
        invalid("masking.class_rules", "every class rule must be drop, redact or allow")

    return tuple(problems)
