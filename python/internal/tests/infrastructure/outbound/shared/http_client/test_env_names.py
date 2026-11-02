"""The names of the HTTP settings: the schema, the code, the documentation and .env.example agree."""

import dataclasses
import json
import pathlib
import re
import unittest

from telemetry.internal.infrastructure.outbound.shared.http_client.env_names import HTTP_ENVIRONMENT_VARIABLES
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import HttpSettings

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[7]
SCHEMA_PATH = REPOSITORY_ROOT / "contracts" / "config" / "http_client.schema.json"
DOCUMENT_PATH = REPOSITORY_ROOT / "docs" / "configuration" / "environment-variables.md"
ENV_EXAMPLE_PATH = REPOSITORY_ROOT / ".env.example"
DOCUMENT_SECTION = "## Shared HTTP client"


def schema_names() -> dict[str, str]:
    """The property name to the x-env name of every property of the schema, in the order of the schema."""
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    return {name: property_schema["x-env"] for name, property_schema in schema["properties"].items()}


def documented_names() -> set[str]:
    """The first backticked cell of the rows TELEMETRY_HTTP_ of the section of the shared HTTP client."""
    text = DOCUMENT_PATH.read_text(encoding="utf-8")
    section = text.split(DOCUMENT_SECTION, 1)[1].split("\n## ", 1)[0]
    return {match.group(1) for match in re.finditer(r"^\| `(TELEMETRY_HTTP_[A-Z0-9_]+)`", section, re.MULTILINE)}


def env_example_names() -> set[str]:
    """The names of the lines NAME= or # NAME= with the TELEMETRY_HTTP_ prefix."""
    text = ENV_EXAMPLE_PATH.read_text(encoding="utf-8")
    return set(re.findall(r"^(?:#\s*)?(TELEMETRY_HTTP_[A-Z0-9_]+)=", text, re.MULTILINE))


class HttpEnvironmentNamesTest(unittest.TestCase):
    def assert_same_names(self, found: set[str], expected: set[str], where: str):
        self.assertEqual(sorted(expected - found), [], f"missing in {where}")
        self.assertEqual(sorted(found - expected), [], f"extra in {where}")

    def test_the_names_of_the_code_are_the_x_env_names_of_the_schema_in_its_order(self):
        self.assertEqual(list(HTTP_ENVIRONMENT_VARIABLES.items()), list(schema_names().items()))
        self.assertEqual(len(set(HTTP_ENVIRONMENT_VARIABLES.values())), len(HTTP_ENVIRONMENT_VARIABLES))

    def test_the_properties_are_the_fields_of_the_dataclass_in_the_order_of_the_schema(self):
        fields = [item.name for item in dataclasses.fields(HttpSettings)]
        self.assertEqual(fields, list(schema_names()))

    def test_the_documented_table_has_exactly_the_names(self):
        expected = set(HTTP_ENVIRONMENT_VARIABLES.values())
        self.assert_same_names(documented_names(), expected, "docs/configuration/environment-variables.md")
        self.assert_same_names(expected, documented_names(), "HTTP_ENVIRONMENT_VARIABLES (against the document)")

    def test_env_example_has_exactly_the_names(self):
        expected = set(HTTP_ENVIRONMENT_VARIABLES.values())
        self.assert_same_names(env_example_names(), expected, ".env.example")
        self.assert_same_names(expected, env_example_names(), "HTTP_ENVIRONMENT_VARIABLES (against .env.example)")
