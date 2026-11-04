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
| [building/](building) | one case per file of the event builder (T-03.3): `{ description, kind: "build", setup, input, expected \| error_code }`; the setup gives the fixed clock, the event id and the resource, the input is what a capability gives the builder, and `expected` is the exact canonical JSON of the envelope (bytes, computed with Node.js) or the error code; both languages must give the same bytes |
| [masking/](masking) | the tables of the masker (T-03.4): `text.json`, `field_names.json` and `values.json` have the expected values produced by the **masker of the old library** as oracle (`{ description, oracle, cases: [ { description, input, expected } ] }`), `text_unicode_folding.json` the cases where the case folding of Go and Python must agree, and `envelopes/` one case per file `{ description, kind: "mask", options?, input, expected \| error_code }` for the new options and the whole envelope; both languages must give the same text |
| [limits/](limits) | the cases of the limits (T-03.5), the expected values computed with a reference algorithm in Node.js: one case per file `{ description, kind: "limits", options?, input, report, expected \| error_code }` (the limited envelope as exact canonical JSON and the four counters, or the error code) and `batch.json` for `fits_in_batch`; both languages must give the same envelope, the same counters and the same errors |
| [config/](config) | the cases of the configuration (T-03.6), the expected values computed with a reference model in Node.js: one case per file `{ description, kind: "config", profile, overrides?, expected \| problems }`; `expected` is the exact canonical JSON of the effective configuration (defaults, then the profile, then the overrides), `problems` the exact list of `{ field, code }` in the order of the schema; both languages must give the same |
| [errors/](errors) | `codes.json` (T-03.7): 29 failures that the core produces, each with the code the catalog gives it; each language has a producer of the same name that makes the REAL failure and the test checks the code, that a secret-looking input value is not in the message, that the set of names equals the set of producers and that every code of the catalog is used |
| [call_context/](call_context) | the call context (T-03.8): `ids.json` (one field and one value, valid or not: `trace_id` 32 and `span_id` 16 lowercase hexadecimal digits and not all zeros, the other fields identifiers) and `mapping.json` (what a call context gives the envelope context, as exact canonical JSON or null); both languages must accept and reject the same and give the same bytes |
| [core/](core) | the whole core through its public entry point (T-03.9): 15 cases `{ description, kind: "core", config, clock, event_ids, sinks, call_context?, emits, expected \| new_error }` with the exact canonical JSON each sink receives, the result of each emit and the problems of a refused start; and `public_names.json`, the names both languages export (REQ-ARC-010) |
| [logging/](logging) | the logger (T-04.1): 10 direct-call cases (`kind: "log"`: calls to `trace` ... `critical` with category, function, message and fields), 5 native-handler cases (`kind: "handler"`: a `slog` record in Go, a `logging` record in Python, with the same expected event) and `handler_levels.json` (`kind: "levels"`: the severity of each native level in each language); the expected bytes are exact canonical JSON computed independently |
| [http/](http) | the pure parts of the shared HTTP client (T-04.4), the expected values computed independently with Node.js: `batch_body.json` (exact canonical bytes of the batch; the gzipped body must decompress to them), `response_classification.json`, `retry_after.json`, `backoff.json`, `auth_header.json` and `settings.json` (the effective settings or the problems with their field and code) |
| [console/](console) | the console destination (T-04.2), expected values computed independently with Node.js: `lines.json` (the exact bytes written for a list of events in the json and the text format, with the minimum severity, and the result of the send: 11 cases), `settings.json` (the effective settings or the problems) and `public_names.json` (the public names of `go/pkg/logging/console` and `telemetry.logging.console`) |
| [file/](file) | the file destination (T-04.3), expected values computed with a reference simulation in Node.js: `scenarios.json` (9 end-to-end cases: the content of every file of the directory and the counters after the batches), `rotation_plan.json` and `quota_plan.json` (the pure plans), `settings.json` and `public_names.json` |
| [delivery/batching_cases.json](delivery/batching_cases.json) | the rule of the batcher (T-05.1): 39 cases (9 named and 30 seeded) of events with their arrival times and the batches the worker must make, with the algorithm of an independent model written in the description |
| [delivery/full_queue_cases.json](delivery/full_queue_cases.json), [delivery/classification_cases.json](delivery/classification_cases.json) | what a full queue does with a new event for each policy and class (54 cases, 40 of them seeded) and the class of an event by type, severity and configuration (13 cases), computed by an independent model (T-05.2) |
| [delivery/shutdown_results.json](delivery/shutdown_results.json) | what flush and shutdown report (T-05.4): 8 cases of destinations that are healthy, slow, hanging, failing or dropping, with the events sent and dropped and whether the call returns before its timeout, computed by an independent model |
| [delivery/mode_scenarios.json](delivery/mode_scenarios.json) | what a worker does with the answers of a destination by delivery mode (T-05.3): 10 cases (best_effort sends once; retry sends again after a transient failure, never a permanent rejection, honors the `Retry-After` hint and drops at the limit) with the attempts and the counters |
| [http_destination/](http_destination) | the HTTP destination (T-04.5), expected values computed independently with Node.js: `requests.json` (13 cases: the exact batch body a local receiver gets, the headers, the filtering by severity, the status the receiver answers and the result returned: accepted, failed for a transient failure, dropped for a permanent rejection), `settings.json` (the 12 settings of the destination: the client's and `minimum_severity`) and `public_names.json` |
| [http/receiver_script_examples.json](http/receiver_script_examples.json) | the fake receiver (T-04.8): 14 cases of a script of answers (statuses, `Retry-After`, a redirect, a delay, a closed connection, a truncated answer, a rejected body with its reason, gzip, authentication) that the fake of each language must answer the same way and record the same |
| [aws_http/](aws_http) | the AWS HTTP adapter (T-04.6): 8 events and the exact `add_log` request each becomes (6 translated, 2 skipped), the settings cases and the public names; expected values computed independently |
| [event_validation/](event_validation) | the valid and invalid cases of the validation at the source, and the tables of the constructs the generated validators support |

## Adding or changing a case

- **A new case** is a new file; run the tests of both languages (`go test ./internal/tests/contract/` in `go/`; `python -m unittest internal.tests.contract.test_fixtures` in `python/`).
- **The expected text must not come from the implementation under test.** The first cases were computed with Node.js, whose `JSON.stringify` is the ECMAScript serialization that RFC 8785 is defined on and whose key sort is by UTF-16 code units.
- **A fixture is a change of the contract if it changes bytes that were already written**: an existing `expected` is changed only with the same pull request and dated ADR note as a contract change, implemented in both languages (Q01-4).
- The examples of Annex B of PR-001 are not fixtures yet: they become fixtures in T-08.5, after the device schemas exist.
