# T-07 — Executions, faults, reports and health

| | |
|---|---|
| Status | todo |
| Kind | epic |
| Parent | project |
| Depends on | T-03, T-05 |
| Blocks | T-08, T-09 |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 (sections 6.4 to 6.6) and the answers of [Q02](../../setup/questionnaires/Q02_contract-format-identifiers-and-generation.md) (question 5: `execution.*`, `fault.*` and `report.*` come first after logs), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (question 16: the build order) and [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (question 6: `1.0` only when every capability works).

## Goal

The common capabilities that devices (T-08) and AI agents (T-09) reuse: one execution abstraction with its states and transitions (REQ-EXE-001 to REQ-EXE-005), the failure model and the generic report (REQ-FLT-001, REQ-RPT-001) and health with heartbeats (REQ-HLT-001 to REQ-HLT-003). Everything is built in Go and in Python with the same names and the same events.

## What it keeps and what it changes

- Keeps: the call context of the old logging library, already migrated to the new core by T-03.8, which carries the execution id; the sink ports and the delivery of T-05, through which every new event travels; the rule that a failing destination never fails the caller (ADR-005) and that masking comes before any destination (ADR-006).
- Changes: the old Go library in `pkg/logging` (README section 1) knows only log entries, so it has no execution, fault, report or health concept. This epic adds them as new event families in `contracts/` and as services of the new core, without touching `pkg/logging` (T-04.7 removes it).

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-07.1 Execution model and transitions](T-07.1_execution-model-and-transitions.md) | todo |
| 2 | [T-07.2 `execution.*` events and API](T-07.2_execution-events-and-api.md) | todo |
| 3 | [T-07.3 Faults and generic reports](T-07.3_faults-and-generic-reports.md) | todo |
| 4 | [T-07.4 Health and heartbeats](T-07.4_health-and-heartbeats.md) | todo |
| 5 | [T-07.5 Cross tests](T-07.5_cross-tests.md) | todo |

Order: 1, then 2; 3 depends only on T-02.3 and 4 only on T-03.9, so both can run in parallel with 1 and 2; 5 closes the epic when 1 to 4 are done. T-08 and T-09 wait for this epic (T-09.2 needs T-07.2 in particular).

## Left out on purpose (no over-engineering)

An HTTP endpoint for health (REQ-HLT-002), a scheduler or a job runner (the library reports executions, the consumer runs them), automatic retries of the consumer's work (the library records `retrying`, it does not retry), a cost or SLA calculation, and an alerting rule engine.

## Acceptance criteria (draft)

A cronjob, a worker and an agent run report their executions in one shape, and both languages reject the same invalid transitions. Faults, reports and health use the same events and fields in both languages, an exporter failure never makes the application unhealthy by itself, and every document that describes these capabilities is true again.
