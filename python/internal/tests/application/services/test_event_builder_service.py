"""Black-box tests of EventBuilderService: the shared building cases and the rules the cases do not show."""

import datetime
import json
import pathlib
import unittest
from typing import Any, Optional

from internal.tests.fakes import FakeClock, FakeConfiguration, FakeIdGenerator
from telemetry.internal.application.dtos.build_result import BuildResult
from telemetry.internal.application.dtos.clock_info import ClockInfo
from telemetry.internal.application.dtos.configuration import Configuration
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.errors import EventBuildFailedError
from telemetry.internal.application.services.event_builder_service import EventBuilderService
from telemetry.internal.domain.errors import ClassifiedError, EventSchemaInvalidError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.event_validation.event_validation import validate_event
from telemetry.internal.domain.value_objects.actor_identity import ActorIdentity
from telemetry.internal.domain.value_objects.correlation_context import CorrelationContext
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[5]
BUILDING_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "building"

EVENT_ID = "01900000-0000-7000-8000-000000000001"
WALL_TIME = datetime.datetime(2026, 10, 28, 10, 0, 0, 123456, tzinfo=datetime.timezone.utc)
LOG_DATA = {"category": "http", "message": "started"}
LOG_EVENT = "log.entry.recorded"


def permissive_validator(event_type: str, data: Any) -> Optional[Exception]:
    """A validator that accepts everything, to test the builder alone."""
    return None


def instant_of(text: str) -> UtcInstant:
    """Read an ISO text with an explicit offset as an instant."""
    return UtcInstant(datetime.datetime.fromisoformat(text))


def make_service(
    validator=validate_event, clock_info: Optional[ClockInfo] = None
) -> tuple[EventBuilderService, FakeClock, FakeIdGenerator, FakeConfiguration]:
    """A service over fakes with the fixed time and id."""
    clock = FakeClock(wall_time=WALL_TIME)
    if clock_info is not None:
        clock.set_clock_info(clock_info)
    id_generator = FakeIdGenerator([EventId(EVENT_ID)])
    configuration = FakeConfiguration(
        Configuration(resource=ResourceIdentity(service_name="sensor-hub", instance_id="hub-01"))
    )
    return EventBuilderService(clock, id_generator, configuration, validator), clock, id_generator, configuration


def log_input(**overrides: Any) -> NewEventInput:
    """A valid log input with the given fields replaced."""
    fields: dict[str, Any] = {"event_type": LOG_EVENT, "severity": Severity.INFO, "data": dict(LOG_DATA)}
    fields.update(overrides)
    return NewEventInput(**fields)


def case_input(raw: dict) -> NewEventInput:
    """Build the input of a shared case."""
    fields: dict[str, Any] = {
        "event_type": raw["event_type"],
        "severity": Severity(raw["severity"]),
        "data": raw.get("data"),
    }
    if "occurred_at" in raw:
        fields["occurred_at"] = instant_of(raw["occurred_at"])
    if "actor" in raw:
        fields["actor"] = ActorIdentity(**raw["actor"])
    if "context" in raw:
        fields["context"] = CorrelationContext(**raw["context"])
    if "attributes" in raw:
        fields["attributes"] = EventAttributes(raw["attributes"])
    return NewEventInput(**fields)


class SharedBuildingCasesTest(unittest.TestCase):
    def test_every_shared_case_gives_the_expected_result(self):
        paths = sorted(BUILDING_FOLDER.glob("*.json"))
        self.assertTrue(paths, f"no fixture files found in {BUILDING_FOLDER}")
        for path in paths:
            with self.subTest(case=path.name):
                case = json.loads(path.read_text(encoding="utf-8"))
                self.assertEqual(case["kind"], "build")
                setup = case["setup"]
                clock = FakeClock(wall_time=datetime.datetime.fromisoformat(setup["wall_time"]))
                known = setup.get("clock_info", {})
                clock.set_clock_info(ClockInfo(source=known.get("source"), synced=known.get("synced")))
                configuration = FakeConfiguration(Configuration(resource=ResourceIdentity(**setup["resource"])))
                id_generator = FakeIdGenerator([EventId(setup["event_id"])])
                service = EventBuilderService(clock, id_generator, configuration, validate_event)

                result = service.build(case_input(case["input"]))

                if "expected" in case:
                    self.assertIsNone(result.error)
                    self.assertIsNotNone(result.envelope)
                    self.assertEqual(canonical_json(result.envelope).decode("utf-8"), case["expected"])
                else:
                    self.assertIsNone(result.envelope)
                    self.assertIsInstance(result.error, ClassifiedError)
                    self.assertEqual(result.error.code, case["error_code"])
                    for piece in case.get("message_contains", []):
                        self.assertIn(piece, str(result.error))


class EventBuilderServiceTest(unittest.TestCase):
    def build_envelope(self, service: EventBuilderService, new_event: NewEventInput):
        result = service.build(new_event)
        self.assertIsNone(result.error)
        self.assertIsNotNone(result.envelope)
        return result.envelope

    def test_the_same_input_gives_the_same_bytes(self):
        first = canonical_json(self.build_envelope(make_service()[0], log_input()))
        second = canonical_json(self.build_envelope(make_service()[0], log_input()))
        self.assertEqual(first, second)

    def test_recorded_at_is_the_clock_even_when_occurred_at_is_given(self):
        given = instant_of("2026-10-28T09:00:00+00:00")
        envelope = self.build_envelope(make_service()[0], log_input(occurred_at=given))
        self.assertEqual(envelope.occurred_at, "2026-10-28T09:00:00.000000Z")
        self.assertEqual(envelope.recorded_at, "2026-10-28T10:00:00.123456Z")

    def test_without_occurred_at_both_times_are_the_clock(self):
        envelope = self.build_envelope(make_service()[0], log_input())
        self.assertEqual(envelope.occurred_at, envelope.recorded_at)

    def test_observed_at_sets_occurred_at_and_overrides_a_given_one(self):
        data = {"observed_at": "2026-10-28T08:30:00.000001Z"}
        given = instant_of("2026-10-28T09:00:00+00:00")
        for occurred_at in (None, given):
            with self.subTest(occurred_at=occurred_at):
                service, _, _, _ = make_service(permissive_validator)
                envelope = self.build_envelope(service, log_input(data=data, occurred_at=occurred_at))
                self.assertEqual(envelope.occurred_at, "2026-10-28T08:30:00.000001Z")
                self.assertEqual(envelope.recorded_at, "2026-10-28T10:00:00.123456Z")

    def test_an_invalid_observed_at_is_a_schema_error_that_does_not_echo_the_value(self):
        for value in (12345, None, "2026-10-28T10:00:00Z", "secret-not-a-time", "2026-02-30T10:00:00.000000Z"):
            with self.subTest(value=value):
                service = make_service(permissive_validator)[0]
                result = service.build(log_input(data={"observed_at": value}))
                self.assertIsNone(result.envelope)
                self.assertIsInstance(result.error, EventSchemaInvalidError)
                self.assertIn("data.observed_at", str(result.error))
                self.assertNotIn("secret-not-a-time", str(result.error))
                self.assertNotIn("12345", str(result.error))

    def test_data_that_is_not_an_object_is_a_schema_error(self):
        result = make_service(permissive_validator)[0].build(log_input(data="text"))
        self.assertIsInstance(result.error, EventSchemaInvalidError)
        self.assertIsNone(result.envelope)

    def test_a_failing_id_generator_gives_a_build_failure_and_nothing_is_raised(self):
        service, _, id_generator, _ = make_service()
        cause = RuntimeError("generator down")
        id_generator.fail_with(cause)
        result = service.build(log_input())
        self.assertIsInstance(result.error, EventBuildFailedError)
        self.assertIs(result.error.__cause__, cause)
        self.assertIsNone(result.envelope)

    def test_a_failing_configuration_gives_a_build_failure(self):
        service, _, _, configuration = make_service()
        configuration.fail_with(RuntimeError("no configuration"))
        result = service.build(log_input())
        self.assertIsInstance(result.error, EventBuildFailedError)

    def test_a_port_that_raises_something_unexpected_gives_a_build_failure(self):
        service, clock, _, _ = make_service()
        clock.clock_info = lambda: (_ for _ in ()).throw(ValueError("odd"))  # type: ignore[method-assign]
        result = service.build(log_input())
        self.assertIsInstance(result.error, EventBuildFailedError)
        self.assertIsInstance(result.error.__cause__, ValueError)

    def test_a_validator_that_raises_gives_a_build_failure(self):
        def broken_validator(event_type: str, data: Any) -> Optional[Exception]:
            raise RuntimeError("validator down")

        result = make_service(broken_validator)[0].build(log_input())
        self.assertIsInstance(result.error, EventBuildFailedError)

    def test_data_none_becomes_an_empty_object(self):
        service = make_service(permissive_validator)[0]
        envelope = self.build_envelope(service, log_input(data=None))
        self.assertEqual(envelope.data, {})
        self.assertIn('"data":{}', canonical_json(envelope).decode("utf-8"))

    def test_changing_the_callers_mapping_after_build_changes_nothing(self):
        data = dict(LOG_DATA)
        envelope = self.build_envelope(make_service()[0], log_input(data=data))
        data["message"] = "changed"
        data["extra"] = 1
        self.assertEqual(envelope.data, LOG_DATA)
        self.assertIsNot(envelope.data, data)

    def test_the_clock_is_read_once_per_build(self):
        service, clock, id_generator, configuration = make_service()
        self.build_envelope(service, log_input())
        self.assertEqual(clock.now_calls, 1)
        self.assertEqual(clock.clock_info_calls, 1)
        self.assertEqual(id_generator.event_id_calls, 1)
        self.assertEqual(configuration.calls, 1)

    def test_clock_attributes_are_added_only_when_known(self):
        cases = [
            (ClockInfo(), None),
            (ClockInfo(source="rtc"), {"clock_source": "rtc"}),
            (ClockInfo(synced=False), {"clock_synced": False}),
            (ClockInfo(source="ntp", synced=True), {"clock_source": "ntp", "clock_synced": True}),
        ]
        for clock_info, expected in cases:
            with self.subTest(clock_info=clock_info):
                service = make_service(clock_info=clock_info)[0]
                self.assertEqual(self.build_envelope(service, log_input()).attributes, expected)

    def test_clock_attributes_merge_with_the_callers_and_win_a_clash(self):
        service = make_service(clock_info=ClockInfo(source="ntp", synced=True))[0]
        attributes = EventAttributes({"order": 7, "clock_synced": False})
        envelope = self.build_envelope(service, log_input(attributes=attributes))
        self.assertEqual(envelope.attributes, {"order": 7, "clock_synced": True, "clock_source": "ntp"})
        self.assertEqual(attributes.as_dict(), {"order": 7, "clock_synced": False})

    def test_empty_attributes_become_none(self):
        envelope = self.build_envelope(make_service()[0], log_input(attributes=EventAttributes({})))
        self.assertIsNone(envelope.attributes)

    def test_an_empty_context_is_absent_and_a_given_one_is_kept(self):
        service = make_service()[0]
        self.assertIsNone(self.build_envelope(service, log_input(context=CorrelationContext())).context)
        service = make_service()[0]
        context = CorrelationContext(correlation_id="order-9")
        self.assertEqual(self.build_envelope(service, log_input(context=context)).context.correlation_id, "order-9")

    def test_the_actor_is_absent_when_not_given(self):
        self.assertIsNone(self.build_envelope(make_service()[0], log_input()).actor)

    def test_a_result_has_exactly_one_of_envelope_and_error(self):
        valid = make_service()[0].build(log_input())
        invalid = make_service()[0].build(log_input(data={}))
        unknown = make_service()[0].build(log_input(event_type="not.registered"))
        for result in (valid, invalid, unknown):
            self.assertIsInstance(result, BuildResult)
            self.assertNotEqual(result.envelope is None, result.error is None)
        self.assertIsNotNone(valid.envelope)
        self.assertEqual(invalid.error.code, "event_schema_invalid")
        self.assertEqual(unknown.error.code, "event_type_unknown")


if __name__ == "__main__":
    unittest.main()
