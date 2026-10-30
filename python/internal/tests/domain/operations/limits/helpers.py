"""Helpers of the limits tests: a small envelope and the limits of a fixture."""

import copy
from typing import Any, Optional

from internal.tests.application.services.masking_helpers import envelope_from_json
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.limits import Limits

BASE_EVENT: dict[str, Any] = {
    "schema_version": "0.1",
    "event_id": "01900000-0000-7000-8000-000000000001",
    "event_type": "log.entry.recorded",
    "occurred_at": "2026-10-28T10:00:00.123456Z",
    "recorded_at": "2026-10-28T10:00:00.123456Z",
    "severity": "INFO",
    "resource": {"service_name": "sensor-hub", "instance_id": "hub-01"},
    "data": {"category": "c", "message": "m"},
}


def make_envelope(attributes: Optional[dict[str, Any]] = None, message: str = "m") -> Envelope:
    """Build an envelope with the given attributes and message."""
    event = copy.deepcopy(BASE_EVENT)
    event["data"]["message"] = message
    if attributes is not None:
        event["attributes"] = attributes
    return envelope_from_json(event)


def limits_from_json(options: dict[str, Any]) -> Limits:
    """Create the limits of a fixture; an absent key is the default. It raises a ClassifiedError if invalid."""
    return Limits(**options)
