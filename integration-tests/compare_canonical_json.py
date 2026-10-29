"""Compare the canonical JSON of Go and Python on the shared events fixtures, byte for byte.

The seed of the cross-language runner of T-12.1. It uses the standard library only:

1. It asks Go for the canonical bytes of every case of integration-tests/fixtures/canonical_json/events.json by running
   the test TestWriteCanonicalFixtures, which writes one file per case to a temporary folder.
2. It serializes the same cases in Python, from the plain dict and from the generated Envelope dataclass.
3. It compares both with the expected text of the fixture, and with each other.

Run it from anywhere: python integration-tests/compare_canonical_json.py (it needs the go command on the PATH).
It exits with 0 when every case is identical and with 1 otherwise.
"""

import json
import os
import pathlib
import subprocess
import sys
import tempfile

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[1]
GO_MODULE = REPOSITORY_ROOT / "go"
EVENTS_FIXTURE = REPOSITORY_ROOT / "integration-tests" / "fixtures" / "canonical_json" / "events.json"

sys.path.insert(0, str(REPOSITORY_ROOT / "python" / "src"))
sys.path.insert(0, str(REPOSITORY_ROOT / "python"))

from internal.tests.domain.operations.canonical_json.helpers import envelope_from_dict  # noqa: E402
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json  # noqa: E402


def run_go(out_directory: pathlib.Path) -> None:
    """Make Go write the canonical bytes of every events case to out_directory."""
    environment = dict(os.environ, CANONICAL_JSON_OUT_DIR=str(out_directory))
    command = [
        "go",
        "test",
        "./internal/tests/domain/operations/canonical_json",
        "-run",
        "^TestWriteCanonicalFixtures$",
        "-count=1",
    ]
    completed = subprocess.run(command, cwd=GO_MODULE, env=environment, capture_output=True, text=True, check=False)
    if completed.returncode != 0:
        print(completed.stdout + completed.stderr)
        raise SystemExit("the Go side failed")


def main() -> int:
    """Run the comparison and print one line per case; return the exit code."""
    cases = json.loads(EVENTS_FIXTURE.read_text(encoding="utf-8"))["cases"]
    with tempfile.TemporaryDirectory() as temporary:
        out_directory = pathlib.Path(temporary)
        run_go(out_directory)
        failures = 0
        for case in cases:
            expected = case["expected"].encode("utf-8")
            go_bytes = (out_directory / f"{case['name']}.json").read_bytes()
            python_from_dict = canonical_json(case["input"])
            python_from_dataclass = canonical_json(envelope_from_dict(case["input"]))
            same = go_bytes == python_from_dict == python_from_dataclass == expected
            failures += 0 if same else 1
            print(f"{'OK  ' if same else 'FAIL'} {case['name']}: {len(go_bytes)} bytes")
            if not same:
                print(f"  expected: {expected!r}")
                print(f"  go:       {go_bytes!r}")
                print(f"  python:   {python_from_dict!r}")
                print(f"  python (dataclass): {python_from_dataclass!r}")
    print(f"{len(cases) - failures} of {len(cases)} cases are identical in Go and Python")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
