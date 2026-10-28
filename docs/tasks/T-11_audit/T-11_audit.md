# T-11 — Audit events

| | |
|---|---|
| Status | todo (T-11.4 deferred) |
| Kind | epic |
| Parent | project |
| Depends on | T-03, T-05 |
| Blocks | — |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 (REQ-SEC-006, sections 1.3 and 7.2) and the answers of [Q06](../../setup/questionnaires/Q06_security-configuration-and-release.md) (Q06-6: no project needs audit events in v1; the contract is written and the adapter comes later), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (Q07-16: audit is last in the build order) and [Q02](../../setup/questionnaires/Q02_contract-format-identifiers-and-generation.md) (Q02-5: one capability at a time).

## Goal

Make `audit.*` events a type of their own, separate from diagnostic logs, that can go to a destination of their own (REQ-SEC-006), and say in the documents what the library does not guarantee. The contract and the routing are built in the first pass (Q06-6); no project needs an audit adapter in v1, so the adapter is deferred.

## What it keeps and what it changes

- Keeps: the envelope, the routing of T-05, masking before every sink (ADR-006) and the rule that a sink never fails the caller (ADR-005). The old Go logging library described in [README.md](../../../README.md) has no audit events, so nothing existing is replaced.
- Adds: the `audit.*` schemas in `contracts/schemas/`; a routing rule that sends audit events to their own destination and never mixes them with logs; the written warning that HTTP or a file do not guarantee immutability or legal retention; later, an audit adapter.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-11.1 Audit schemas](T-11.1_audit-schemas.md) | todo |
| 2 | [T-11.2 Separate routing](T-11.2_separate-routing.md) | todo |
| 3 | [T-11.3 The warning](T-11.3_the-warning.md) | todo |
| 4 | [T-11.4 Audit adapter](T-11.4_audit-adapter.md) | deferred |

Order: 1, 2, 3 in sequence (the warning describes what the routing really does). T-11.4 waits for a project that needs audit events and is not part of the first pass.

## Left out on purpose (no over-engineering)

Tamper-evident storage, hash chains, signing, legal retention, an audit query API, a second envelope for audit events, and an audit adapter before a project asks for one (REQ-SEC-006 and section 1.3 of PR-001 say the library is not an audit system with legal guarantees).

## Acceptance criteria (draft)

`audit.*` has its own schemas and its own routing in both languages, an audit event never goes to a destination only because a log does, and the documents state the limits of what it guarantees.
