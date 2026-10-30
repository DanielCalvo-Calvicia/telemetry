# Requirements

Status as of 2026-10-27. **Done** means implemented and verified in the repository (tests pass); **Planned** means decided and not built. Where a row names a record, it is the ADR that explains the decision.

## Functional: logging

| ID | Requirement | Status | Record |
|---|---|---|---|
| FR-1 | A logger with one method per severity (`Trace`, `Debug`, `Info`, `Warn`, `Error`, `Fatal`); `Fatal` logs and does not stop the process. | Done | ADR-003 |
| FR-2 | An entry carries the producer, application, instance and process ids, category, function, indentation, depth, message and free fields; the level comes from the method and the timestamp from the logger. | Done | ADR-003 |
| FR-3 | The minimum level follows the environment: `development` from DEBUG up, `production` from INFO up; TRACE is never sent; there is no level variable; an environment without a level is an error at construction. | Done | ADR-007 |
| FR-4 | A call context (application, instance, process id, indentation) is carried in `context.Context`; the logger takes those four values from it when it is present; a helper starts the context of a request and another goes one layer deeper. | Done | ADR-003 |
| FR-5 | Every entry is masked before any sink: AWS access keys, bearer credentials, JWTs, private key blocks, passwords in connection strings, credential pairs and the value of sensitive field names are replaced by `*****MASKED*****`. | Done | ADR-006 |
| FR-6 | A consumer adds masking patterns (`LOGGER_MASK_EXTRA_PATTERNS`, separated by `;`); an invalid pattern stops the construction. | Done | ADR-006 |
| FR-7 | Console sink: one JSON line per entry on the standard output. | Done | |
| FR-8 | HTTP sink: posts the flat JSON entry to a generic receiver at `host:port/path`; the three values are required. | Done | |
| FR-9 | AWS HTTP sink: adapts the entry to the `add_log` contract of aws_microservice (`log_group_name`, `log_stream_name`, `message`, `timestamp` in ms) and posts it; the five values are required. | Done | ADR-008 |
| FR-10 | A failing sink never fails the caller or the other sinks; the failure is reported to the console without repeating the message or the fields; the HTTP sinks time out after 3 s and never retry. | Done | ADR-005 |
| FR-11 | The only error a log call returns is a failure to build the entry. | Done | ADR-005 |
| FR-12 | The library never reads the environment: `Config` carries the env tags and the consumer loads it. | Done | ADR-004 |
| FR-13 | The public API is `pkg/logging` only (aliases and thin functions); a consumer never imports `internal/`. | Done | ADR-003 |
| FR-14 | A ready-made constructor per sink (`NewConsoleSink`, `NewHTTPSink`, `NewAWSHTTPSink`), the `Sink` interface for sinks of the consumer, and `NewWithSinks` to build the logger over them. | Done | ADR-009 |
| FR-15 | Metrics (counters, gauges, histograms) behind outbound ports, with one real adapter, in `internal/metrics` and `pkg/metrics`. | Planned (the new standard, [PR-001](../proposals/PR-001-telemetry-standard-go-python.md), moves metrics to their own turn in the build order) | ADR-002 |
| FR-16 | A queue or a disk buffer for entries that could not be delivered. | Planned: proposed in [PR-001](../proposals/PR-001-telemetry-standard-go-python.md) (bounded queue, three delivery modes, SQLite spool); accepted on 2026-10-28, not built | ADR-005 |

## Functional: delivery and documentation

| ID | Requirement | Status | Record |
|---|---|---|---|
| FR-17 | The library is released by git tags on `main` (`v0.x.y`) and is public; there is no deployment. | Done (v0.1.0, v0.2.0) | ADR-010 |
| FR-18 | CI checks every Go module and the Python package (format, vet or lint, build, tests, a coverage gate of 95%) with a read-only token and actions pinned by SHA. | Done for the file and its commands (T-01.5, 2026-10-28; they pass locally); the GitHub run was not checked | ADR-010 |
| FR-19 | A runnable sample (`examples/aws_http_smoke`) and a test that sends one line to a deployed aws_microservice (`internal/tests/e2e`). | Done | |
| FR-20 | The documents of `docs/` describe what the code does; every change updates them (see `CLAUDE.md`). | Done | |

## Non-functional

| ID | Requirement | Status |
|---|---|---|
| NFR-1 | The layers follow the shared Go standard: domain imports no other layer; only the composition root imports the concrete outbound adapters; the I/O of an adapter is in its `external/` folder. | Done |
| NFR-2 | Every test lives under `internal/tests/` (and `pkg/logging` for the public API), black-box (`package x_test`). | Done |
| NFR-3 | Coverage of `internal/`, excluding `external/` folders, is at least 95%. | Done (98.4% on 2026-10-20) |
| NFR-4 | The only third-party dependency is `github.com/google/uuid`. | Done |
| NFR-5 | A log call never blocks longer than the slowest sink's timeout (3 s) and never panics because of a sink. | Done |
| NFR-6 | No secret, internal address or customer name in the repository; placeholders only. | Done (checklist in security.md) |
| NFR-7 | Code, comments and documents are in English. | Done |

## The new standard (PR-001, accepted, not built)

Only the requirements whose state moved are listed; the full list is in [PR-001](../proposals/PR-001-telemetry-standard-go-python.md) and the tasks in [docs/tasks](../tasks/README.md).

| ID | Requirement | Status |
|---|---|---|
| REQ-DST-001 | Go is distributed as one core module and one module per adapter, joined by a committed `go.work` file. | Started (T-01.2, 2026-10-28): the core module `go/` and `go.work` exist; no adapter yet |
| REQ-ARC-012 | The public packages follow `<functionality>/<adapter>`; the core is `go/pkg/telemetry`. | Planned (T-03.9) |
| REQ-ARC-001, REQ-ARC-002 | Dependencies point inward; the domain imports no other layer and no I/O, database, backend or AI package. | Built for the empty layers (T-01.4, 2026-10-28): Go `Test_the_real_module_keeps_the_dependencies_pointing_inward`, Python `DependencyDirectionTest` |
| REQ-ARC-003 | The wiring happens only in a composition root. | Built for the empty layers (T-01.4): Go `Test_the_real_module_wires_the_adapters_only_in_the_composition_root`, Python `CompositionRootTest` |
| REQ-ARC-008 | Both implementations have architecture tests that verify the direction of dependencies. | Built (T-01.4); they run in the `go` and `python` jobs of CI (T-01.5) |
| REQ-CTR-001, REQ-CTR-005, REQ-CTR-009, REQ-CTR-010 | The versioned envelope; the identities of resource, actor, execution and correlation are separate; `event_id` is a UUIDv7; the attributes accept scalars and lists of scalars within limits. | Schema written (T-02.2, 2026-10-28): `contracts/schemas/envelope.schema.json` and `common/`; checked with valid and invalid samples; the code that enforces it is T-02.6 and T-03. **Built (T-03.1, 2026-10-29):** the value objects of the identities (REQ-CTR-005), the UUIDv7 check (REQ-CTR-009) and the scalar rule of the attributes (REQ-CTR-010) in both languages; the limits of size are T-03.5 and the building of the whole envelope is T-03.3 |
| REQ-LOG-007, REQ-CTR-002, REQ-CNF-004, REQ-FLT-001 | A log call is the event `log.entry.recorded`; every event type has a schema for its `data`; the error codes are identical in both languages; failures are classified by code, category, severity and recoverable. | Contract written (T-02.3, 2026-10-28): `contracts/schemas/log/entry_recorded.schema.json`, `contracts/events/registry.yaml`, `contracts/errors/catalog.yaml` (5 codes), `contracts/schemas/common/failure.schema.json`; checked with valid and invalid samples; the check over them is T-02.4 and the code is T-04.1 and T-03.7 |
| REQ-CNF-003, REQ-CTR-004 | The JSON of the same logical event is byte-for-byte identical in both languages (RFC 8785), with the fixed time format. | Built in both languages (T-02.5, 2026-10-28): `canonical_json` in the domain of Go and Python; checked on shared fixtures (85 cases, also against Node.js) and a byte comparison of the full events; integers beyond 2^53 are rejected (the plan's default) |
| REQ-CTR-002, REQ-CTR-003 | An event whose data does not validate against the schema of its type is rejected at the source with a classified error and never throws to the caller; the types are validated against the schema. | Built in both languages (T-02.6, 2026-10-29): `event_validation` in the domain, generated from the schemas with no library in the core; 24 shared invalid cases give the same code and message in both languages; not called from a logging path yet (T-03.3) |
| REQ-CNF-007, REQ-CNF-008 and section 2 of PR-001 | Every requirement has a conformance test; a capability is supported only when both languages pass; the library maturity is that of the least complete capability. | Built (T-02.7, 2026-10-29): `contracts/capabilities/registry.yaml` and `tools/registry_check`, run by the CI job `registry-check`; of 130 requirements 19 are covered by tests, 103 are pending on a task and 8 have no task yet (2026-10-30, T-03.4); no capability is supported; maturity is beta |
| REQ-CTR-007, REQ-OUT-001 | The clock and the identifier generator sit behind ports; ports with identical semantics may share an abstraction, different payloads are not forced into an untyped one. | Built (T-03.2, 2026-10-29): ten outbound ports in both languages (`application/ports/outbound/`), a fake of each; the six sink ports share one method shape and stay separate interfaces. The adapters are T-03.9 and later |
| REQ-CTR-001, REQ-CTR-008 | Every event has the versioned envelope; two instants in UTC, `recorded_at` only from the clock, `occurred_at` given or the time of the call or `observed_at`, and the clock attributes when known. | Built (T-03.3, 2026-10-29): `EventBuilderService` in both languages; the nine cases of `integration-tests/fixtures/building/` give the same canonical bytes in Go and Python. The envelope is not yet validated as a whole against the envelope schema, and it is not masked until T-03.4 and T-03.9 |
| REQ-SEC-001, REQ-SEC-002, REQ-SEC-003 | Configurable redaction by field name and by pattern before any output; a default list that is added to the detectors of ADR-006; optional classification per field with the rules drop, redact and allow. | Built for the core (T-03.4, 2026-10-30): the masker and `MaskingService` in both languages, the detectors ported unchanged (the old library is the oracle of the shared tables), the default list, extra names and patterns, classification. REQ-SEC-002 and REQ-SEC-003 are covered by tests. REQ-SEC-001 waits for T-03.9: "before any output" is true only when the chain is wired |
| REQ-CNF-005 | There is a set of shared fixtures that both implementations run with the same expected results. | Built for the first events (T-02.8, 2026-10-29): `integration-tests/fixtures/` (10 envelope and event cases, plus the canonical JSON and validation tables), read by `go/internal/tests/contract/fixtures_test.go` and `python/internal/tests/contract/test_fixtures.py` |
| REQ-ARC-010 | Public names are equivalent between the languages by a mechanical rule. | Written (T-02.1, 2026-10-28): `contracts/conventions/naming.md`; the check is T-12.3 |
| REQ-CNF-001, REQ-CNF-002, REQ-DST-005 | `contracts/` is the single source of truth; enums, event types, error codes and the envelope types are generated from it in each language; the generated code is committed and CI fails on a difference. | Built (T-02.4, 2026-10-28): `tools/contract_generator` and the `generated-code` job; the configuration schema is not generated yet (T-03.6) |
| REQ-VER-001 | The library and the contract are versioned separately (Go core, each Go adapter, the Python package, `schema_version`); a contract change is implemented and released in both languages. | Written (T-01.6, 2026-10-28): the tag shapes and the release rule are in the deployment document; nothing is tagged |
| REQ-DST-002 | Python is one package with optional extras, installed from the Git repository. | Started (T-01.3, 2026-10-28): the package `telemetry` installs; the extras are empty |
| REQ-DST-003 | The library supports Go >= 1.25 and Python >= 3.10. | Started (T-01.2, T-01.3): both modules declare it |
| REQ-ARC-009 | Go and Python have the same layers, directory names and file names. | Started (T-01.3): the four layers and the test folders match; the rest arrives with the code |

## Out of scope

Deployment of any kind; reading environment variables or files; sending entries to AWS directly (CloudWatch is reached through aws_microservice); log rotation and retention (CloudWatch's concern).
