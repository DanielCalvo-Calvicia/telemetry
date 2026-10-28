"""The paths of the real package and of the fixtures, and the matching of a violation (architecture tests)."""

import pathlib
import unittest

from internal.tests.architecture import checker

TESTS_ROOT = pathlib.Path(__file__).resolve().parents[1]
REAL_PACKAGE_ROOT = TESTS_ROOT.parents[1] / "src" / "telemetry"
REAL_EXAMPLES_ROOT = TESTS_ROOT.parents[1] / "examples"
FIXTURE_PACKAGE = "example_pkg"


def real_package_violations() -> list[checker.Violation]:
    """Check the real package, ``python/src/telemetry``."""
    return checker.check(REAL_PACKAGE_ROOT, "telemetry", REAL_EXAMPLES_ROOT)


def fixture_violations(case_name: str) -> list[checker.Violation]:
    """Check one planted-violation tree of the fixtures."""
    case_root = TESTS_ROOT / "fixtures" / "architecture" / case_name
    return checker.check(case_root, FIXTURE_PACKAGE, case_root / "examples")


def require_violation(test: unittest.TestCase, violations, rule: str, file: str, imported: str) -> None:
    """Fail unless the violations hold one of the rule on the file with the import, and its message names both."""
    for violation in checker.of_rule(violations, rule):
        if violation.file == file and violation.imported == imported:
            test.assertIn(file, violation.message())
            test.assertIn(imported, violation.message())
            return
    test.fail(f"no violation of {rule} on {file} importing {imported} in {violations}")


def require_no_violation(test: unittest.TestCase, violations, *rules: str) -> None:
    """Fail if the violations hold any of the rules, listing them by message."""
    for rule in rules:
        found = checker.of_rule(violations, rule)
        if found:
            test.fail("unexpected violations:\n" + "\n".join(violation.message() for violation in found))
