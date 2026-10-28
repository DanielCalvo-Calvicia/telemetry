# ADR-010 — No deployment; release by tags

**Status:** Accepted (2026-10-19, the build guide of the library); first releases 2026-10-19 (v0.1.0) and 2026-10-20 (v0.2.0, see [docs/deployment/deployment.md](../deployment/deployment.md))
**Related:** [ADR-002](ADR-002-one-module-capabilities-side-by-side.md)

## Context

The library is public. It is used by importing it; nothing of it runs on its own.

## Decision

The repository has no `cmd/`, Dockerfile, Kubernetes manifest, docker-compose or goreleaser file, and its only workflow is `ci.yml` (vet, tests, coverage gate; read-only token; actions pinned by commit SHA). A release is a git tag on `main` (`v0.x.y`). Work goes through a `feature_*` branch, then `feature`, then `main`; the tag is created on `main`. A tag is never moved or reused: a bad release gets a `retract` line in `go.mod` and a new patch version. Consumers pin a tag and do not set `GOPRIVATE` for this module.

## Consequences

- Positive: nothing to operate; the Go proxy and the checksum database protect the consumers.
- Negative: everything pushed publicly is permanent (the proxy caches versions forever), so the audit of the public checklist (no secrets, no internal names or addresses, fake values only) must pass before a push.

> **Update 2026-10-28 (T-01.6):** the new layout has more things that are tagged: `go/vX.Y.Z` for the Go core module, `go/pkg/<functionality>/<adapter>/vX.Y.Z` for each Go adapter module and `python-vX.Y.Z` for the Python package; the old library keeps `vX.Y.Z`. The core is released first, nothing is tagged unless the maintainer asks and a tag is still permanent. The rule is in the [deployment document](../deployment/deployment.md). No tag of the new layout exists yet.
