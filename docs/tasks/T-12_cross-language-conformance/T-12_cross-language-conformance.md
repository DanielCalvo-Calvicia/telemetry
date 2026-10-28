# T-12 — Cross-language conformance

| | |
|---|---|
| Status | todo (recurring: it grows with every capability) |
| Kind | epic |
| Parent | project |
| Depends on | T-02.5 and, for each check, the capability it checks |
| Blocks | T-13.5 |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 (section 13 and REQ-ARC-008 to REQ-ARC-011) and the answers of [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (Q07-5: no Go pilot, so the cross tests protect the Go side), [Q06](../../setup/questionnaires/Q06_security-configuration-and-release.md) (Q06-7: the cross tests run in CI with the 95% gate for Python), [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (Q09-6: `1.0` only when every capability works) and [Q02](../../setup/questionnaires/Q02_contract-format-identifiers-and-generation.md) (Q02-5: one capability at a time).

## Goal

Everything common is identical in Go and Python, and a capability counts as supported only when both implementations pass its conformance tests (REQ-CNF-005 to REQ-CNF-008). There is no Go pilot (Q07-5), so these tests are the main protection of the Go side. This epic owns the runner, the gate and the report; each capability epic adds its own cross tests.

## What it keeps and what it changes

- Keeps: the shared fixtures and the registry check that T-02.7 and T-02.8 create, and the cross tests that each capability epic writes (T-06.7, T-07.5, T-10.6).
- Adds: one CI job that runs the shared fixtures through both languages and compares the results; a gate that lets `registry.yaml` mark a capability as supported only when both languages pass; a mechanical check of layers and names between the two trees; a generated maturity report.
- The old Go logging library described in [README.md](../../../README.md) has no cross-language tests, so nothing existing is replaced.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-12.1 Cross-test runner](T-12.1_cross-test-runner.md) | todo |
| 2 | [T-12.3 Mechanical parity check](T-12.3_mechanical-parity-check.md) | todo |
| 3 | [T-12.2 Registry gate](T-12.2_registry-gate.md) | todo |
| 4 | [T-12.4 Maturity report](T-12.4_maturity-report.md) | todo |

Order: 1 and 2 can run in parallel (the runner needs the fixtures of T-02.8, the parity check needs only the layout of T-01.4); 3 needs 1; 4 needs 3. The IDs keep their numbers. The epic is recurring: after every capability epic its cross tests are added to the runner and its row in the registry is moved to supported by the gate.

## Left out on purpose (no over-engineering)

A third language, a benchmark suite, a dashboard of the report, randomized or property-based cross testing, running the cross tests against a real receiver (it is a separate project, Q07-9), and a check of the text of error messages (only the codes are identical, REQ-CNF-004).

## Acceptance criteria (draft)

CI fails when Go and Python disagree on a fixture, on a name or on a state; a capability is supported in the registry only when both languages pass its tests (an adapter of one language only is "not applicable" in the other); the maturity report is generated from the registry and states the level of the library.
