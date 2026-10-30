"""The defaults, the profiles, the validation, the errors and the provider of the configuration."""

import dataclasses
import unittest

from internal.tests.fakes.fake_id_generator import FakeIdGenerator
from internal.tests.infrastructure.config.helpers import PROFILES
from telemetry.internal.application.dtos.configuration import Configuration
from telemetry.internal.application.errors import ConfigurationInstanceIdFailedError
from telemetry.internal.application.ports.outbound.configuration_port import ConfigurationPort
from telemetry.internal.domain.errors import ClassifiedError, DeliveryModeUnknownError
from telemetry.internal.domain.operations.masking.sensitive_data_masker import MaskerOptions
from telemetry.internal.domain.value_objects.delivery_mode import parse_delivery_mode
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.config import profiles
from telemetry.internal.infrastructure.config.config import (
    Config,
    DeliveryConfig,
    LimitsConfig,
    MaskingConfig,
    ResourceConfig,
    SpoolConfig,
)
from telemetry.internal.infrastructure.config.defaults import defaults
from telemetry.internal.infrastructure.config.errors import ConfigProblem, ConfigurationError
from telemetry.internal.infrastructure.config.provider import ConfigurationProvider
from telemetry.internal.infrastructure.config.validate import validate_config

SECRET_LOOKING = "secret-looking-name"


def valid_config() -> Config:
    base = profiles.microservice()
    return dataclasses.replace(base, resource=ResourceConfig(service_name="sensor-hub"))


def with_section(config: Config, section_name: str, **changes) -> Config:
    section = getattr(config, section_name)
    return dataclasses.replace(config, **{section_name: dataclasses.replace(section, **changes)})


def problem_fields(config: Config) -> list[str]:
    return [problem.field for problem in validate_config(config)]


class DefaultsAndProfilesTest(unittest.TestCase):
    def test_defaults_are_the_documented_ones(self):
        config = defaults()
        self.assertEqual(config.resource, ResourceConfig("", "", "", "", "", ""))
        self.assertEqual(config.delivery, DeliveryConfig("best_effort", 10000, 100, 2000, 3000))
        self.assertEqual(config.limits, LimitsConfig(32, 64, 256, 65536, 1048576))
        self.assertEqual(config.spool, SpoolConfig("", 268435456))
        self.assertEqual(config.masking, MaskingConfig((), None, (), {}, {}))
        self.assertIsNone(config.masking.field_names)

    def test_each_profile_has_its_mode_and_everything_else_is_the_default(self):
        modes = {
            "microservice": "best_effort",
            "cronjob": "retry",
            "worker": "retry",
            "robotics": "durable",
            "offline_device": "durable",
        }
        self.assertEqual(set(modes), set(PROFILES))
        for name, build in PROFILES.items():
            with self.subTest(profile=name):
                config = build()
                self.assertEqual(config.delivery.mode, modes[name])
                self.assertEqual(config, with_section(defaults(), "delivery", mode=modes[name]))

    def test_precedence_default_then_profile_then_code(self):
        self.assertEqual(defaults().delivery.mode, "best_effort")
        self.assertEqual(profiles.cronjob().delivery.mode, "retry")
        changed = with_section(profiles.cronjob(), "delivery", mode="durable", queue_size=5)
        self.assertEqual((changed.delivery.mode, changed.delivery.queue_size), ("durable", 5))
        self.assertEqual(profiles.cronjob().delivery.queue_size, 10000)

    def test_parse_delivery_mode(self):
        for member in DeliveryMode:
            self.assertIs(parse_delivery_mode(member.value), member)
        for text in ("Retry", "", "lazy", None):
            with self.assertRaises(DeliveryModeUnknownError) as raised:
                parse_delivery_mode(text)
            self.assertEqual(raised.exception.code, "delivery_mode_unknown")


class ValidateConfigTest(unittest.TestCase):
    def test_a_valid_configuration_has_no_problems(self):
        self.assertEqual(validate_config(valid_config()), ())

    def test_the_all_empty_configuration_reports_several_problems_together(self):
        empty = Config(
            ResourceConfig(),
            DeliveryConfig("", 0, 0, 0, 0),
            LimitsConfig(0, 0, 0, 0, 0),
            SpoolConfig(directory="", quota_bytes=0),
            MaskingConfig(),
        )
        self.assertEqual(
            problem_fields(empty),
            [
                "resource.service_name",
                "delivery.mode",
                "delivery.queue_size",
                "delivery.batch_size",
                "delivery.flush_interval_ms",
                "delivery.timeout_ms",
                "limits.max_attribute_keys",
                "limits.max_attribute_key_chars",
                "limits.max_attribute_value_chars",
                "limits.max_event_bytes",
                "limits.max_batch_bytes",
                "spool.quota_bytes",
            ],
        )

    def test_the_optional_resource_fields_and_the_time_zone(self):
        for field_name in ("instance_id", "environment", "service_version", "host"):
            for bad in ("   ", "a\tb", 5):
                with self.subTest(field=field_name, bad=bad):
                    config = with_section(valid_config(), "resource", **{field_name: bad})
                    self.assertEqual(problem_fields(config), [f"resource.{field_name}"])
        for bad in ("not a zone", 7):
            config = with_section(valid_config(), "resource", time_zone=bad)
            self.assertEqual(problem_fields(config), ["resource.time_zone"])
        config = with_section(valid_config(), "resource", time_zone="Europe/Madrid", host="h")
        self.assertEqual(validate_config(config), ())

    def test_the_delivery_mode_code_and_a_member_of_the_enum(self):
        config = with_section(valid_config(), "delivery", mode="lazy")
        self.assertEqual(
            [(p.field, p.code) for p in validate_config(config)], [("delivery.mode", "delivery_mode_unknown")]
        )
        self.assertEqual(validate_config(with_section(valid_config(), "delivery", mode=DeliveryMode.RETRY)), ())

    def test_the_batch_size_against_the_queue_size(self):
        config = with_section(valid_config(), "delivery", queue_size=10, batch_size=11)
        self.assertEqual(problem_fields(config), ["delivery.batch_size"])
        config = with_section(valid_config(), "delivery", queue_size=0, batch_size=11)
        self.assertEqual(problem_fields(config), ["delivery.queue_size"])
        config = with_section(valid_config(), "delivery", queue_size=10, batch_size=10)
        self.assertEqual(validate_config(config), ())

    def test_the_numbers_must_be_integers_above_zero(self):
        for bad in (0, -1, True, 1.5, "3", None):
            with self.subTest(bad=bad):
                config = with_section(valid_config(), "delivery", flush_interval_ms=bad, timeout_ms=bad)
                self.assertEqual(problem_fields(config), ["delivery.flush_interval_ms", "delivery.timeout_ms"])

    def test_the_batch_limit_against_the_event_limit(self):
        config = with_section(valid_config(), "limits", max_event_bytes=100, max_batch_bytes=99)
        self.assertEqual(problem_fields(config), ["limits.max_batch_bytes"])
        config = with_section(valid_config(), "limits", max_event_bytes=0, max_batch_bytes=99)
        self.assertEqual(problem_fields(config), ["limits.max_event_bytes"])

    def test_the_spool_directory_is_required_only_for_durable(self):
        durable = with_section(valid_config(), "delivery", mode="durable")
        self.assertEqual(problem_fields(durable), ["spool.directory"])
        self.assertEqual(problem_fields(with_section(durable, "spool", directory=5)), ["spool.directory"])
        self.assertEqual(validate_config(with_section(durable, "spool", directory="spool-dir")), ())
        unknown = with_section(valid_config(), "delivery", mode="lazy")
        self.assertEqual(problem_fields(unknown), ["delivery.mode"])

    def test_the_masking_options(self):
        for bad in (("(",), ["a", 3], "abc", None):
            with self.subTest(extra_patterns=bad):
                config = with_section(valid_config(), "masking", extra_patterns=bad)
                self.assertEqual(problem_fields(config), ["masking.extra_patterns"])
        for bad in ({"pii": "keep"}, {"a": "drop", "b": 3}, None, ["drop"]):
            with self.subTest(class_rules=bad):
                config = with_section(valid_config(), "masking", class_rules=bad)
                self.assertEqual(problem_fields(config), ["masking.class_rules"])
        good = with_section(valid_config(), "masking", extra_patterns=["ID-[0-9]+"], class_rules={"c": "allow"})
        self.assertEqual(validate_config(good), ())

    def test_a_reason_never_holds_a_configured_value(self):
        config = with_section(valid_config(), "resource", service_name="  ", host=SECRET_LOOKING + "\n")
        config = with_section(config, "delivery", mode=SECRET_LOOKING)
        config = with_section(
            config, "masking", extra_patterns=(SECRET_LOOKING + "(",), class_rules={"x": SECRET_LOOKING}
        )
        problems = validate_config(config)
        self.assertEqual(len(problems), 5)
        for problem in problems:
            self.assertNotIn(SECRET_LOOKING, problem.reason)
        with self.assertRaises(ConfigurationError) as raised:
            ConfigurationProvider(config, FakeIdGenerator())
        self.assertNotIn(SECRET_LOOKING, str(raised.exception))


class ConfigurationErrorTest(unittest.TestCase):
    def test_the_problems_the_code_and_the_message(self):
        problems = (
            ConfigProblem("delivery.mode", "delivery_mode_unknown", "bad mode"),
            ConfigProblem("spool.quota_bytes", "config_field_invalid", "bad quota"),
        )
        error = ConfigurationError(problems)
        self.assertIsInstance(error, ClassifiedError)
        self.assertEqual(error.problems, problems)
        self.assertEqual(error.code, "delivery_mode_unknown")
        self.assertEqual(
            str(error),
            "delivery.mode: bad mode (delivery_mode_unknown)\nspool.quota_bytes: bad quota (config_field_invalid)",
        )

    def test_without_problems_the_code_is_config_field_invalid(self):
        error = ConfigurationError(())
        self.assertEqual((error.problems, error.code, str(error)), ((), "config_field_invalid", ""))


class ConfigurationProviderTest(unittest.TestCase):
    def test_it_satisfies_the_configuration_port(self):
        self.assertIsInstance(ConfigurationProvider(valid_config(), FakeIdGenerator()), ConfigurationPort)

    def test_an_empty_instance_id_is_generated(self):
        generator = FakeIdGenerator(instance_id="generated-1")
        configuration = ConfigurationProvider(valid_config(), generator).configuration()
        self.assertEqual(configuration.resource.instance_id, "generated-1")
        self.assertEqual(generator.instance_id_calls, 1)

    def test_a_given_instance_id_is_kept(self):
        generator = FakeIdGenerator()
        config = with_section(valid_config(), "resource", instance_id="hub-01")
        configuration = ConfigurationProvider(config, generator).configuration()
        self.assertEqual(configuration.resource.instance_id, "hub-01")
        self.assertEqual(generator.instance_id_calls, 0)

    def test_a_failure_of_the_generator_is_wrapped_with_the_cause(self):
        class Broken:
            def new_instance_id(self):
                raise RuntimeError("no entropy")

        with self.assertRaises(ConfigurationInstanceIdFailedError) as raised:
            ConfigurationProvider(valid_config(), Broken())
        self.assertIsInstance(raised.exception.__cause__, RuntimeError)

    def test_an_invalid_configuration_is_refused_before_the_generator_is_used(self):
        generator = FakeIdGenerator()
        with self.assertRaises(ConfigurationError):
            ConfigurationProvider(defaults(), generator)
        self.assertEqual(generator.instance_id_calls, 0)

    def test_the_configuration_has_every_part(self):
        config = Config(
            ResourceConfig("svc", "i-1", "staging", "1.2.3", "host-a", "Europe/Madrid"),
            DeliveryConfig("durable", 500, 50, 1000, 1500),
            LimitsConfig(10, 20, 30, 4000, 9000),
            SpoolConfig("spool-dir", 12345),
            MaskingConfig(("a+",), ("pwd",), ("ssn",), {"f": "c"}, {"c": "drop"}),
        )
        configuration = ConfigurationProvider(config, FakeIdGenerator()).configuration()
        self.assertIsInstance(configuration, Configuration)
        resource = configuration.resource
        self.assertEqual(
            (resource.service_name, resource.instance_id, resource.environment),
            ("svc", "i-1", "staging"),
        )
        self.assertEqual(
            (resource.service_version, resource.host, resource.time_zone), ("1.2.3", "host-a", "Europe/Madrid")
        )
        delivery = configuration.delivery
        self.assertIs(delivery.mode, DeliveryMode.DURABLE)
        self.assertEqual(
            (
                delivery.queue_size,
                delivery.batch_size,
                delivery.flush_interval_milliseconds,
                delivery.timeout_milliseconds,
            ),
            (500, 50, 1000, 1500),
        )
        self.assertEqual(configuration.limits, Limits(10, 20, 30, 4000, 9000))
        self.assertEqual((configuration.spool.directory, configuration.spool.quota_bytes), ("spool-dir", 12345))
        self.assertEqual(
            configuration.masking,
            MaskerOptions(("a+",), ("pwd",), ("ssn",), {"f": "c"}, {"c": "drop"}),
        )

    def test_empty_optional_fields_become_none_and_the_field_names_keep_their_meaning(self):
        configuration = ConfigurationProvider(valid_config(), FakeIdGenerator()).configuration()
        resource = configuration.resource
        self.assertEqual(
            (resource.environment, resource.service_version, resource.host, resource.time_zone), (None,) * 4
        )
        self.assertIsNone(configuration.masking.field_names)
        replaced = with_section(valid_config(), "masking", field_names=())
        self.assertEqual(ConfigurationProvider(replaced, FakeIdGenerator()).configuration().masking.field_names, ())

    def test_the_configuration_does_not_change_with_the_config(self):
        field_classes = {"f": "c"}
        config = with_section(valid_config(), "masking", field_classes=field_classes, class_rules={"c": "drop"})
        configuration = ConfigurationProvider(config, FakeIdGenerator()).configuration()
        field_classes["g"] = "d"
        self.assertEqual(dict(configuration.masking.field_classes), {"f": "c"})
        self.assertIsNot(configuration.masking.field_classes, field_classes)
