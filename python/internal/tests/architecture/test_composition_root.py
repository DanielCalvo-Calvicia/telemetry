"""Only a composition root imports a concrete outbound adapter (REQ-ARC-003)."""

import unittest

from internal.tests.architecture import checker, helpers

FIXTURE = helpers.FIXTURE_PACKAGE


class CompositionRootTest(unittest.TestCase):
    def test_the_real_package_wires_the_adapters_only_in_the_composition_root(self):
        helpers.require_no_violation(
            self, helpers.real_package_violations(), checker.RULE_ADAPTERS_WIRED_BY_COMPOSITION_ROOT
        )

    def test_an_adapter_imported_outside_a_composition_root_is_reported(self):
        helpers.require_violation(
            self,
            helpers.fixture_violations("adapter_outside_composition_root"),
            checker.RULE_ADAPTERS_WIRED_BY_COMPOSITION_ROOT,
            "internal/infrastructure/config/bad.py",
            f"{FIXTURE}.internal.infrastructure.outbound.console",
        )

    def test_a_composition_root_may_import_an_adapter(self):
        helpers.require_no_violation(
            self, helpers.fixture_violations("clean"), checker.RULE_ADAPTERS_WIRED_BY_COMPOSITION_ROOT
        )
