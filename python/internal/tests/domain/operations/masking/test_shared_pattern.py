"""The translation of the Go patterns for Python: the places where the two engines would differ."""

import re
import unittest

from telemetry.internal.domain.operations.masking.shared_pattern import compile_shared_pattern, translate_pattern


class SharedPatternTest(unittest.TestCase):
    def test_the_flags(self):
        text, flags = translate_pattern("(?i)(?s)ab")
        self.assertEqual(text, "ab")
        self.assertTrue(flags & re.IGNORECASE and flags & re.DOTALL and flags & re.ASCII)
        _, flags = translate_pattern("(?m)a")
        self.assertTrue(flags & re.MULTILINE)
        _, flags = translate_pattern("a")
        self.assertEqual(flags, re.ASCII)

    def test_whitespace_is_the_go_set_only(self):
        pattern = compile_shared_pattern(r"a\sb")
        self.assertIsNotNone(pattern.search("a\tb"))
        self.assertIsNotNone(pattern.search("a b"))
        self.assertIsNone(pattern.search("a\x0bb"))  # the vertical tab is not whitespace in Go
        self.assertIsNone(pattern.search("a\u00a0b"))
        inside = compile_shared_pattern(r"a[^\s:]b")
        self.assertIsNone(inside.search("a\nb"))
        self.assertIsNotNone(inside.search("a\x0bb"))
        self.assertIsNone(compile_shared_pattern(r"a[\s]b").search("a\x0bb"))

    def test_not_whitespace(self):
        pattern = compile_shared_pattern(r"a\Sb")
        self.assertIsNotNone(pattern.search("a\x0bb"))
        self.assertIsNone(pattern.search("a b"))

    def test_word_boundaries_are_ascii(self):
        pattern = compile_shared_pattern(r"\bkey\b")
        self.assertIsNotNone(pattern.search("\u00f1key"))
        self.assertIsNone(pattern.search("xkey"))

    def test_the_end_of_the_text_is_the_very_end(self):
        self.assertIsNone(compile_shared_pattern("a$").search("a\n"))
        self.assertIsNotNone(compile_shared_pattern("a$").search("a"))
        self.assertIsNone(compile_shared_pattern(r"a\z").search("a\n"))
        self.assertIsNotNone(compile_shared_pattern("(?m)a$").search("a\n"))
        self.assertIsNotNone(compile_shared_pattern("[$]").search("$"))

    def test_a_closing_bracket_first_in_a_class_is_a_member(self):
        self.assertIsNotNone(compile_shared_pattern("[]a]+").fullmatch("]a]"))
        self.assertIsNotNone(compile_shared_pattern("[^]a]").fullmatch("b"))
        self.assertIsNone(compile_shared_pattern("[^]a]").fullmatch("]"))

    def test_an_escape_is_kept_and_a_trailing_backslash_is_invalid(self):
        self.assertIsNotNone(compile_shared_pattern(r"a\.b").search("a.b"))
        self.assertIsNone(compile_shared_pattern(r"a\.b").search("axb"))
        with self.assertRaises(re.error):
            compile_shared_pattern("a\\")

    def test_an_invalid_pattern_raises(self):
        with self.assertRaises(re.error):
            compile_shared_pattern("(")

    def test_the_kelvin_sign_and_the_long_s_fold_like_in_go(self):
        keyword = compile_shared_pattern(r"(?i)key=")
        self.assertIsNotNone(keyword.search("Key="))
        self.assertIsNotNone(keyword.search("KEY="))
        self.assertIsNone(compile_shared_pattern(r"key=").search("Key="))  # not case-insensitive
        sharp = compile_shared_pattern(r"(?i)\bpass(?:word|)\b")
        self.assertIsNotNone(sharp.search("paſſword"))
        self.assertIsNone(sharp.search("paſſ"))  # \b stays ASCII: the long s is not a word character

    def test_a_class_that_covers_s_or_k_gets_the_folded_characters(self):
        for text, covered in [
            ("[a-z]", ["ſ", "K"]),
            ("[A-Z0-9]", ["ſ", "K"]),
            ("[s]", ["ſ"]),
            ("[K]", ["K"]),
            ("[x-z_.-]", []),
            ("[0-9._~+/=-]", []),
        ]:
            with self.subTest(text):
                pattern = compile_shared_pattern("(?i)^" + text + "$")
                for character in ("ſ", "K"):
                    self.assertEqual(pattern.search(character) is not None, character in covered, character)

    def test_a_negated_class_excludes_the_folded_characters_too(self):
        pattern = compile_shared_pattern("(?i)^[^a-z]$")
        self.assertIsNone(pattern.search("ſ"))
        self.assertIsNone(pattern.search("s"))
        self.assertIsNotNone(pattern.search("1"))
        self.assertIsNotNone(compile_shared_pattern(r"(?i)^[^\s,]$").search("ſ"))

    def test_escapes_group_names_and_flags_are_not_folded(self):
        self.assertIsNone(compile_shared_pattern(r"(?i)\ds").search("1x"))
        self.assertIsNotNone(compile_shared_pattern(r"(?i)\ds").search("1S"))
        named = compile_shared_pattern(r"(?i)(?P<skey>s)(?P=skey)")
        self.assertIsNotNone(named.search("ss"))
        self.assertIsNotNone(compile_shared_pattern(r"(?i)(?:s)(?i:k)").search("ſk"))
        self.assertIsNone(compile_shared_pattern(r"(?i)\d").search("ſ"))

    def test_an_unterminated_class_is_invalid(self):
        with self.assertRaises(re.error):
            compile_shared_pattern("(?i)[abc")
