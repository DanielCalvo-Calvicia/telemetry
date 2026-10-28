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
| REQ-DST-002 | Python is one package with optional extras, installed from the Git repository. | Started (T-01.3, 2026-10-28): the package `telemetry` installs; the extras are empty |
| REQ-DST-003 | The library supports Go >= 1.25 and Python >= 3.10. | Started (T-01.2, T-01.3): both modules declare it |
| REQ-ARC-009 | Go and Python have the same layers, directory names and file names. | Started (T-01.3): the four layers and the test folders match; the rest arrives with the code |

## Out of scope

Deployment of any kind; reading environment variables or files; sending entries to AWS directly (CloudWatch is reached through aws_microservice); log rotation and retention (CloudWatch's concern).
