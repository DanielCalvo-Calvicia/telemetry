# integration-tests

The fixtures and the cross-language tests that both implementations run with the same expected results (REQ-CNF-005, REQ-CNF-006). It is empty for now: T-02.8 creates the first fixtures and T-12 the runner.

See [PR-001](../docs/proposals/PR-001-telemetry-standard-go-python.md) and [T-12](../docs/tasks/T-12_cross-language-conformance/T-12_cross-language-conformance.md).

## What exists

- [compare_canonical_json.py](compare_canonical_json.py): the seed of the cross-language runner (T-12.1). It runs a Go test that writes the canonical JSON of the shared event fixtures, serializes the same events in Python (as a dict and as the generated `Envelope` dataclass) and compares the bytes with each other and with the expected text. It needs `go` and Python on the path and uses only the standard library: `python integration-tests/compare_canonical_json.py`. The CI job `cross-tests` does not run it yet (T-12.1).
- [fixtures/](fixtures/README.md): the shared fixtures that both implementations read (the case format is in its README). `envelope/` and `events/` hold one case per file, checked by a thin test in each language (`go/internal/tests/contract/fixtures_test.go`, `python/internal/tests/contract/test_fixtures.py`), so adding a case needs no code change; `canonical_json/` and `event_validation/` hold the tables of the canonical JSON and of the validation at the source (moved here from `go/internal/tests/fixtures/` by T-02.8).
