# T-03 — Core domain and application

| | |
|---|---|
| Status | todo |
| Kind | epic |
| Parent | project |
| Depends on | T-02 |
| Blocks | T-04 to T-13 |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 and the answers of [Q02](../../setup/questionnaires/Q02_contract-format-identifiers-and-generation.md) (UUIDv7, the log entry as an event, JSON `snake_case` and the Go standard), [Q06](../../setup/questionnaires/Q06_security-configuration-and-release.md) (the library never reads the environment, the redaction list), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (synchronous Python API, PEP 8 and the Go layers, Q07-7 and Q07-8), [Q08](../../setup/questionnaires/Q08_last-points-before-rewriting-the-proposal.md) (size limits, Q08-9) and [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (adapters chosen in code, Q09-2). Decision records: [ADR-004](../../decisions/ADR-004-library-never-reads-the-environment.md), [ADR-006](../../decisions/ADR-006-masking-before-every-sink.md), [ADR-012](../../decisions/ADR-012-two-times-in-every-event.md), [ADR-013](../../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md).

## Goal

The part of the library that every capability shares, in Go and in Python with the same layers and names: value objects, ports, the building of an event, masking, limits, configuration, errors, the call context and the public entry point. The Go code follows the Go Clean/Hexagonal standard of the organization; the Python code follows the same layers with PEP 8 naming (Q07-8).

## What it keeps and what it changes

- Keeps, by migrating the code of the old library (README.md): the masking detectors of ADR-006 (`internal/logging/domain/operations`: keys, bearer credentials, JWTs, private key blocks, credential pairs, connection-string passwords, sensitive field names), the call context (application, instance, process id, indentation) and its call depth, the clock and id-generator ports, and the rule that the library never reads the environment (ADR-004). The old code is read, not edited; it stays until T-04.7 removes it.
- Changes: new code in `go/internal/` and `python/src/telemetry/internal/`; the entry fields `Application`, `Category`, `Function` and `Message` become an envelope plus a `log.entry.recorded` event (Q02-4); the ports grow from three to the list of REQ-OUT; masking adds the default redaction list (`password`, `token`, `authorization`, `api_key`, `secret`) without removing a detector (Q06-5); configuration becomes a validated model with profiles instead of the old `TelemetryConfig`.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-03.1 Value objects and base value objects](T-03.1_value-objects-and-base-value-objects.md) | done (2026-10-29) |
| 2 | [T-03.2 Ports](T-03.2_ports.md) | done (2026-10-29) |
| 3 | [T-03.3 Building an event](T-03.3_building-an-event.md) | todo |
| 4 | [T-03.4 Masking and redaction](T-03.4_masking-and-redaction.md) | todo |
| 5 | [T-03.5 Limits](T-03.5_limits.md) | todo |
| 6 | [T-03.6 Configuration and profiles](T-03.6_configuration-and-profiles.md) | todo |
| 7 | [T-03.7 Error model](T-03.7_error-model.md) | todo |
| 8 | [T-03.8 Call context](T-03.8_call-context.md) | todo |
| 9 | [T-03.9 Core composition root and the public entry point](T-03.9_core-composition-root-and-the-public-entry-point.md) | todo |

Order: 1, 2, 3; then 4 and 5 (both need 3); 6 and 7 only need T-02.4 and T-02.3, so they can start as soon as the generator exists, in parallel with 1 to 3; 8 needs 2; 9 closes the epic and needs 2 to 8. Go and Python are done together for each subtask, so the two never drift. Go tests live under `go/internal/tests/` and are black-box; the coverage gate is 95% without `external/`. Apply the `go-architecture-standard` skill before writing Go here.

## Left out on purpose (no over-engineering)

Real adapters (T-04 and later), a global logger (REQ-ARC-006 allows an optional documented one; it is not built here), reading a file or the environment (ADR-004), a plugin registry (adapters register by being passed in, REQ-ARC-013), a dependency-injection framework, asynchronous Python (Q07-7: the API is synchronous).

## Acceptance criteria (draft)

The core builds and passes its tests in both languages; the architecture tests of T-01.4 pass on it; an event built in Go and in Python has the same fields and, serialized by T-02.5, the same bytes; no code path logs an entry unmasked; the old library still builds.
