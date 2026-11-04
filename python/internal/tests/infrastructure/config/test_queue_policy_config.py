"""Tests of the configuration of the full-queue rules: queue_policy, block_timeout_ms and event_classes."""

import dataclasses
import unittest
from typing import Any

from internal.tests.fakes.fake_id_generator import FakeIdGenerator
from internal.tests.infrastructure.config.helpers import PROFILES
from telemetry.internal.domain.value_objects.enums_generated import EventClass, QueuePolicy
from telemetry.internal.infrastructure.config import profiles
from telemetry.internal.infrastructure.config.config import Config
from telemetry.internal.infrastructure.config.defaults import defaults
from telemetry.internal.infrastructure.config.env_names import ENVIRONMENT_VARIABLES
from telemetry.internal.infrastructure.config.provider import ConfigurationProvider
from telemetry.internal.infrastructure.config.validate import validate_config

SECRET = "hunter2secret"


def valid_config(**delivery_changes: Any) -> Config:
    base = profiles.microservice()
    resource = dataclasses.replace(base.resource, service_name="sensor-hub")
    delivery = dataclasses.replace(base.delivery, **delivery_changes)
    return dataclasses.replace(base, resource=resource, delivery=delivery)


def problem_fields(config: Config) -> list[str]:
    return [problem.field for problem in validate_config(config)]


class DefaultsTest(unittest.TestCase):
    def test_the_defaults_are_drop_newest_no_timeout_and_no_assignment(self):
        delivery = defaults().delivery
        self.assertEqual(
            (delivery.queue_policy, delivery.block_timeout_ms, delivery.event_classes), ("drop_newest", 0, {})
        )

    def test_the_fields_come_in_the_order_of_the_schema(self):
        names = [field.name for field in dataclasses.fields(defaults().delivery)]
        self.assertEqual(
            names,
            [
                "mode",
                "queue_size",
                "batch_size",
                "flush_interval_ms",
                "timeout_ms",
                "queue_policy",
                "block_timeout_ms",
                "event_classes",
                "retry_max_attempts",
                "retry_base_delay_ms",
                "retry_max_delay_ms",
            ],
        )

    def test_every_profile_keeps_the_default_policy(self):
        for name, profile in PROFILES.items():
            with self.subTest(profile=name):
                delivery = profile().delivery
                self.assertEqual((delivery.queue_policy, delivery.block_timeout_ms), ("drop_newest", 0))
                self.assertEqual(delivery.event_classes, {})

    def test_the_default_map_is_not_shared_between_configurations(self):
        first, second = defaults(), defaults()
        self.assertIsNot(first.delivery.event_classes, second.delivery.event_classes)

    def test_the_new_settings_are_loadable_by_these_names(self):
        self.assertEqual(ENVIRONMENT_VARIABLES["delivery.queue_policy"], "TELEMETRY_QUEUE_POLICY")
        self.assertEqual(ENVIRONMENT_VARIABLES["delivery.block_timeout_ms"], "TELEMETRY_BLOCK_TIMEOUT_MS")
        self.assertEqual(ENVIRONMENT_VARIABLES["delivery.event_classes"], "TELEMETRY_EVENT_CLASSES")


class ValidationTest(unittest.TestCase):
    def test_the_defaults_are_valid(self):
        self.assertEqual(validate_config(valid_config()), ())

    def test_every_policy_is_valid_and_block_with_timeout_needs_a_timeout_above_zero(self):
        for policy in ("drop_newest", "drop_oldest"):
            with self.subTest(policy=policy):
                self.assertEqual(problem_fields(valid_config(queue_policy=policy)), [])
                self.assertEqual(problem_fields(valid_config(queue_policy=policy, block_timeout_ms=500)), [])
        self.assertEqual(problem_fields(valid_config(queue_policy="block_with_timeout", block_timeout_ms=1)), [])
        self.assertEqual(
            problem_fields(valid_config(queue_policy="block_with_timeout", block_timeout_ms=0)),
            ["delivery.block_timeout_ms"],
        )
        self.assertEqual(problem_fields(valid_config(queue_policy="block_with_timeout")), ["delivery.block_timeout_ms"])

    def test_an_unknown_or_badly_typed_policy_is_invalid(self):
        for bad in ("lazy", "", "DROP_NEWEST", None, 3, QueuePolicy):
            with self.subTest(bad=bad):
                self.assertEqual(problem_fields(valid_config(queue_policy=bad)), ["delivery.queue_policy"])

    def test_the_policy_enum_member_is_accepted_as_its_text(self):
        self.assertEqual(problem_fields(valid_config(queue_policy=QueuePolicy.DROP_OLDEST)), [])

    def test_a_negative_or_badly_typed_block_timeout_is_invalid(self):
        for bad in (-1, 1.5, "10", None, True):
            with self.subTest(bad=bad):
                self.assertEqual(problem_fields(valid_config(block_timeout_ms=bad)), ["delivery.block_timeout_ms"])

    def test_a_block_timeout_that_is_negative_is_one_problem_even_with_the_blocking_policy(self):
        config = valid_config(queue_policy="block_with_timeout", block_timeout_ms=-5)
        self.assertEqual(problem_fields(config), ["delivery.block_timeout_ms"])

    def test_the_problem_of_an_unknown_policy_does_not_hide_the_timeout_one(self):
        self.assertEqual(
            problem_fields(valid_config(queue_policy="lazy", block_timeout_ms=-1)),
            ["delivery.queue_policy", "delivery.block_timeout_ms"],
        )

    def test_event_classes_with_the_three_classes_are_valid(self):
        classes = {"a.b": "critical", "c.d": "normal", "e.f": "bulk"}
        self.assertEqual(problem_fields(valid_config(event_classes=classes)), [])
        self.assertEqual(problem_fields(valid_config(event_classes={"a.b": EventClass.CRITICAL})), [])

    def test_a_bad_event_class_map_is_one_problem_for_the_field(self):
        bad_maps: list[Any] = [
            {"a.b": "urgent"},
            {"": "normal"},
            {"a.b": "urgent", "": "normal", "c.d": "x"},
            {"a.b": ""},
            {"a.b": None},
            {"a.b": 1},
            {1: "normal"},
            {None: "normal"},
            ["a.b"],
            "critical",
            None,
        ]
        for bad in bad_maps:
            with self.subTest(bad=bad):
                self.assertEqual(problem_fields(valid_config(event_classes=bad)), ["delivery.event_classes"])

    def test_the_problems_follow_the_order_of_the_schema_and_use_the_code_of_the_catalog(self):
        config = valid_config(
            queue_size=0, queue_policy="lazy", block_timeout_ms=-1, event_classes={"a": "x"}, timeout_ms=0
        )
        found = [(problem.field, problem.code) for problem in validate_config(config)]
        self.assertEqual(
            found,
            [
                ("delivery.queue_size", "config_field_invalid"),
                ("delivery.timeout_ms", "config_field_invalid"),
                ("delivery.queue_policy", "config_field_invalid"),
                ("delivery.block_timeout_ms", "config_field_invalid"),
                ("delivery.event_classes", "config_field_invalid"),
            ],
        )

    def test_a_reason_never_holds_the_configured_value(self):
        config = valid_config(queue_policy=SECRET, block_timeout_ms=-123456, event_classes={SECRET: SECRET})
        problems = validate_config(config)
        self.assertEqual(len(problems), 3)
        for problem in problems:
            self.assertNotIn(SECRET, problem.reason)
            self.assertNotIn("123456", problem.reason)


class ProviderTest(unittest.TestCase):
    def test_the_core_receives_the_policy_the_timeout_and_the_classes_as_values_of_the_domain(self):
        config = valid_config(
            queue_policy="block_with_timeout",
            block_timeout_ms=250,
            event_classes={"log.entry.recorded": "critical", "x.y": "bulk"},
        )
        delivery = ConfigurationProvider(config, FakeIdGenerator()).configuration().delivery
        self.assertIs(delivery.queue_policy, QueuePolicy.BLOCK_WITH_TIMEOUT)
        self.assertEqual(delivery.block_timeout_milliseconds, 250)
        self.assertEqual(delivery.event_classes, {"log.entry.recorded": EventClass.CRITICAL, "x.y": EventClass.BULK})
        self.assertIs(delivery.event_classes["x.y"], EventClass.BULK)

    def test_the_defaults_reach_the_core(self):
        delivery = ConfigurationProvider(valid_config(), FakeIdGenerator()).configuration().delivery
        self.assertEqual((delivery.queue_policy, delivery.block_timeout_milliseconds), (QueuePolicy.DROP_NEWEST, 0))
        self.assertEqual(delivery.event_classes, {})

    def test_what_the_core_holds_is_a_copy_of_the_map(self):
        classes = {"a.b": "bulk"}
        provider = ConfigurationProvider(valid_config(event_classes=classes), FakeIdGenerator())
        classes["a.b"] = "critical"
        classes["c.d"] = "critical"
        self.assertEqual(provider.configuration().delivery.event_classes, {"a.b": EventClass.BULK})


if __name__ == "__main__":
    unittest.main()
