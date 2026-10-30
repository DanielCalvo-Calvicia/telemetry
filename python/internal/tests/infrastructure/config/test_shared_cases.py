"""The shared configuration cases of integration-tests/fixtures/config, read by the Go tests too."""

import dataclasses
import json
import pathlib
import unittest

from internal.tests.fakes.fake_id_generator import FakeIdGenerator
from internal.tests.infrastructure.config.helpers import PROFILES, merged_config
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.config.provider import ConfigurationProvider
from telemetry.internal.infrastructure.config.validate import validate_config

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[5]
CONFIG_FOLDER = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "config"


class SharedConfigCasesTest(unittest.TestCase):
    def test_every_shared_case_gives_the_expected_result(self):
        self.assertTrue(CONFIG_FOLDER.is_dir(), f"the folder {CONFIG_FOLDER} does not exist")
        paths = sorted(CONFIG_FOLDER.glob("*.json"))
        self.assertTrue(paths, f"no fixture files found in {CONFIG_FOLDER}")
        for path in paths:
            with self.subTest(case=path.name):
                case = json.loads(path.read_text(encoding="utf-8"))
                self.assertEqual(case["kind"], "config", "unknown kind")
                self.assertIn(case["profile"], PROFILES, "unknown profile")
                config = merged_config(case["profile"], case.get("overrides", {}))
                if "expected" in case:
                    self._check_valid_case(case, config)
                else:
                    self._check_invalid_case(case, config)

    def _check_valid_case(self, case, config):
        self.assertEqual(validate_config(config), ())
        self.assertEqual(canonical_json(dataclasses.asdict(config)).decode("utf-8"), case["expected"])

    def _check_invalid_case(self, case, config):
        expected = [(problem["field"], problem["code"]) for problem in case["problems"]]
        self.assertTrue(expected)
        found = [(problem.field, problem.code) for problem in validate_config(config)]
        self.assertEqual(found, expected)
        with self.assertRaises(ConfigurationError) as raised:
            ConfigurationProvider(config, FakeIdGenerator())
        self.assertEqual([(problem.field, problem.code) for problem in raised.exception.problems], expected)
