"""The names of the AWS HTTP settings: the schema, the code, the documentation and .env.example agree."""

import dataclasses
import json
import pathlib
import re
import unittest

from telemetry.internal.infrastructure.outbound.logging.aws_http.env_names import AWS_HTTP_ENVIRONMENT_VARIABLES
from telemetry.internal.infrastructure.outbound.logging.aws_http.settings import Settings

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[7]
SCHEMA_PATH = REPOSITORY_ROOT / "contracts" / "config" / "aws_http.schema.json"
DOCUMENT_PATH = REPOSITORY_ROOT / "docs" / "configuration" / "environment-variables.md"
ENV_EXAMPLE_PATH = REPOSITORY_ROOT / ".env.example"
PREFIX = "TELEMETRY_AWS_HTTP_"


def schema_names() -> dict[str, str]:
    """The property name to the x-env name, in the order of the schema."""
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    return {name: property_schema["x-env"] for name, property_schema in schema["properties"].items()}


def documented_names() -> set[str]:
    """The TELEMETRY_AWS_HTTP_ names of the rows of the section of the AWS HTTP adapter."""
    text = DOCUMENT_PATH.read_text(encoding="utf-8")
    section = text.split("## AWS HTTP adapter", 1)[1].split("\n## ", 1)[0]
    return {m.group(1) for m in re.finditer(rf"^\| `({PREFIX}[A-Z0-9_]+)`", section, re.MULTILINE)}


def env_example_names() -> set[str]:
    """The names of the lines NAME= or # NAME= with the TELEMETRY_AWS_HTTP_ prefix."""
    text = ENV_EXAMPLE_PATH.read_text(encoding="utf-8")
    return set(re.findall(rf"^(?:#\s*)?({PREFIX}[A-Z0-9_]+)=", text, re.MULTILINE))


class AwsHttpEnvironmentNamesTest(unittest.TestCase):
    def assert_same_names(self, found: set[str], expected: set[str], where: str):
        self.assertEqual(sorted(expected - found), [], f"missing in {where}")
        self.assertEqual(sorted(found - expected), [], f"extra in {where}")

    def test_the_names_of_the_code_are_the_x_env_names_of_the_schema(self):
        self.assertEqual(list(AWS_HTTP_ENVIRONMENT_VARIABLES.items()), list(schema_names().items()))
        values = list(AWS_HTTP_ENVIRONMENT_VARIABLES.values())
        self.assertEqual(len(set(values)), len(values))
        self.assertEqual(len(values), 13)
        self.assertTrue(all(value.startswith(PREFIX) for value in values))

    def test_the_properties_are_the_fields_of_the_dataclass_in_order(self):
        fields = [item.name for item in dataclasses.fields(Settings)]
        self.assertEqual(fields, list(schema_names()))

    def test_the_table_cannot_be_changed(self):
        with self.assertRaises(TypeError):
            AWS_HTTP_ENVIRONMENT_VARIABLES["host"] = "OTHER"  # type: ignore[index]

    def test_the_documented_table_has_exactly_the_names(self):
        expected = set(AWS_HTTP_ENVIRONMENT_VARIABLES.values())
        self.assert_same_names(documented_names(), expected, "docs/configuration/environment-variables.md")
        self.assert_same_names(expected, documented_names(), "the table of the code (against the document)")

    def test_env_example_has_exactly_the_names(self):
        expected = set(AWS_HTTP_ENVIRONMENT_VARIABLES.values())
        self.assert_same_names(env_example_names(), expected, ".env.example")
        self.assert_same_names(expected, env_example_names(), "the table of the code (against .env.example)")
