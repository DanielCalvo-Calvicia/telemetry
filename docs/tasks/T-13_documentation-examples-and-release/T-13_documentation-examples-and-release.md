# T-13 — Documentation, examples and release

| | |
|---|---|
| Status | todo (T-13.1 is recurring: it runs after every epic) |
| Kind | epic |
| Parent | project |
| Depends on | T-01 and, for the release, T-02 to T-12 |
| Blocks | — |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 (sections 15 and 17) and the answers of [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (Q09-6: `1.0` only when every capability works; until then every release is a `0.x` beta), [Q06](../../setup/questionnaires/Q06_security-configuration-and-release.md) (Q06-8: one tag per language; the repository is public), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (Q07-1 to Q07-4: the Go module under `go/`, tags `go/vX.Y.Z`, Python installed from Git) and [Q01](../../setup/questionnaires/Q01_scope-repository-layout-and-rollout.md) (Q01-3: the old surface disappears with no deprecation period).

## Goal

Keep every document true while the standard is built (the rule of `CLAUDE.md`), give each capability a runnable example in both languages, and release. `1.0` is released only when every capability works (Q09-6); until then every release is a `0.x` beta. A tag is permanent and is created only when the maintainer asks.

## What it keeps and what it changes

- Keeps: the documents that exist today (architecture, API, configuration, requirements, security, monitoring, deployment, the README) and the release flow of [ADR-010](../../decisions/ADR-010-no-deployment-release-by-tags.md), tags on `main` after the checks pass.
- Changes: each document is rewritten for the new library, one capability at a time; the single `go get` line and the table of releases for `v0.1.0` and `v0.2.0` grow into tags per module (`go/vX.Y.Z`, `go/pkg/<functionality>/<adapter>/vX.Y.Z`) and one Python tag (`python-vX.Y.Z`); the one example of the old library (`examples/aws_http_smoke`) becomes a set of examples per capability and language.
- Adds: a note for the consumer of the old module, and the decision of the `1.0` release.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-13.1 Documentation sync (recurring)](T-13.1_documentation-sync-recurring.md) | todo |
| 2 | [T-13.3 Security checklist](T-13.3_security-checklist.md) | todo |
| 3 | [T-13.6 Note for the old consumer](T-13.6_note-for-the-old-consumer.md) | todo |
| 4 | [T-13.2 Examples](T-13.2_examples.md) | todo |
| 5 | [T-13.4 Releases `0.x`](T-13.4_releases-0x.md) | todo |
| 6 | [T-13.5 The `1.0` check](T-13.5_the-1-0-check.md) | todo |

Order: T-13.1 runs after every epic, not once. T-13.3 can start as soon as the redaction of T-03.4 exists and T-13.6 as soon as T-04.7 has replaced the old library; T-13.2 grows with each capability; T-13.4 needs T-01.6 and the maintainer's request for each tag; T-13.5 is last, after the maturity report of T-12.4. The IDs keep their numbers.

## Left out on purpose (no over-engineering)

Publishing to a package index (PyPI or the Go proxy beyond what a tag does), a documentation website, automatic release tooling that creates tags without the maintainer, a deprecation period for the old module (Q01-3), translations (English only, Q06-1), and a `1.0` before every capability works.

## Acceptance criteria (draft)

No document says something false at any point of the build; every capability has an example in both languages; the release process is written and used for the `0.x` tags, each only on the maintainer's request; the `1.0` decision is recorded with the maturity report next to it.
