# T-04 — Logging and the core adapters

| | |
|---|---|
| Status | todo |
| Kind | epic |
| Parent | project |
| Depends on | T-03 |
| Blocks | T-05 to T-13 |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 and the answers of [Q01](../../setup/questionnaires/Q01_scope-repository-layout-and-rollout.md) (the old API disappears with no deprecation period), [Q03](../../setup/questionnaires/Q03_delivery-receiver-and-spool.md) (the receiver is a separate project; tests use a fake), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (Q07-9 receiver, Q07-10 the AWS sink stays as an adapter), [Q08](../../setup/questionnaires/Q08_last-points-before-rewriting-the-proposal.md) (Q08-6 authentication, Q08-7 batch body, Q08-9 limits) and [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (Q09-2 and Q09-3: the project chooses its adapters in code; one module per optional adapter). Decisions: [ADR-011](../../decisions/ADR-011-adopt-the-telemetry-standard-go-python.md) and [ADR-013](../../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md).

## Goal

Logging on the new standard (`log.entry.recorded`), the three adapters of the core (console, file, HTTP) with the shared HTTP infrastructure, the AWS HTTP adapter as a module of its own, and the removal of the old `pkg/logging`.

## What it keeps and what it changes

- Keeps: what the old library does for a project. Structured logging with console, HTTP and AWS HTTP sinks, masking before every sink (ADR-006), the call context, the rule that a failing sink never fails the caller (ADR-005) and the rule that the library never reads the environment (ADR-004). Today this is `pkg/logging` and `internal/logging` (see [README.md](../../../README.md)).
- Changes: an entry with fixed fields (`Application`, `Category`, `Function`, `Message`) becomes the `log.entry.recorded` event of the envelope (REQ-LOG-007); the console, file and HTTP adapters move into the core under `internal/infrastructure/outbound/`, on top of the ports of T-03; the HTTP client, TLS, authentication, retries and response classification are written once per language (REQ-OUT-006), so the HTTP sink and the AWS sink stop having their own copies; the AWS sink becomes a module of its own with its own tag; the old surface is removed in T-04.7, with no deprecation period (Q01-3, Q01-5).
- New: the file adapter, the Python side of everything, the fake receiver for tests.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-04.1 Logger and the `log.entry.recorded` event](T-04.1_logger-and-the-log-entry-recorded-event.md) | todo |
| 2 | [T-04.4 Shared HTTP infrastructure](T-04.4_shared-http-infrastructure.md) | todo |
| 3 | [T-04.2 Console adapter](T-04.2_console-adapter.md) | todo |
| 4 | [T-04.3 File adapter](T-04.3_file-adapter.md) | todo |
| 5 | [T-04.5 HTTP destination](T-04.5_http-destination.md) | todo |
| 6 | [T-04.8 Fake receiver for tests](T-04.8_fake-receiver-for-tests.md) | todo |
| 7 | [T-04.6 AWS HTTP adapter](T-04.6_aws-http-adapter.md) | todo |
| 8 | [T-04.7 Replace and remove the old library](T-04.7_replace-and-remove-the-old-library.md) | todo |

Order: T-04.1 and T-04.4 are independent (T-04.4 only needs T-03.5, so it can start as soon as the limits exist). T-04.2 and T-04.3 need only T-04.1 and run in parallel with each other and with T-04.4. T-04.5, T-04.6 and T-04.8 need T-04.4 and can run in parallel. T-04.7 goes last: it needs every other task of the epic, because it is the moment the old API disappears. T-05.1 can start when T-04.5 is done, without waiting for T-04.7.

## Left out on purpose (no over-engineering)

A synchronous send mode (the queue is T-05); a deprecation period or a compatibility wrapper for the old API (Q01-3); OpenTelemetry and Prometheus logging adapters (T-10, deferred); reading the environment or a file to configure an adapter (ADR-004); self-registering adapters (REQ-ARC-013); a global logger instance on by default (REQ-ARC-006).

## Acceptance criteria (draft)

A project logs through console, file and HTTP in Go and in Python and gets the same `log.entry.recorded` bytes from both languages; the AWS HTTP adapter is a module of its own that builds and tests alone; `pkg/logging` and `internal/logging` are gone; every document that described the old library is true again.
