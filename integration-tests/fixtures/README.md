# fixtures

The files that **both implementations read** (REQ-CNF-005): neither Go nor Python owns them. Each language has a thin test that loads the folders `envelope/` and `events/`, so **adding a case needs no code change** in either test; the cross comparison of the two languages with each other is T-12.1. The files are data: no real host, address, secret or customer name, placeholders only (`sensor-hub`, `hub-01`).

## The case format (`envelope/` and `events/`)

One JSON file is one case:

| Field | Meaning |
|---|---|
| `description` | one sentence: what the case shows |
| `kind` | how it is checked: `canonical`, `time` or `validation` |
| `input` | the case input, as a structured document |
| `expected` | the expected text, for a case that succeeds |
| `error_code` | the expected code of `contracts/errors/catalog.yaml`, for a case that must be rejected |
| `message_contains` | (`validation` only) pieces the error message must contain |

- **`canonical`**: `input` is an event (any JSON value); both languages serialize it with the JSON Canonicalization Scheme and the bytes are compared **exactly** with `expected` (not as parsed JSON). Two files with the same logical event and a different key order have the same `expected`.
- **`time`**: `input` is a time as text with an explicit offset (for example `2026-10-28T10:00:00.120+00:00`); both languages format it with the contract time format (`expected`, six decimals and `Z`) or reject it (`error_code`: an offset that is not zero is never converted).
- **`validation`**: `input` is `{ "event_type": ..., "data": ... }`; both languages validate it at the source and return no error (no `error_code`) or the code `error_code` with a message that contains every piece of `message_contains`.
- An unknown `kind` fails the test: a typo is never skipped silently.

## The other tables

| Folder | What it holds |
|---|---|
| [canonical_json/](canonical_json) | the number, string, sorting and event tables of the canonical JSON (the RFC 8785 vectors), read by the canonical JSON tests of both languages and by [compare_canonical_json.py](../compare_canonical_json.py) |
| [value_objects/](value_objects) | one table per value object of the domain (severity, actor kind, event id, UTC instant, identifier, hex digits, resource, actor, context, attributes): `{ description, cases: [ { description, input, valid } ] }`; both languages accept exactly the cases with `valid: true` and reject the others (T-03.1) |
| [event_validation/](event_validation) | the valid and invalid cases of the validation at the source, and the tables of the constructs the generated validators support |

## Adding or changing a case

- **A new case** is a new file; run the tests of both languages (`go test ./internal/tests/contract/` in `go/`; `python -m unittest internal.tests.contract.test_fixtures` in `python/`).
- **The expected text must not come from the implementation under test.** The first cases were computed with Node.js, whose `JSON.stringify` is the ECMAScript serialization that RFC 8785 is defined on and whose key sort is by UTF-16 code units.
- **A fixture is a change of the contract if it changes bytes that were already written**: an existing `expected` is changed only with the same pull request and dated ADR note as a contract change, implemented in both languages (Q01-4).
- The examples of Annex B of PR-001 are not fixtures yet: they become fixtures in T-08.5, after the device schemas exist.
