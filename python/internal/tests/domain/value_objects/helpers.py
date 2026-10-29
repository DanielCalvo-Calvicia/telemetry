"""Helpers of the value object tests: the shared fixture tables of both languages."""

import json
import pathlib
from typing import Any

# The tables are shared with the Go tests: both languages read the SAME files, so they cannot drift apart.
REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[5]
FIXTURES_DIRECTORY = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "value_objects"


def load_cases(file_name: str) -> list[dict[str, Any]]:
    """Read the cases of a shared fixture file; a missing, empty or case-less file fails the test."""
    content = json.loads((FIXTURES_DIRECTORY / file_name).read_text(encoding="utf-8"))
    cases = content["cases"]
    if not cases:
        raise AssertionError(f"the fixture {file_name} has no case")
    return cases
