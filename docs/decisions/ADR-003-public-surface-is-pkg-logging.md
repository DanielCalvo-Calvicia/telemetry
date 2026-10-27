# ADR-003 — The public surface is `pkg/logging`, by aliases

**Status:** Accepted (2026-10-19, the build guide of the library); extended 2026-10-20 (the sinks, [ADR-009](ADR-009-ready-made-sink-constructors.md))
**Related:** [ADR-002](ADR-002-one-module-capabilities-side-by-side.md)

## Context

Everything of the library is under `internal/`, so a consumer cannot import it. The consumer needs a small, stable API: a logger, an entry, a config and a way to build them.

## Decision

`pkg/logging` is the only public package. Its types are **aliases** (`type Logger = inbound.LoggerPort`, `type Entry = dtos.LogEntryInboundDTO`, `type Config = config.TelemetryConfig`, `type Sink = outbound.LogSinkPort`) so each type has one definition and the consumer can use every field. Functions are thin: they call the container or a dependencies function. Only what a consumer was seen to use is exported; every exported identifier has a doc comment that starts with its own name.

## Consequences

- Positive: nothing speculative is public; the internal code can be reorganized without breaking consumers; the public API is one file to review (`pkg/logging/logging.go`, `sinks.go`).
- Negative: an alias exposes the shape of the internal type, so a change to an internal DTO is a change of the public API.
