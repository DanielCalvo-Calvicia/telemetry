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
