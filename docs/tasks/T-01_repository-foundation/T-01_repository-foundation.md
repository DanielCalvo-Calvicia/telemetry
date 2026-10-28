# T-01 — Repository foundation

| | |
|---|---|
| Status | in-progress (T-01.1 to T-01.4 done 2026-10-28) |
| Kind | epic |
| Parent | project |
| Depends on | PR-001 version 0.2 accepted (done, T-01.1) |
| Blocks | T-02 to T-13 |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 and the answers of [Q01](../../setup/questionnaires/Q01_scope-repository-layout-and-rollout.md) (one repository), [Q06](../../setup/questionnaires/Q06_security-configuration-and-release.md) (CI, coverage and releases), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (where `go.mod` lives, the first version, how Python is installed) and [Q08](../../setup/questionnaires/Q08_last-points-before-rewriting-the-proposal.md) and [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (one module per adapter). Decision records: [ADR-011](../../decisions/ADR-011-adopt-the-telemetry-standard-go-python.md) and [ADR-013](../../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md).

## Goal

Turn the repository into the layout of the standard: `contracts/`, `go/` (the core module, and later one module per adapter), `python/` and `integration-tests/`, with a CI that checks both languages. Nothing of the new library is written here; it is the ground the rest stands on.

## What it keeps and what it changes

- Keeps: the old Go logging library described in [README.md](../../../README.md) (`pkg/logging`, `internal/logging`, the root `go.mod` of module `github.com/DanielCalvo-Calvicia/telemetry`, tags `v0.1.0` and `v0.2.0`), working and tested, until T-04.7 removes it (Q01-5: no deprecation period, its only consumer takes it from the published package). The rules of `CLAUDE.md` stay: the library never reads the environment (ADR-004), a sink never fails the caller (ADR-005), masking before every sink (ADR-006), tests under `internal/tests/`, public repository.
- Changes: a new Go module under `go/` (`github.com/DanielCalvo-Calvicia/telemetry/go`, first tag `go/v0.3.0`, Q07-1, Q07-2), a Python project under `python/` installed from Git (Q07-3), the directories `contracts/` and `integration-tests/`, architecture tests in both languages, a CI for both, and the tag rule of REQ-VER-001.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-01.1 Accept PR-001 version 0.2](T-01.1_accept-pr-001-version-0-2.md) | done (2026-10-28) |
| 2 | [T-01.2 Repository layout](T-01.2_repository-layout.md) | done (2026-10-28) |
| 3 | [T-01.3 Python project skeleton](T-01.3_python-project-skeleton.md) | done (2026-10-28) |
| 4 | [T-01.4 Architecture tests](T-01.4_architecture-tests.md) | done (2026-10-28) |
| 5 | [T-01.5 CI](T-01.5_ci.md) | in-progress (the GitHub run waits for a pull request) |
| 6 | [T-01.6 Tags and the release rule](T-01.6_tags-and-the-release-rule.md) | todo |

Order: 1, 2, 3; then 4, 5 and 6 (4 and 5 need 2 and 3; 6 only needs 2, so it can run in parallel with 3). The old library keeps working throughout, so nobody is left without a logger while the new one is built. The e2e test of the deployed AWS microservice keeps its switch `run_against_deployed_service` set to `false` in everything that is committed.

## Left out on purpose (no over-engineering)

Moving the old library into `go/` (it stays where it is until T-04.7 removes it), a package index for Python (Q07-3: installed from Git), a release automation (a tag is asked for, T-13.4), a dependency-update bot, a Windows or macOS matrix in CI.

## Acceptance criteria (draft)

The layout exists and builds in both languages; the architecture tests and the CI pass; the old library still builds and its tests pass; the release rule is written; every document that describes the layout is true.
