# ADR-013 — One Go module per adapter, and the project chooses its adapters in code

**Status:** Accepted (2026-10-28, with [PR-001](../proposals/PR-001-telemetry-standard-go-python.md) version 0.2; decided by the maintainer in Q08-2, Q08-3, Q09-2 and Q09-3). Nothing is built. When it is built it replaces [ADR-002](ADR-002-one-module-capabilities-side-by-side.md) and [ADR-003](ADR-003-public-surface-is-pkg-logging.md).
**Related:** [PR-001](../proposals/PR-001-telemetry-standard-go-python.md) (REQ-ARC-012, REQ-ARC-013, REQ-DST-001, REQ-DST-002), [ADR-004](ADR-004-library-never-reads-the-environment.md), [ADR-011](ADR-011-adopt-the-telemetry-standard-go-python.md)

## Context

A project must be able to select only the adapters it needs, so that the dependencies of the others (SQLite, OpenTelemetry, Prometheus, Langfuse) are never downloaded. The current library is one module with the single public package `pkg/logging`.

## Decision

- Go has one core module, `github.com/DanielCalvo-Calvicia/telemetry/go`, and one module per adapter, each with its own `go.mod` and its own tag (`go/vX.Y.Z` for the core, `go/pkg/<functionality>/<adapter>/vX.Y.Z` for an adapter). Each adapter pins a minimum core version; the core is released first. A `go.work` file is committed for local work and CI.
- The adapters of the core (console, file, HTTP) live in the core module. Every other adapter is a module of its own at `go/pkg/<functionality>/<adapter>/`, with the same inner folders as the core (`external/`, `error/`, `helpers/`). The wiring of the optional adapters is done in the consumer's own composition root.
- The public package of the core, with the entry point `New`, `Shutdown` and the shared types, is `go/pkg/telemetry` (T-01.1, 2026-10-28). Until T-04.7 removes the old library its root `go.mod` stays beside `go/go.mod` and `go.work` joins them.
- The public packages are `go/pkg/<functionality>/<adapter>` in Go and `telemetry.<functionality>.<adapter>` in Python (installed from the Git repository with extras).
- The project chooses its adapters by importing them and passing them to the core in code. The configuration carries only each adapter's settings, never an adapter name, and an adapter never registers itself on import. This keeps no hidden global state and fits separate modules, because the core cannot import an adapter.
- The SQLite adapters use a pure-Go driver (no cgo) in Go and the standard `sqlite3` in Python.

## Consequences

- Positive: a project downloads and builds only what it imports; the choice of adapters is visible in its own code.
- Negative: many `go.mod` files and tags, slower releases, and an adapter cannot be switched on by an environment variable alone. The rule that `internal/` packages of the core are importable from a nested module (the Go rule is by path) is to be verified when this is built.

> **Update 2026-10-28 (T-01.2):** the rule that was to be verified holds. A throwaway module with the path `github.com/DanielCalvo-Calvicia/telemetry/go/pkg/throwaway`, joined to the core by a `go.work` file, imported `github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain` and built; a module with an unrelated path (`example.invalid/outsider`) importing the same package failed with "use of internal package ... not allowed". The Go rule is by import path, so an adapter module at `go/pkg/<functionality>/<adapter>/` may import the `internal/` packages of the core. Go 1.25.5.

> **Update 2026-10-28 (T-01.6):** the tag shapes and the release order (the core first, then the adapters, then Python) are written in the [deployment document](../deployment/deployment.md); a released adapter must build without `go.work`.
