"""The error model: the same code in both languages, the hierarchy and error_code_of (REQ-CNF-004, T-03.7).

The shared table integration-tests/fixtures/errors/codes.json lists the failures of the core. Each one has a
producer here that makes the REAL failure through the real code, and the code of the error it gives must be the one
of the table. Go has a producer of the same name for each case.
"""

import dataclasses
import datetime
import importlib
import inspect
import json
import pathlib
import pkgutil
import re
import unittest
from typing import Any, Callable

from internal.tests.domain.operations.limits.helpers import make_envelope
from internal.tests.fakes import FakeClock, FakeConfiguration, FakeIdGenerator
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.errors import (
    ConfigurationInstanceIdFailedError,
    EnvelopeLimitsFailedError,
    EnvelopeMaskingFailedError,
    EventBuildFailedError,
)
from telemetry.internal.application.services.event_builder_service import EventBuilderService
from telemetry.internal.application.services.limits_service import LimitsService
from telemetry.internal.application.services.masking_service import MaskingService
from telemetry.internal.domain.base_value_objects.errors import TelemetryError, ValueObjectError
from telemetry.internal.domain.base_value_objects.hex_digits import HexDigits
from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.errors import (
    ClassifiedError,
    EventSchemaInvalidError,
    error_code_of,
)
from telemetry.internal.domain.errors import TelemetryError as ReexportedTelemetryError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.event_validation.event_validation import validate_event
from telemetry.internal.domain.operations.limits.apply_limits import apply_limits
from telemetry.internal.domain.operations.masking.sensitive_data_masker import MaskerOptions, SensitiveDataMasker
from telemetry.internal.domain.value_objects.actor_identity import ActorIdentity
from telemetry.internal.domain.value_objects.correlation_context import CorrelationContext
from telemetry.internal.domain.value_objects.delivery_mode import parse_delivery_mode
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.error_codes_generated import (
    ERROR_CODE_EVENT_SCHEMA_INVALID,
    ERROR_CODE_LIMIT_EXCEEDED,
    ERROR_CODES,
)
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity, validate_time_zone
from telemetry.internal.domain.value_objects.severity import parse_severity
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant
from telemetry.internal.infrastructure.config import profiles
from telemetry.internal.infrastructure.config.config import ResourceConfig
from telemetry.internal.infrastructure.config.provider import ConfigurationProvider

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[4]
CODES_FILE = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "errors" / "codes.json"
CATALOG_FILE = REPOSITORY_ROOT / "contracts" / "errors" / "catalog.yaml"
WALL_TIME = datetime.datetime(2026, 10, 28, 10, 0, 0, tzinfo=datetime.timezone.utc)
LOG_EVENT = "log.entry.recorded"


def load_table() -> dict[str, Any]:
    """Read the shared table; a missing or empty file fails the test."""
    table = json.loads(CODES_FILE.read_text(encoding="utf-8"))
    if not table.get("cases") or not table.get("secret"):
        raise AssertionError("the table of error codes has no cases or no secret")
    return table


def catalog_codes() -> set[str]:
    """Read the codes of contracts/errors/catalog.yaml with a line reader (no yaml library)."""
    pattern = re.compile(r"^\s*-\s*code:\s*([a-z_]+)\s*$")
    codes = set()
    for line in CATALOG_FILE.read_text(encoding="utf-8").splitlines():
        match = pattern.match(line)
        if match:
            codes.add(match.group(1))
    return codes


def caught(action: Callable[[], Any]) -> BaseException:
    """Run an action that must raise and return what it raised."""
    try:
        action()
    except BaseException as error:  # the producer wants any error
        return error
    raise AssertionError("the action did not raise")


class _MaskerThatRaises:
    def mask_map(self, mapping: Any) -> Any:
        raise RuntimeError("the masker failed")

    def mask_value(self, field_name: str, value: Any) -> Any:
        raise RuntimeError("the masker failed")


class _InstanceIdFails:
    def new_event_id(self) -> EventId:
        return EventId("01900000-0000-7000-8000-000000000001")

    def new_instance_id(self) -> Identifier:
        raise RuntimeError("the id source is not available")


def _valid_config_with(**resource_changes: str):
    base = profiles.microservice()
    resource = ResourceConfig(**{"service_name": "sensor-hub", **resource_changes})
    return dataclasses.replace(base, resource=resource)


def _produce_event_id_invalid(secret: str) -> BaseException:
    return caught(lambda: EventId(secret))


def _produce_severity_invalid(secret: str) -> BaseException:
    return caught(lambda: parse_severity(secret))


def _produce_actor_kind_invalid(secret: str) -> BaseException:
    return caught(lambda: ActorIdentity(secret, "actor-1"))  # type: ignore[arg-type]


def _produce_identifier_blank(secret: str) -> BaseException:
    return caught(lambda: Identifier("   "))


def _produce_identifier_control_characters(secret: str) -> BaseException:
    return caught(lambda: Identifier(secret + "\n"))


def _produce_hex_digits_invalid(secret: str) -> BaseException:
    return caught(lambda: HexDigits(secret, 8))


def _produce_time_zone_invalid(secret: str) -> BaseException:
    return caught(lambda: validate_time_zone(secret + " x"))


def _produce_utc_instant_not_utc(secret: str) -> BaseException:
    local = datetime.datetime(2026, 10, 28, 10, 0, 0, tzinfo=datetime.timezone(datetime.timedelta(hours=2)))
    return caught(lambda: UtcInstant(local))


def _produce_utc_instant_text_invalid(secret: str) -> BaseException:
    return caught(lambda: UtcInstant.parse(secret))


def _produce_attribute_key_invalid(secret: str) -> BaseException:
    return caught(lambda: EventAttributes({"": "v"}))


def _produce_attribute_value_invalid(secret: str) -> BaseException:
    return caught(lambda: EventAttributes({"key": {"nested": secret}}))


def _produce_resource_identity_invalid(secret: str) -> BaseException:
    return caught(lambda: ResourceIdentity("s", "i", environment=secret + "\n"))


def _produce_actor_identity_invalid(secret: str) -> BaseException:
    return caught(lambda: ActorIdentity("device", secret + "\n"))


def _produce_correlation_context_invalid(secret: str) -> BaseException:
    return caught(lambda: CorrelationContext(trace_id=secret))


def _produce_number_out_of_range(secret: str) -> BaseException:
    return caught(lambda: canonical_json(float("nan")))


def _produce_unsupported_value(secret: str) -> BaseException:
    return caught(lambda: canonical_json(secret + "\ud800"))


def _produce_event_schema_invalid(secret: str) -> BaseException:
    error = validate_event(LOG_EVENT, {"category": "http", "extra_field": secret})
    assert error is not None
    return error


def _produce_event_type_unknown(secret: str) -> BaseException:
    error = validate_event("nothing.here.recorded", {})
    assert error is not None
    return error


def _produce_limit_exceeded(secret: str) -> BaseException:
    outcome = apply_limits(make_envelope(message=secret), Limits(max_event_bytes=10, max_batch_bytes=10))
    assert outcome.error is not None
    return outcome.error


def _produce_limit_invalid(secret: str) -> BaseException:
    return caught(lambda: Limits(max_attribute_keys=0))


def _produce_batch_limit_below_event_limit(secret: str) -> BaseException:
    return caught(lambda: Limits(max_event_bytes=100, max_batch_bytes=50))


def _produce_mask_pattern_invalid(secret: str) -> BaseException:
    return caught(lambda: SensitiveDataMasker(MaskerOptions(extra_patterns=(secret + "(",))))


def _produce_mask_rule_invalid(secret: str) -> BaseException:
    return caught(lambda: SensitiveDataMasker(MaskerOptions(class_rules={"c": secret})))


def _produce_delivery_mode_unknown(secret: str) -> BaseException:
    return caught(lambda: parse_delivery_mode(secret))


def _produce_config_field_invalid(secret: str) -> BaseException:
    config = _valid_config_with(service_name=secret + "\n", time_zone="not a zone")
    return caught(lambda: ConfigurationProvider(config, FakeIdGenerator()))


def _produce_instance_id_unavailable(secret: str) -> BaseException:
    return caught(lambda: ConfigurationProvider(_valid_config_with(), _InstanceIdFails()))  # type: ignore[arg-type]


def _produce_event_build_failed(secret: str) -> BaseException:
    id_generator = FakeIdGenerator()
    id_generator.fail_with(RuntimeError("the id source is not available"))
    service = EventBuilderService(FakeClock(wall_time=WALL_TIME), id_generator, FakeConfiguration(), validate_event)
    result = service.build(NewEventInput(event_type=LOG_EVENT, severity=Severity.INFO, data={"message": secret}))
    assert result.error is not None
    return result.error


def _produce_masking_failed(secret: str) -> BaseException:
    result = MaskingService(_MaskerThatRaises()).mask(make_envelope(message=secret))  # type: ignore[arg-type]
    assert result.error is not None
    return result.error


def _produce_limits_failed(secret: str) -> BaseException:
    result = LimitsService(Limits()).apply(make_envelope({"a\ud800": secret}))
    assert result.error is not None
    return result.error


PRODUCERS: dict[str, Callable[[str], BaseException]] = {
    "event_id_invalid": _produce_event_id_invalid,
    "severity_invalid": _produce_severity_invalid,
    "actor_kind_invalid": _produce_actor_kind_invalid,
    "identifier_blank": _produce_identifier_blank,
    "identifier_control_characters": _produce_identifier_control_characters,
    "hex_digits_invalid": _produce_hex_digits_invalid,
    "time_zone_invalid": _produce_time_zone_invalid,
    "utc_instant_not_utc": _produce_utc_instant_not_utc,
    "utc_instant_text_invalid": _produce_utc_instant_text_invalid,
    "attribute_key_invalid": _produce_attribute_key_invalid,
    "attribute_value_invalid": _produce_attribute_value_invalid,
    "resource_identity_invalid": _produce_resource_identity_invalid,
    "actor_identity_invalid": _produce_actor_identity_invalid,
    "correlation_context_invalid": _produce_correlation_context_invalid,
    "number_out_of_range": _produce_number_out_of_range,
    "unsupported_value": _produce_unsupported_value,
    "event_schema_invalid": _produce_event_schema_invalid,
    "event_type_unknown": _produce_event_type_unknown,
    "limit_exceeded": _produce_limit_exceeded,
    "limit_invalid": _produce_limit_invalid,
    "batch_limit_below_event_limit": _produce_batch_limit_below_event_limit,
    "mask_pattern_invalid": _produce_mask_pattern_invalid,
    "mask_rule_invalid": _produce_mask_rule_invalid,
    "delivery_mode_unknown": _produce_delivery_mode_unknown,
    "config_field_invalid": _produce_config_field_invalid,
    "instance_id_unavailable": _produce_instance_id_unavailable,
    "event_build_failed": _produce_event_build_failed,
    "masking_failed": _produce_masking_failed,
    "limits_failed": _produce_limits_failed,
}


def chain_of(error: BaseException) -> list[BaseException]:
    """The error and every error of its chain of causes (``__cause__``, else ``__context__``), without cycles."""
    chain: list[BaseException] = []
    current: BaseException | None = error
    while current is not None and all(current is not seen for seen in chain):
        chain.append(current)
        current = current.__cause__ if current.__cause__ is not None else current.__context__
    return chain


class ErrorCodesTableTest(unittest.TestCase):
    def test_the_producers_are_the_cases_of_the_table(self):
        names = {case["name"] for case in load_table()["cases"]}
        producers = set(PRODUCERS)
        self.assertEqual(names - producers, set(), "a case of the table has no producer")
        self.assertEqual(producers - names, set(), "a producer is not a case of the table")

    def test_every_case_gives_its_code(self):
        table = load_table()
        for case in table["cases"]:
            with self.subTest(case=case["name"]):
                error = PRODUCERS[case["name"]](table["secret"])
                self.assertIsNotNone(error)
                self.assertEqual(error_code_of(error), case["expected_code"])

    def test_no_message_holds_the_value_of_the_caller(self):
        table = load_table()
        for case in table["cases"]:
            if not case["value_in_input"]:
                continue
            with self.subTest(case=case["name"]):
                error = PRODUCERS[case["name"]](table["secret"])
                for link in chain_of(error):
                    if isinstance(link, TelemetryError):
                        self.assertNotIn(table["secret"], str(link))

    def test_every_code_of_the_catalog_is_expected_and_every_expected_code_is_in_it(self):
        expected = {case["expected_code"] for case in load_table()["cases"]}
        self.assertEqual(set(ERROR_CODES) - expected, set())
        self.assertEqual(expected - set(ERROR_CODES), set())

    def test_the_generated_codes_are_the_codes_of_the_catalog_file(self):
        self.assertEqual(set(ERROR_CODES), catalog_codes())
        self.assertTrue(catalog_codes())


def _library_modules():
    modules = []
    for package_name in ("domain", "application", "infrastructure"):
        package = importlib.import_module(f"telemetry.internal.{package_name}")
        modules.append(package)
        for info in pkgutil.walk_packages(package.__path__, package.__name__ + "."):
            modules.append(importlib.import_module(info.name))
    return modules


class HierarchyTest(unittest.TestCase):
    def test_every_exception_of_the_library_derives_from_telemetry_error(self):
        checked = 0
        for module in _library_modules():
            for name, candidate in inspect.getmembers(module, inspect.isclass):
                if candidate.__module__ != module.__name__ or not issubclass(candidate, Exception):
                    continue
                checked += 1
                with self.subTest(exception=f"{module.__name__}.{name}"):
                    self.assertTrue(issubclass(candidate, TelemetryError))
        self.assertGreater(checked, 20)

    def test_the_root_is_one_class_and_the_classes_with_a_code_are_classified(self):
        self.assertIs(ReexportedTelemetryError, TelemetryError)
        self.assertTrue(issubclass(ValueObjectError, TelemetryError))
        self.assertTrue(issubclass(ClassifiedError, TelemetryError))
        for application_error in (
            EventBuildFailedError,
            EnvelopeMaskingFailedError,
            EnvelopeLimitsFailedError,
            ConfigurationInstanceIdFailedError,
        ):
            self.assertTrue(issubclass(application_error, ClassifiedError))
            self.assertIn(application_error.code, ERROR_CODES)


class ErrorCodeOfTest(unittest.TestCase):
    def test_a_classified_error_gives_its_code(self):
        self.assertEqual(error_code_of(EventSchemaInvalidError("x")), ERROR_CODE_EVENT_SCHEMA_INVALID)
        self.assertEqual(error_code_of(ClassifiedError("x", ERROR_CODE_LIMIT_EXCEEDED)), ERROR_CODE_LIMIT_EXCEEDED)

    def test_a_plain_exception_gives_the_code_of_its_cause(self):
        try:
            try:
                raise ClassifiedError("inner", ERROR_CODE_LIMIT_EXCEEDED)
            except ClassifiedError as cause:
                raise RuntimeError("outer") from cause
        except RuntimeError as error:
            self.assertEqual(error_code_of(error), ERROR_CODE_LIMIT_EXCEEDED)

    def test_a_plain_exception_gives_the_code_of_its_context(self):
        try:
            try:
                raise ClassifiedError("inner", ERROR_CODE_LIMIT_EXCEEDED)
            except ClassifiedError:
                raise RuntimeError("outer")  # the implicit context is what is tested
        except RuntimeError as error:
            self.assertEqual(error_code_of(error), ERROR_CODE_LIMIT_EXCEEDED)

    def test_a_value_object_error_is_a_validation_error(self):
        self.assertEqual(error_code_of(ValueObjectError("x")), ERROR_CODE_EVENT_SCHEMA_INVALID)

    def test_an_unknown_exception_gives_no_code(self):
        self.assertEqual(error_code_of(RuntimeError("x")), "")
        self.assertEqual(error_code_of(TelemetryError("x")), "")

    def test_a_cycle_in_the_chain_does_not_loop(self):
        first = RuntimeError("first")
        second = RuntimeError("second")
        first.__cause__ = second
        second.__cause__ = first
        self.assertEqual(error_code_of(first), "")


if __name__ == "__main__":
    unittest.main()
