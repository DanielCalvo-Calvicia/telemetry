# T-02 — The contract and the generator

| | |
|---|---|
| Status | in-progress (T-02.1 to T-02.4 done 2026-10-28) |
| Kind | epic |
| Parent | project |
| Depends on | T-01 |
| Blocks | T-03 to T-13 |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 and the answers of [Q02](../../setup/questionnaires/Q02_contract-format-identifiers-and-generation.md) (JSON Schema, UUIDv7, one small generator in Go, the log entry as an event, all event types built one at a time), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (generated code is committed, Q07-4), [Q08](../../setup/questionnaires/Q08_last-points-before-rewriting-the-proposal.md) and [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (two times in every event, the storage layout of the SQLite destination). Decision record: [ADR-011](../../decisions/ADR-011-adopt-the-telemetry-standard-go-python.md) and [ADR-012](../../decisions/ADR-012-two-times-in-every-event.md).

## Goal

Write `contracts/`, the single source of truth between the two languages (JSON Schema, D-02), and the small Go generator that turns it into committed code for Go and Python (REQ-CNF-001 to REQ-CNF-003, REQ-DST-005). At the end both languages serialize the first events to the same bytes and CI fails when the committed code drifts from the contract.

## What it keeps and what it changes

- Keeps: nothing of the contract exists today. The old library has an `Entry` with `Application`, `Category`, `Function` and `Message` (README.md); it is not changed here. Its mapping to a `log.entry.recorded` event (Q02-4) is a schema written here and used from T-04.
- Changes: adds `contracts/` (schemas, enums, capabilities registry, naming rules, storage layout), the generator, generated code in `go/` and `python/`, canonical JSON and validation at the source in both languages, and the first shared fixtures under `integration-tests/`.
- The configuration schema (REQ-CNF-002) and the error codes (REQ-CNF-004) are generated from the same folder; see the open points in T-02.1 and T-02.3.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-02.1 Layout of `contracts/`](T-02.1_layout-of-contracts.md) | done (2026-10-28) |
| 2 | [T-02.2 Envelope schema](T-02.2_envelope-schema.md) | done (2026-10-28) |
| 3 | [T-02.3 Event types and error codes](T-02.3_event-types-and-error-codes.md) | done (2026-10-28) |
| 4 | [T-02.4 The generator](T-02.4_the-generator.md) | done (2026-10-28) |
| 5 | [T-02.5 Canonical JSON](T-02.5_canonical-json.md) | todo |
| 6 | [T-02.6 Validation at the source](T-02.6_validation-at-the-source.md) | todo |
| 7 | [T-02.7 Capabilities registry and its check](T-02.7_capabilities-registry-and-its-check.md) | todo |
| 8 | [T-02.8 Shared fixtures](T-02.8_shared-fixtures.md) | todo |

Order: 1, 2, 3, 4; then 5, 6 and 7 (5 and 6 need 4; 7 only needs 1, so it can start right after 1 and run in parallel with 2 to 4); then 8 (needs 5). T-03 can start when T-02.4 is done, in parallel with 5 to 8. A contract change needs a pull request and a dated note in an ADR (Q01-4).

## Left out on purpose (no over-engineering)

A schema registry service, a third-party code generator for each language (Q02-3 chose one small generator), schemas for device and AI events (they arrive in T-08 and T-09), a contract versioning tool beyond `schema_version`, a generator for the public API (only enums, event types, error codes, envelope types and the configuration schema).

## Acceptance criteria (draft)

`contracts/` validates itself; the generator is deterministic and its output is committed; both languages serialize the first events to the same bytes; CI fails on a drift between the contract and the committed code; a Python install never needs Go.
