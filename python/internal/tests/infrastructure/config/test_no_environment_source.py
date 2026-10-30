"""The library never reads the environment (ADR-004, REQ-ARC-007): a grep of the sources, comments included."""

import pathlib
import re
import unittest

SOURCE_ROOT = pathlib.Path(__file__).resolve().parents[4] / "src" / "telemetry"
FORBIDDEN = re.compile(r"\b(?:os\.environ|os\.getenv|getenv|dotenv|environ)\b")


class NoEnvironmentSourceTest(unittest.TestCase):
    def test_no_source_file_mentions_how_to_read_the_environment(self):
        paths = [path for path in SOURCE_ROOT.rglob("*.py") if "tests" not in path.parts]
        self.assertTrue(paths, f"no source files found in {SOURCE_ROOT}")
        for path in paths:
            for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
                match = FORBIDDEN.search(line)
                self.assertIsNone(match, f"{path}:{number} mentions {match and match.group(0)!r}")

    def test_the_pattern_finds_what_it_must(self):
        for text in ("os.environ['A']", "os.getenv('A')", "from os import getenv", "import dotenv", "environ.get"):
            self.assertIsNotNone(FORBIDDEN.search(text), text)
        for text in ("environment", "ENVIRONMENT_VARIABLES", "TELEMETRY_ENVIRONMENT"):
            self.assertIsNone(FORBIDDEN.search(text), text)
