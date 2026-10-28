# ADR-011 — Adopt the telemetry standard: one contract for Go and Python

**Status:** Proposed. Decided by the maintainer in the questionnaires Q01 to Q09 (2026-10-27 and 2026-10-28); it becomes Accepted with [PR-001](../proposals/PR-001-telemetry-standard-go-python.md). Nothing is built.
**Related:** [PR-001](../proposals/PR-001-telemetry-standard-go-python.md), [ADR-001](ADR-001-adopt-shared-go-standard.md), [ADR-004](ADR-004-library-never-reads-the-environment.md), [ADR-012](ADR-012-two-times-in-every-event.md), [ADR-013](ADR-013-go-modules-per-adapter-chosen-in-code.md)

## Context

The library only logs. The projects that will use it include hardware on a Raspberry Pi and an AI agent, both in Python, and Go services. Data from a Go agent and from Python sensor software is shown in the same interface, so any difference between the two languages produces display errors.

## Decision

The library becomes the telemetry standard of all the projects: logs, metrics, traces, executions, health, device data, AI agent data and generic reports, in Go and in Python.

- One repository holds `contracts/` (JSON Schema, the single source of truth), `go/` and `python/`. The code generated from the contract is committed.
- Everything common is identical in both languages: envelope, schemas, enums, error codes, configuration model, execution transitions, layer and adapter names, spool format, delivery behavior and the JSON bytes.
- The library is passive: it does not decide what is captured, the data policy or the destination.
- It has no hardware drivers. A project keeps its own drivers and reports through typed source helpers that all produce the same generic event.
- `1.0` is released only when every capability works; until then the library is a beta (`0.x`).
- The ADRs 004 (never reads the environment), 005 (a failing sink never fails the caller) and 006 (masking before every sink) stay.

## Consequences

- Positive: one contract and one UI for every project; a Python or Go project adopts only what it needs.
- Negative: a very large scope; `1.0` is far away; the Go side is built without a Go pilot and tested with the cross tests and fakes; the receiver of the HTTP events is a separate project that does not exist yet.
