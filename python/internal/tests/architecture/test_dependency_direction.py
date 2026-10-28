"""The dependencies of the Python package point inward (REQ-ARC-001, REQ-ARC-002), and planted violations fail."""

import unittest

from internal.tests.architecture import checker, helpers

FIXTURE = helpers.FIXTURE_PACKAGE


class DependencyDirectionTest(unittest.TestCase):
    def test_the_real_package_keeps_the_dependencies_pointing_inward(self):
        helpers.require_no_violation(
            self,
            helpers.real_package_violations(),
            checker.RULE_DOMAIN_ISOLATED,
            checker.RULE_APPLICATION_IMPORTS_ONLY_DOMAIN,
            checker.RULE_INFRASTRUCTURE_NOT_COMPOSITION_ROOT,
            checker.RULE_IO_ONLY_IN_EXTERNAL,
        )

    def test_a_clean_tree_has_no_violation(self):
        self.assertEqual([], helpers.fixture_violations("clean"))

    def test_a_planted_violation_of_each_dependency_rule_is_reported(self):
        planted_cases = (
            (
                "domain_imports_infrastructure",
                checker.RULE_DOMAIN_ISOLATED,
                "internal/domain/bad.py",
                f"{FIXTURE}.internal.infrastructure.config",
            ),
            ("domain_imports_io", checker.RULE_DOMAIN_ISOLATED, "internal/domain/bad.py", "http.client"),
            (
                "application_imports_adapter",
                checker.RULE_APPLICATION_IMPORTS_ONLY_DOMAIN,
                "internal/application/bad.py",
                f"{FIXTURE}.internal.infrastructure.outbound.console",
            ),
            (
                "infrastructure_imports_composition_root",
                checker.RULE_INFRASTRUCTURE_NOT_COMPOSITION_ROOT,
                "internal/infrastructure/config/bad.py",
                f"{FIXTURE}.internal.composition_root.containers",
            ),
            (
                "io_outside_external",
                checker.RULE_IO_ONLY_IN_EXTERNAL,
                "internal/infrastructure/outbound/console/console.py",
                "http.client",
            ),
        )
        for case_name, rule, file, imported in planted_cases:
            with self.subTest(case=case_name):
                helpers.require_violation(self, helpers.fixture_violations(case_name), rule, file, imported)

    def test_the_io_of_an_adapter_is_allowed_in_its_external_folder(self):
        violations = checker.of_rule(
            helpers.fixture_violations("io_outside_external"), checker.RULE_IO_ONLY_IN_EXTERNAL
        )
        self.assertNotIn(
            "internal/infrastructure/outbound/console/external/client.py", [violation.file for violation in violations]
        )

    def test_a_relative_import_is_resolved_against_its_own_package(self):
        violations = helpers.fixture_violations("relative_import")
        helpers.require_violation(
            self,
            violations,
            checker.RULE_DOMAIN_ISOLATED,
            "internal/domain/bad.py",
            f"{FIXTURE}.internal.infrastructure.config",
        )
