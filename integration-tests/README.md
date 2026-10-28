# integration-tests

The fixtures and the cross-language tests that both implementations run with the same expected results (REQ-CNF-005, REQ-CNF-006). It is empty for now: T-02.8 creates the first fixtures and T-12 the runner.

See [PR-001](../docs/proposals/PR-001-telemetry-standard-go-python.md) and [T-12](../docs/tasks/T-12_cross-language-conformance/T-12_cross-language-conformance.md).

## What exists

- [compare_canonical_json.py](compare_canonical_json.py): the seed of the cross-language runner (T-12.1). It runs a Go test that writes the canonical JSON of the shared event fixtures, serializes the same events in Python (as a dict and as the generated `Envelope` dataclass) and compares the bytes with each other and with the expected text. It needs `go` and Python on the path and uses only the standard library: `python integration-tests/compare_canonical_json.py`. The CI job `cross-tests` does not run it yet (T-12.1).
- The shared fixture tables of the canonical JSON live for now in `go/internal/tests/fixtures/canonical_json/` (`numbers.json`, `strings.json`, `sorting.json`, `events.json`); the Python tests read those same files. T-02.8 moves them here.
