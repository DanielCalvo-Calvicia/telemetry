# ADR-002 — One module, capabilities side by side

**Status:** Accepted (2026-10-19, the build guide of the library)
**Related:** [ADR-001](ADR-001-adopt-shared-go-standard.md), [ADR-003](ADR-003-public-surface-is-pkg-logging.md)

## Context

The library is named `telemetry` because it is meant to hold logging and, later, metrics. The options were one module per capability or one module with the capabilities side by side.

## Decision

One Go module. Each capability lives under `internal/<capability>/{domain,application,infrastructure}` (`internal/logging` today, `internal/metrics` **planned**), with its own public package under `pkg/<capability>`. The composition root (`internal/composition_root`) has one container per capability.

## Consequences

- Positive: one version, one tag and one CI for the whole library; a consumer that only logs does not compile metrics code (the packages are separate).
- Negative: a breaking change in one capability moves the version of both. The library stays on `v0.x` until the API settles; `v2` and later would need a `/v2` module path.

> **Update 2026-10-28:** [PR-001](../proposals/PR-001-telemetry-standard-go-python.md) proposes one core module plus one module per adapter, under `go/`. [ADR-013](ADR-013-go-modules-per-adapter-chosen-in-code.md) would replace this decision when it is built. Nothing changes today.

> **Update 2026-11-03 (T-04.7):** replaced by [ADR-013](ADR-013-go-modules-per-adapter-chosen-in-code.md). The old module, with its `internal/logging` capability and its `pkg/logging` surface, was removed from the tree in T-04.7; the library is now the core module `go/` plus one module per adapter that has its own dependencies (the first is `go/pkg/logging/aws_http`). The tags `v0.1.0` and `v0.2.0` keep the old module on the Go proxy.
