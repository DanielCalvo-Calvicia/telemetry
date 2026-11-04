"""Tests of the retry settings of the configuration, and of the delivery mode that each profile reaches."""

import dataclasses
import unittest
from typing import Any

import telemetry
from internal.tests.eventually import eventually, stop_workers_on_cleanup
from internal.tests.fakes import FakeLogSink
from internal.tests.fakes.fake_id_generator import FakeIdGenerator
from internal.tests.infrastructure.config.helpers import PROFILES
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode
from telemetry.internal.infrastructure.config import profiles
from telemetry.internal.infrastructure.config.config import Config
from telemetry.internal.infrastructure.config.defaults import defaults
from telemetry.internal.infrastructure.config.env_names import ENVIRONMENT_VARIABLES
from telemetry.internal.infrastructure.config.provider import ConfigurationProvider
from telemetry.internal.infrastructure.config.validate import validate_config

RETRY_FIELDS = ("delivery.retry_max_attempts", "delivery.retry_base_delay_ms", "delivery.retry_max_delay_ms")


def valid_config(profile: Any = profiles.microservice, **delivery_changes: Any) -> Config:
    base = profile()
    resource = dataclasses.replace(base.resource, service_name="sensor-hub")
    delivery = dataclasses.replace(base.delivery, **delivery_changes)
    return dataclasses.replace(base, resource=resource, delivery=delivery)


def problems_of(config: Config) -> list[tuple[str, str]]:
    return [(problem.field, problem.code) for problem in validate_config(config)]


class RetrySettingsTest(unittest.TestCase):
    def test_the_defaults_are_five_attempts_200_ms_and_30_seconds(self):
        delivery = defaults().delivery
        self.assertEqual(
            (delivery.retry_max_attempts, delivery.retry_base_delay_ms, delivery.retry_max_delay_ms), (5, 200, 30000)
        )
        for name, build in PROFILES.items():
            with self.subTest(profile=name):
                self.assertEqual(build().delivery.retry_max_attempts, 5)

    def test_the_names_to_load_them_are_the_ones_of_the_schema(self):
        self.assertEqual(ENVIRONMENT_VARIABLES["delivery.retry_max_attempts"], "TELEMETRY_RETRY_MAX_ATTEMPTS")
        self.assertEqual(ENVIRONMENT_VARIABLES["delivery.retry_base_delay_ms"], "TELEMETRY_RETRY_BASE_DELAY_MS")
        self.assertEqual(ENVIRONMENT_VARIABLES["delivery.retry_max_delay_ms"], "TELEMETRY_RETRY_MAX_DELAY_MS")

    def test_the_provider_hands_the_mode_and_the_three_values_to_the_delivery_settings(self):
        config = valid_config(profiles.cronjob, retry_max_attempts=7, retry_base_delay_ms=50, retry_max_delay_ms=900)
        settings = ConfigurationProvider(config, FakeIdGenerator()).configuration().delivery
        self.assertIsInstance(settings, DeliverySettings)
        self.assertEqual(
            (
                settings.mode,
                settings.retry_max_attempts,
                settings.retry_base_delay_milliseconds,
                settings.retry_max_delay_milliseconds,
            ),
            (DeliveryMode.RETRY, 7, 50, 900),
        )

    def test_a_valid_configuration_with_the_limits_of_each_setting_has_no_problem(self):
        self.assertEqual(
            problems_of(valid_config(retry_max_attempts=1, retry_base_delay_ms=1, retry_max_delay_ms=1)), []
        )
        self.assertEqual(problems_of(valid_config(retry_base_delay_ms=500, retry_max_delay_ms=500)), [])

    def test_each_value_below_one_is_a_problem_with_its_own_field(self):
        for field_name in ("retry_max_attempts", "retry_base_delay_ms", "retry_max_delay_ms"):
            for value in (0, -1):
                with self.subTest(field=field_name, value=value):
                    self.assertEqual(
                        problems_of(valid_config(**{field_name: value})),
                        [(f"delivery.{field_name}", "config_field_invalid")],
                    )

    def test_a_bool_and_a_value_that_is_not_an_integer_are_invalid_like_in_the_other_settings(self):
        for field_name in ("retry_max_attempts", "retry_base_delay_ms", "retry_max_delay_ms"):
            for value in (True, False, 1.5, "5", None):
                with self.subTest(field=field_name, value=value):
                    self.assertEqual(
                        problems_of(valid_config(**{field_name: value})),
                        [(f"delivery.{field_name}", "config_field_invalid")],
                    )

    def test_a_ceiling_below_the_base_is_a_problem_of_the_ceiling(self):
        config = valid_config(retry_base_delay_ms=500, retry_max_delay_ms=100)
        self.assertEqual(problems_of(config), [("delivery.retry_max_delay_ms", "config_field_invalid")])

    def test_an_invalid_base_does_not_make_the_ceiling_a_problem_too(self):
        config = valid_config(retry_base_delay_ms=0, retry_max_delay_ms=100)
        self.assertEqual(problems_of(config), [("delivery.retry_base_delay_ms", "config_field_invalid")])

    def test_the_problems_come_after_the_one_of_the_event_classes_and_before_the_limits(self):
        base = valid_config(
            event_classes={"x": "unknown"}, retry_max_attempts=0, retry_base_delay_ms=0, retry_max_delay_ms=0
        )
        config = dataclasses.replace(base, limits=dataclasses.replace(base.limits, max_event_bytes=0))
        self.assertEqual(
            [field for field, _ in problems_of(config)],
            ["delivery.event_classes", *RETRY_FIELDS, "limits.max_event_bytes"],
        )

    def test_a_reason_never_holds_a_value(self):
        config = valid_config(retry_max_attempts=-12345, retry_max_delay_ms=-98765)
        for problem in validate_config(config):
            self.assertNotIn("12345", problem.reason)
            self.assertNotIn("98765", problem.reason)


class ProfileModeTest(unittest.TestCase):
    """Each profile reaches the mode of REQ-CFG-004 and, through it, the behavior of that mode."""

    MODES = {
        "microservice": DeliveryMode.BEST_EFFORT,
        "cronjob": DeliveryMode.RETRY,
        "worker": DeliveryMode.RETRY,
        "robotics": DeliveryMode.DURABLE,
        "offline_device": DeliveryMode.DURABLE,
    }

    def test_every_profile_gives_its_mode_to_the_delivery_settings(self):
        self.assertEqual(set(self.MODES), set(PROFILES))
        for name, build in PROFILES.items():
            with self.subTest(profile=name):
                base = build()
                config = dataclasses.replace(
                    base,
                    resource=dataclasses.replace(base.resource, service_name="sensor-hub"),
                    spool=dataclasses.replace(base.spool, directory="spool-dir"),
                )
                settings = ConfigurationProvider(config, FakeIdGenerator()).configuration().delivery
                self.assertIs(settings.mode, self.MODES[name])

    def test_an_empty_mode_is_not_a_valid_text_but_the_delivery_settings_start_in_best_effort(self):
        self.assertIs(DeliverySettings().mode, DeliveryMode.BEST_EFFORT)
        self.assertEqual(defaults().delivery.mode, "best_effort")

    def test_the_profiles_that_are_available_reach_the_behavior_of_their_mode(self):
        """A destination that always fails with 503: one attempt with best_effort, the retry limit with retry."""
        expected_attempts = {"microservice": 1, "cronjob": 3, "worker": 3}
        for name, attempts in expected_attempts.items():
            with self.subTest(profile=name):
                sink = FakeLogSink()
                sink.set_result(SendResult(accepted=0, dropped=0, failed=1, reason="status_503"))
                base = PROFILES[name]()
                config = dataclasses.replace(
                    base,
                    resource=dataclasses.replace(base.resource, service_name="sensor-hub"),
                    delivery=dataclasses.replace(
                        base.delivery,
                        flush_interval_ms=5,
                        retry_max_attempts=3,
                        retry_base_delay_ms=1,
                        retry_max_delay_ms=2,
                    ),
                )
                core = stop_workers_on_cleanup(self, telemetry.new(config, telemetry.with_log_sink(sink)))
                core.logger.info(telemetry.LogEntry(category="http", message="one"))
                eventually(5.0, lambda: core._delivery.stats()[0].batches == 1, "the batch to end")
                stats = core._delivery.stats()[0]
                self.assertEqual((stats.attempts, stats.retries), (attempts, attempts - 1))
                self.assertEqual(sink.calls, attempts)
                if name == "microservice":
                    self.assertEqual((stats.failed, stats.dropped), (1, 0))
                else:
                    self.assertEqual((stats.failed, stats.dropped_by_reason), (0, {"retry_limit_reached": 1}))

    def test_the_durable_profiles_are_refused_at_start(self):
        for name in ("robotics", "offline_device"):
            with self.subTest(profile=name):
                base = PROFILES[name]()
                config = dataclasses.replace(
                    base,
                    resource=dataclasses.replace(base.resource, service_name="sensor-hub"),
                    spool=dataclasses.replace(base.spool, directory="spool-dir"),
                )
                self.assertEqual(validate_config(config), ())
                with self.assertRaises(telemetry.ConfigurationError) as raised:
                    telemetry.new(config)
                self.assertEqual([problem.field for problem in raised.exception.problems], ["delivery.mode"])


if __name__ == "__main__":
    unittest.main()
