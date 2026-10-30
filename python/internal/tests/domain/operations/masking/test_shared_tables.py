"""The shared tables of masking: text, field names and values, read by the Go tests too."""

import unittest

from internal.tests.domain.operations.masking.helpers import MASKING_FIXTURES, load_table
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.masking.is_sensitive_field_name import is_sensitive_field_name
from telemetry.internal.domain.operations.masking.sensitive_data_masker import new_sensitive_data_masker


class SharedTablesTest(unittest.TestCase):
    def test_the_fixtures_folder_exists_and_has_the_three_tables(self):
        self.assertTrue(MASKING_FIXTURES.is_dir())
        for name in ("text.json", "text_unicode_folding.json", "field_names.json", "values.json"):
            self.assertTrue((MASKING_FIXTURES / name).is_file(), name)

    def test_text(self):
        masker = new_sensitive_data_masker()
        for case in load_table("text.json"):
            with self.subTest(case["description"]):
                self.assertEqual(masker.mask_text(case["input"]), case["expected"])

    def test_text_unicode_folding(self):
        masker = new_sensitive_data_masker()
        for case in load_table("text_unicode_folding.json"):
            with self.subTest(case["description"]):
                self.assertEqual(masker.mask_text(case["input"]), case["expected"])

    def test_field_names(self):
        for case in load_table("field_names.json"):
            with self.subTest(case["description"]):
                self.assertIs(is_sensitive_field_name(case["input"]), case["expected"])

    def test_values(self):
        masker = new_sensitive_data_masker()
        for case in load_table("values.json"):
            with self.subTest(case["description"]):
                actual = masker.mask_value(case["input"]["field_name"], case["input"]["value"])
                self.assertEqual(canonical_json(actual), canonical_json(case["expected"]))
