# ADR-001 — Adopt the shared Go Clean/Hexagonal standard

**Status:** Accepted (2026-10-19, the build guide of the library)
**Related:** [ADR-002](ADR-002-one-module-capabilities-side-by-side.md), [ADR-003](ADR-003-public-surface-is-pkg-logging.md)

## Context

Snow Factory keeps a canonical Go architecture (layers, value objects, entities, ports, services, adapters, composition root, error handling, tests) in Engram project `shared` (`standards/go/*`). The logging code of this library was written and tested under that standard in a service of the group before it became a library.

## Decision

The library follows the standard **inside the capability**: `domain`, `application` and `infrastructure`, with the ports in `application/ports`, one outbound adapter per folder (with `external/`, `error/` and `helpers/`), one dependencies file per adapter and the container in the composition root. Tests mirror the code tree under `internal/tests` and are black-box.

## Consequences

- Positive: the code reads like every other Snow Factory Go service; the transplant from the source service kept its structure and its tests.
- Negative: more folders than a small library would need on its own; the shared standard must be consulted before changing a layer.
