"""The names of the HTTP destination settings: the two schemas, the code, the documentation and .env.example agree."""

import dataclasses
import json
import pathlib
import re
import unittest

from telemetry.internal.infrastructure.outbound.logging.http.env_names import HTTP_DESTINATION_ENVIRONMENT_VARIABLES
from telemetry.internal.infrastructure.outbound.logging.http.settings import Settings

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[7]
CONFIG_FOLDER = REPOSITORY_ROOT / "contracts" / "config"
DOCUMENT_PATH = REPOSITORY_ROOT / "docs" / "configuration" / "environment-variables.md"
ENV_EXAMPLE_PATH = REPOSITORY_ROOT / ".env.example"


def schema_names() -> dict[str, str]:
    """The property name to the x-env name: the properties of the client schema, then those of the destination."""
    names: dict[str, str] = {}
    for file_name in ("http_client.schema.json", "http_destination.schema.json"):
        schema = json.loads((CONFIG_FOLDER / file_name).read_text(encoding="utf-8"))
        names.update({name: property_schema["x-env"] for name, property_schema in schema["properties"].items()})
    return names


def documented_names() -> set[str]:
    """The TELEMETRY_HTTP_ names of the rows of the sections of the shared client and of the HTTP destination."""
    text = DOCUMENT_PATH.read_text(encoding="utf-8")
    found: set[str] = set()
    for heading in ("## Shared HTTP client", "## HTTP destination"):
        section = text.split(heading, 1)[1].split("\n## ", 1)[0]
        found |= {m.group(1) for m in re.finditer(r"^\| `(TELEMETRY_HTTP_[A-Z0-9_]+)`", section, re.MULTILINE)}
    return found


def env_example_names() -> set[str]:
    """The names of the lines NAME= or # NAME= with the TELEMETRY_HTTP_ prefix."""
    text = ENV_EXAMPLE_PATH.read_text(encoding="utf-8")
    return set(re.findall(r"^(?:#\s*)?(TELEMETRY_HTTP_[A-Z0-9_]+)=", text, re.MULTILINE))


class HttpDestinationEnvironmentNamesTest(unittest.TestCase):
    def assert_same_names(self, found: set[str], expected: set[str], where: str):
        self.assertEqual(sorted(expected - found), [], f"missing in {where}")
        self.assertEqual(sorted(found - expected), [], f"extra in {where}")

    def test_the_names_of_the_code_are_the_union_of_the_x_env_names_of_the_two_schemas(self):
        self.assertEqual(list(HTTP_DESTINATION_ENVIRONMENT_VARIABLES.items()), list(schema_names().items()))
        values = list(HTTP_DESTINATION_ENVIRONMENT_VARIABLES.values())
        self.assertEqual(len(set(values)), len(values))
        self.assertEqual(len(values), 12)

    def test_the_properties_are_the_fields_of_the_dataclass_in_order(self):
        fields = [item.name for item in dataclasses.fields(Settings)]
        self.assertEqual(fields, list(schema_names()))

    def test_the_table_cannot_be_changed(self):
        with self.assertRaises(TypeError):
            HTTP_DESTINATION_ENVIRONMENT_VARIABLES["url"] = "OTHER"  # type: ignore[index]

    def test_the_documented_tables_have_exactly_the_names(self):
        expected = set(HTTP_DESTINATION_ENVIRONMENT_VARIABLES.values())
        self.assert_same_names(documented_names(), expected, "docs/configuration/environment-variables.md")
        self.assert_same_names(expected, documented_names(), "the table of the code (against the document)")

    def test_env_example_has_exactly_the_names(self):
        expected = set(HTTP_DESTINATION_ENVIRONMENT_VARIABLES.values())
        self.assert_same_names(env_example_names(), expected, ".env.example")
        self.assert_same_names(expected, env_example_names(), "the table of the code (against .env.example)")
