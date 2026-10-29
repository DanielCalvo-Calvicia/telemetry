"""Black-box tests of the value objects over the shared tables, plus what the tables do not cover."""

import dataclasses
import datetime
import unittest

from internal.tests.domain.value_objects.helpers import load_cases
from telemetry.internal.domain import errors
from telemetry.internal.domain.value_objects.actor_identity import ActorIdentity
from telemetry.internal.domain.value_objects.actor_kind import parse_actor_kind
from telemetry.internal.domain.value_objects.correlation_context import CorrelationContext
from telemetry.internal.domain.value_objects.enums_generated import ActorKind, Severity
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity, validate_time_zone
from telemetry.internal.domain.value_objects.severity import parse_severity
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant

UTC = datetime.timezone.utc


def with_absent_fields(fields: dict, required_names: tuple) -> dict:
    """A fixture leaves a field out to mean absent; the constructors take None for it, so fill the gaps."""
    return {**{name: None for name in required_names}, **fields}


def build_utc_instant(text: str) -> UtcInstant:
    """Parse an ISO text with an explicit offset and wrap it."""
    return UtcInstant(datetime.datetime.fromisoformat(text))


class SharedTablesTest(unittest.TestCase):
    def check(self, file_name, build):
        """Every case with valid=true builds, every other raises a classified or domain error."""
        cases = load_cases(file_name)
        self.assertGreater(len(cases), 0)
        for case in cases:
            with self.subTest(file_name=file_name, case=case["description"]):
                if case["valid"]:
                    build(case["input"])
                else:
                    with self.assertRaises(errors.ClassifiedError):
                        build(case["input"])

    def test_severity(self):
        self.check("severity.json", parse_severity)

    def test_actor_kind(self):
        self.check("actor_kind.json", parse_actor_kind)

    def test_event_id(self):
        self.check("event_id.json", EventId)

    def test_utc_instant(self):
        self.check("utc_instant.json", build_utc_instant)
        for case in load_cases("utc_instant.json"):
            if case["valid"]:
                with self.subTest(case["description"]):
                    instant = datetime.datetime.fromisoformat(case["input"]).astimezone(UTC)
                    expected = instant.strftime("%Y-%m-%dT%H:%M:%S.") + f"{instant.microsecond:06d}Z"
                    self.assertEqual(build_utc_instant(case["input"]).format(), expected)

    def test_resource(self):
        self.check(
            "resource.json",
            lambda fields: ResourceIdentity(**with_absent_fields(fields, ("service_name", "instance_id"))),
        )

    def test_actor(self):
        self.check("actor.json", lambda fields: ActorIdentity(**with_absent_fields(fields, ("kind", "id"))))

    def test_context(self):
        self.check("context.json", lambda fields: CorrelationContext(**fields))

    def test_attributes(self):
        self.check("attributes.json", EventAttributes)


class EnumTest(unittest.TestCase):
    def test_parse_severity_covers_every_member(self):
        for member in Severity:
            with self.subTest(member=member):
                self.assertIs(parse_severity(member.value), member)

    def test_parse_actor_kind_covers_every_member(self):
        for member in ActorKind:
            with self.subTest(member=member):
                self.assertIs(parse_actor_kind(member.value), member)

    def test_the_error_classes(self):
        with self.assertRaises(errors.SeverityInvalidError):
            parse_severity("info")
        with self.assertRaises(errors.ActorKindInvalidError):
            parse_actor_kind("robot")


class EventIdTest(unittest.TestCase):
    def test_error_class_and_default_code(self):
        with self.assertRaises(errors.EventIdInvalidError) as raised:
            EventId("550e8400-e29b-41d4-a716-446655440000")
        self.assertEqual(raised.exception.code, errors.ERROR_CODE_EVENT_SCHEMA_INVALID)
        with self.assertRaises(errors.EventIdInvalidError):
            EventId(None)

    def test_a_trailing_newline_is_rejected(self):
        with self.assertRaises(errors.EventIdInvalidError):
            EventId("018f3c1e-7b2a-7c3d-9e4f-0123456789ab\n")

    def test_it_is_immutable(self):
        event_id = EventId("018f3c1e-7b2a-7c3d-9e4f-0123456789ab")
        self.assertEqual(str(event_id), "018f3c1e-7b2a-7c3d-9e4f-0123456789ab")
        with self.assertRaises(dataclasses.FrozenInstanceError):
            event_id.value = "x"


class UtcInstantTest(unittest.TestCase):
    def test_six_decimals_are_kept_with_trailing_zeros(self):
        instant = UtcInstant(datetime.datetime(2026, 10, 28, 10, 0, 1, 500000, UTC))
        self.assertEqual(instant.format(), "2026-10-28T10:00:01.500000Z")
        self.assertEqual(str(UtcInstant(datetime.datetime(2026, 1, 2, 3, 4, 5, 7, UTC))), "2026-01-02T03:04:05.000007Z")

    def test_the_value_is_the_datetime(self):
        moment = datetime.datetime(2026, 10, 28, 10, 0, 0, 0, UTC)
        self.assertEqual(UtcInstant(moment).value, moment)

    def test_a_naive_or_non_utc_datetime_is_rejected(self):
        madrid = datetime.timezone(datetime.timedelta(hours=2))
        for bad_value in (
            datetime.datetime(2026, 10, 28, 10, 0, 0),
            datetime.datetime(2026, 10, 28, 10, 0, 0, tzinfo=madrid),
            "2026-10-28T10:00:00Z",
        ):
            with self.subTest(bad_value=bad_value):
                with self.assertRaises(errors.NonUtcTimeError):
                    UtcInstant(bad_value)

    def test_it_is_immutable(self):
        instant = UtcInstant(datetime.datetime(2026, 10, 28, 10, 0, 0, 0, UTC))
        with self.assertRaises(dataclasses.FrozenInstanceError):
            instant.value = datetime.datetime(2027, 1, 1, tzinfo=UTC)


class ResourceIdentityTest(unittest.TestCase):
    def test_a_missing_required_field_is_reported_with_every_other_problem(self):
        with self.assertRaises(errors.ResourceFieldMissingError) as raised:
            ResourceIdentity(service_name=None, instance_id="i", host="", time_zone="a b")
        message = str(raised.exception)
        for part in ("service_name", "host", "time_zone"):
            self.assertIn(part, message)

    def test_an_invalid_present_field_has_its_own_class(self):
        with self.assertRaises(errors.ResourceFieldInvalidError):
            ResourceIdentity(service_name="s", instance_id="i", environment="")
        with self.assertRaises(errors.ResourceFieldInvalidError):
            ResourceIdentity(service_name="s", instance_id="i", time_zone="Europe Madrid")

    def test_the_time_zone_rule(self):
        self.assertEqual(validate_time_zone("America/Argentina/Buenos_Aires"), "America/Argentina/Buenos_Aires")
        for bad_value in ("", "Europe//Madrid", "/Madrid", "Europe/", None, "Europe/Madríd"):
            with self.subTest(bad_value=bad_value):
                with self.assertRaises(errors.TimeZoneInvalidError):
                    validate_time_zone(bad_value)

    def test_it_is_immutable(self):
        resource = ResourceIdentity(service_name="s", instance_id="i")
        self.assertIsNone(resource.time_zone)
        with self.assertRaises(dataclasses.FrozenInstanceError):
            resource.service_name = "other"


class ActorIdentityTest(unittest.TestCase):
    def test_the_kind_is_kept_as_the_enum(self):
        self.assertIs(ActorIdentity(kind="agent", id="a").kind, ActorKind.AGENT)
        self.assertIs(ActorIdentity(kind=ActorKind.DEVICE, id="a").kind, ActorKind.DEVICE)

    def test_the_error_class_and_every_failed_field(self):
        with self.assertRaises(errors.ActorFieldInvalidError) as raised:
            ActorIdentity(kind="robot", id=None, type="", version="")
        message = str(raised.exception)
        for part in ("kind", "id", "type", "version"):
            self.assertIn(part, message)

    def test_it_is_immutable(self):
        actor = ActorIdentity(kind="device", id="d")
        with self.assertRaises(dataclasses.FrozenInstanceError):
            actor.id = "other"


class CorrelationContextTest(unittest.TestCase):
    def test_the_empty_context_is_valid(self):
        self.assertEqual(CorrelationContext(), CorrelationContext(None, None, None, None))

    def test_the_error_class_and_every_failed_field(self):
        with self.assertRaises(errors.ContextFieldInvalidError) as raised:
            CorrelationContext(trace_id="AB", span_id="", execution_id="", correlation_id="\t")
        message = str(raised.exception)
        for part in ("trace_id", "span_id", "execution_id", "correlation_id"):
            self.assertIn(part, message)

    def test_it_is_immutable(self):
        context = CorrelationContext(execution_id="run")
        with self.assertRaises(dataclasses.FrozenInstanceError):
            context.execution_id = "other"


class EventAttributesTest(unittest.TestCase):
    def test_a_boolean_is_a_scalar_and_stays_a_boolean(self):
        attributes = EventAttributes({"flag": True, "count": 1})
        self.assertIs(attributes.entries["flag"], True)
        self.assertEqual(attributes.entries["count"], 1)

    def test_nan_and_infinity_are_not_special(self):
        EventAttributes({"a": float("nan"), "b": float("inf")})

    def test_the_error_classes(self):
        with self.assertRaises(errors.AttributeKeyInvalidError):
            EventAttributes({"": 1})
        with self.assertRaises(errors.AttributeKeyInvalidError):
            EventAttributes({1: "a"})
        with self.assertRaises(errors.AttributeKeyInvalidError):
            EventAttributes(None)
        with self.assertRaises(errors.AttributeValueInvalidError):
            EventAttributes({"nested": {"a": 1}})

    def test_every_failure_is_in_one_message_and_keys_win_the_class(self):
        with self.assertRaises(errors.AttributeKeyInvalidError) as raised:
            EventAttributes({"": 1, "none": None})
        self.assertIn("'none'", str(raised.exception))

    def test_changing_the_input_does_not_change_the_value_object(self):
        source = {"tags": ["a"], "name": "n"}
        attributes = EventAttributes(source)
        source["name"] = "changed"
        source["tags"].append("b")
        source["added"] = 1
        self.assertEqual(attributes.as_dict(), {"tags": ["a"], "name": "n"})

    def test_the_accessors_give_read_only_or_copied_data(self):
        attributes = EventAttributes({"tags": ["a"], "name": "n"})
        with self.assertRaises(TypeError):
            attributes.entries["name"] = "changed"
        self.assertEqual(attributes.entries["tags"], ("a",))
        copied = attributes.as_dict()
        copied["name"] = "changed"
        copied["tags"].append("b")
        self.assertEqual(attributes.as_dict(), {"tags": ["a"], "name": "n"})

    def test_assigning_an_attribute_raises(self):
        with self.assertRaises(dataclasses.FrozenInstanceError):
            EventAttributes({}).entries = {}

    def test_equality_and_hash(self):
        left = EventAttributes({"tags": ["a"]})
        self.assertEqual(left, EventAttributes({"tags": ("a",)}))
        self.assertEqual(hash(left), hash(EventAttributes({"tags": ["a"]})))
        self.assertNotEqual(left, EventAttributes({"tags": ["b"]}))
        self.assertNotEqual(left, {"tags": ["a"]})


if __name__ == "__main__":
    unittest.main()
