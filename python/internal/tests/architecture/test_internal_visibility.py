"""A consumer of the library never imports an internal package."""

import pathlib
import tempfile
import unittest

from internal.tests.architecture import checker, helpers

FIXTURE = helpers.FIXTURE_PACKAGE


class InternalVisibilityTest(unittest.TestCase):
    def test_the_real_package_has_no_consumer_that_imports_internal(self):
        helpers.require_no_violation(
            self, helpers.real_package_violations(), checker.RULE_CONSUMER_NEVER_IMPORTS_INTERNAL
        )

    def test_a_consumer_that_imports_an_internal_package_is_reported(self):
        helpers.require_violation(
            self,
            helpers.fixture_violations("consumer_imports_internal"),
            checker.RULE_CONSUMER_NEVER_IMPORTS_INTERNAL,
            "examples/smoke/main.py",
            f"{FIXTURE}.internal.domain",
        )

    def test_a_consumer_that_imports_only_the_public_package_is_accepted(self):
        helpers.require_no_violation(
            self, helpers.fixture_violations("clean"), checker.RULE_CONSUMER_NEVER_IMPORTS_INTERNAL
        )

    def test_a_folder_that_does_not_exist_has_no_violation(self):
        with tempfile.TemporaryDirectory() as empty_folder:
            empty_root = pathlib.Path(empty_folder)
            self.assertEqual([], checker.check(empty_root, FIXTURE, empty_root / "examples"))
