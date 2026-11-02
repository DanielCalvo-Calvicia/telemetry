"""The public package exports the names both languages must export (REQ-ARC-010) and documents all of them."""

import inspect
import json
import pathlib
import unittest

import telemetry

NAMES_FILE = (
    pathlib.Path(__file__).resolve().parents[4] / "integration-tests" / "fixtures" / "core" / "public_names.json"
)


def python_names() -> list[str]:
    """The Python names of the shared list: the functions and the types."""
    document = json.loads(NAMES_FILE.read_text(encoding="utf-8"))
    names = [entry["python"] for entry in document["functions"] + document["types"]]
    if not names:
        raise AssertionError("the shared list of public names is empty")
    return names


class PublicNamesTest(unittest.TestCase):
    def test_every_shared_name_is_exported_and_importable(self):
        for name in python_names():
            with self.subTest(name=name):
                self.assertIn(name, telemetry.__all__)
                self.assertTrue(hasattr(telemetry, name))

    def test_every_exported_name_exists_and_has_a_docstring(self):
        for name in telemetry.__all__:
            with self.subTest(name=name):
                self.assertTrue(hasattr(telemetry, name))
                self.assertTrue(inspect.getdoc(getattr(telemetry, name)), f"{name} has no docstring")

    def test_the_package_has_a_docstring_and_no_duplicate_names(self):
        self.assertTrue(telemetry.__doc__)
        self.assertEqual(len(telemetry.__all__), len(set(telemetry.__all__)))

    def test_the_entry_point_names(self):
        self.assertEqual(telemetry.new.__name__, "new_core")
        self.assertEqual(telemetry.Telemetry.__name__, "Core")


if __name__ == "__main__":
    unittest.main()
