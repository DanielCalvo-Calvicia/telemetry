"""The names of the environment variables: the schema, the code, the documentation and .env.example agree."""

import dataclasses
import json
import pathlib
import re
import unittest

from telemetry.internal.infrastructure.config.config import Config
from telemetry.internal.infrastructure.config.defaults import defaults
from telemetry.internal.infrastructure.config.env_names import ENVIRONMENT_VARIABLES

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[5]
SCHEMA_PATH = REPOSITORY_ROOT / "contracts" / "config" / "config.schema.json"
DOCUMENT_PATH = REPOSITORY_ROOT / "docs" / "configuration" / "environment-variables.md"
ENV_EXAMPLE_PATH = REPOSITORY_ROOT / ".env.example"
DOCUMENT_SECTION = "## Core configuration of the new standard"


def schema_names() -> dict[str, str]:
    """The dotted path to the x-env name of every property of the schema."""
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    names = {}
    for section_name, section in schema["properties"].items():
        for property_name, property_schema in section["properties"].items():
            names[f"{section_name}.{property_name}"] = property_schema["x-env"]
    return names


def config_paths() -> set[str]:
    """The dotted paths of the fields of the Config dataclasses."""
    example = defaults()
    return {
        f"{section.name}.{item.name}"
        for section in dataclasses.fields(Config)
        for item in dataclasses.fields(getattr(example, section.name))
    }


def documented_names() -> set[str]:
    """The first backticked cell of the rows TELEMETRY_ of the section of the new standard."""
    text = DOCUMENT_PATH.read_text(encoding="utf-8")
    section = text.split(DOCUMENT_SECTION, 1)[1].split("\n## ", 1)[0]
    return {match.group(1) for match in re.finditer(r"^\| `(TELEMETRY_[A-Z0-9_]+)`", section, re.MULTILINE)}


def env_example_names() -> set[str]:
    """The names of the lines NAME= or # NAME= with the TELEMETRY_ prefix."""
    text = ENV_EXAMPLE_PATH.read_text(encoding="utf-8")
    return set(re.findall(r"^(?:#\s*)?(TELEMETRY_[A-Z0-9_]+)=", text, re.MULTILINE))


class EnvironmentNamesTest(unittest.TestCase):
    def assert_same_names(self, found: set[str], expected: set[str], where: str):
        self.assertEqual(sorted(expected - found), [], f"missing in {where}")
        self.assertEqual(sorted(found - expected), [], f"extra in {where}")

    def test_the_paths_of_the_schema_the_code_and_the_names_are_the_same(self):
        schema = set(schema_names())
        self.assert_same_names(set(ENVIRONMENT_VARIABLES), schema, "ENVIRONMENT_VARIABLES (paths)")
        self.assert_same_names(config_paths(), schema, "the Config dataclasses (paths)")
        self.assert_same_names(schema, set(ENVIRONMENT_VARIABLES), "the schema (paths)")
        self.assert_same_names(schema, config_paths(), "the schema (against the dataclasses)")

    def test_the_names_of_the_code_are_the_x_env_names_of_the_schema(self):
        self.assertEqual(dict(ENVIRONMENT_VARIABLES), schema_names())
        self.assertEqual(len(set(ENVIRONMENT_VARIABLES.values())), len(ENVIRONMENT_VARIABLES))

    def test_the_documented_table_has_exactly_the_names(self):
        expected = set(ENVIRONMENT_VARIABLES.values())
        self.assert_same_names(documented_names(), expected, "docs/configuration/environment-variables.md")
        self.assert_same_names(expected, documented_names(), "ENVIRONMENT_VARIABLES (against the document)")

    def test_env_example_has_exactly_the_names(self):
        expected = set(ENVIRONMENT_VARIABLES.values())
        self.assert_same_names(env_example_names(), expected, ".env.example")
        self.assert_same_names(expected, env_example_names(), "ENVIRONMENT_VARIABLES (against .env.example)")
