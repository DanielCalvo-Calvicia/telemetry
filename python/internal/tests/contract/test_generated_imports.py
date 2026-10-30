"""Checks that the generated modules import with the standard library only and carry the values of the contract."""

import dataclasses
import unittest

from telemetry.internal.domain.value_objects import (
    enums_generated,
    envelope_generated,
    error_codes_generated,
    event_types_generated,
)


class GeneratedEnumsTest(unittest.TestCase):
    def test_the_six_severities(self):
        values = [severity.value for severity in enums_generated.Severity]
        self.assertEqual(values, ["TRACE", "DEBUG", "INFO", "WARN", "ERROR", "CRITICAL"])
        self.assertEqual(enums_generated.Severity.INFO, "INFO")

    def test_the_other_enums(self):
        self.assertEqual([kind.value for kind in enums_generated.ActorKind], ["device", "agent", "subsystem"])
        self.assertEqual(
            [category.value for category in enums_generated.FailureCategory],
            ["validation", "configuration", "internal"],
        )


class GeneratedRegistriesTest(unittest.TestCase):
    def test_the_registered_event_type(self):
        self.assertEqual(event_types_generated.EVENT_TYPE_LOG_ENTRY_RECORDED, "log.entry.recorded")
        self.assertEqual(event_types_generated.EVENT_TYPES, ("log.entry.recorded",))

    def test_the_nine_error_codes(self):
        catalog = error_codes_generated.ERROR_CODES
        self.assertEqual(
            sorted(catalog),
            [
                "config_field_invalid",
                "delivery_mode_unknown",
                "event_build_failed",
                "event_schema_invalid",
                "event_type_unknown",
                "instance_id_unavailable",
                "limit_exceeded",
                "limits_failed",
                "masking_failed",
            ],
        )
        info = catalog[error_codes_generated.ERROR_CODE_EVENT_SCHEMA_INVALID]
        self.assertEqual(info.category, enums_generated.FailureCategory.VALIDATION)
        self.assertTrue(info.default_message)
        with self.assertRaises(dataclasses.FrozenInstanceError):
            info.code = "other"


class GeneratedStructsTest(unittest.TestCase):
    def test_a_log_entry_recorded(self):
        data = envelope_generated.LogEntryRecorded(category="http", message="request received")
        self.assertIsNone(data.function)
        self.assertEqual(dataclasses.asdict(data)["message"], "request received")

    def test_an_envelope(self):
        envelope = envelope_generated.Envelope(
            data={"category": "http", "message": "request received"},
            event_id="01900000-0000-7000-8000-000000000000",
            event_type=event_types_generated.EVENT_TYPE_LOG_ENTRY_RECORDED,
            occurred_at="2026-10-28T10:00:00.000000Z",
            recorded_at="2026-10-28T10:00:00.000001Z",
            resource=envelope_generated.Resource(instance_id="instance-1", service_name="my-service"),
            schema_version="0.1",
            severity=enums_generated.Severity.INFO,
        )
        self.assertIsNone(envelope.actor)
        self.assertIsNone(envelope.attributes)
        self.assertEqual(envelope.resource.service_name, "my-service")
