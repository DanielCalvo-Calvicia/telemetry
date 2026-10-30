"""Helpers of the masking tests: the shared fixture tables and the fake credentials."""

import json
import pathlib
from typing import Any

# The tables are shared with the Go tests: both languages read the SAME files, so they cannot drift apart.
REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[6]
MASKING_FIXTURES = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "masking"

# All the credentials below are fake: the examples published in the AWS documentation or obvious placeholders.
FAKE_ACCESS_KEY_ID = "AKIAIOSFODNN7EXAMPLE"
FAKE_SECRET_KEY = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
FAKE_JWT = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.c2lnbmF0dXJl"
FAKE_PRIVATE_KEY = "-----BEGIN RSA PRIVATE KEY-----\nFAKEFAKEFAKEFAKE\nFAKEFAKEFAKEFAKE\n-----END RSA PRIVATE KEY-----"


def load_table(file_name: str) -> list[dict[str, Any]]:
    """Read the cases of a shared table; a missing folder or file, or an empty table, fails the test."""
    if not MASKING_FIXTURES.is_dir():
        raise AssertionError(f"the fixtures folder {MASKING_FIXTURES} does not exist")
    content = json.loads((MASKING_FIXTURES / file_name).read_text(encoding="utf-8"))
    cases = content["cases"]
    if not cases:
        raise AssertionError(f"the table {file_name} has no case")
    return cases
