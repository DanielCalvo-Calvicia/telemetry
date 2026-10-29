"""Helpers of the event validation tests: the shared fixture tables and the generated fixture validators."""

import json
import pathlib
import types
from typing import Any

# The tables are shared with the Go tests: both languages read the SAME files, so they cannot drift apart. T-02.8
# moves them to integration-tests/fixtures/.
REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
FIXTURES_DIRECTORY = REPOSITORY_ROOT / "go" / "internal" / "tests" / "fixtures" / "event_validation"

# The Python validators that tools/contract_generator wrote for its own fixture contract (demo.thing.happened), which
# uses every construct the validation supports: an enum, a nested object, attributes, an integer, a number, a boolean,
# string limits and an open object.
GENERATED_FIXTURE_VALIDATORS = (
    REPOSITORY_ROOT
    / "tools"
    / "contract_generator"
    / "internal"
    / "tests"
    / "fixtures"
    / "expected"
    / "python"
    / "event_validators_generated.py.golden"
)


def load_cases(relative_path: str) -> list[dict[str, Any]]:
    """Read the cases of a shared fixture file; an empty or missing file fails the test."""
    content = json.loads((FIXTURES_DIRECTORY / relative_path).read_text(encoding="utf-8"))
    cases = content["cases"]
    if not cases:
        raise AssertionError(f"the fixture {relative_path} has no case")
    return cases


def load_generated_fixture_validators() -> types.ModuleType:
    """Run the generated validators of the fixture contract as a module, so the generated code itself is tested."""
    module = types.ModuleType("generated_fixture_validators")
    source = GENERATED_FIXTURE_VALIDATORS.read_text(encoding="utf-8")
    exec(compile(source, str(GENERATED_FIXTURE_VALIDATORS), "exec"), module.__dict__)
    return module
