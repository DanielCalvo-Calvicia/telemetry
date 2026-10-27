# ADR-009 — Ready-made constructors for each sink

**Status:** Accepted (2026-10-20, requested by the maintainer; released in v0.2.0)
**Related:** [ADR-003](ADR-003-public-surface-is-pkg-logging.md), [ADR-008](ADR-008-aws-http-sink-adapts-to-add-log.md)

## Context

`logging.New` builds the sinks a `Config` switches on. A consumer that wants to choose its sinks in code, or to add a sink of its own, had no public way to build one.

## Decision

`pkg/logging` exports a constructor per outbound adapter (`NewConsoleSink`, `NewHTTPSink`, `NewAWSHTTPSink`), the `Sink` interface (an alias of the outbound port) with `SinkEntry`, and `NewWithSinks(ctx, app_env, mask_extra_patterns, sinks...)` to build the logger over the sinks the caller built. The constructors call the dependencies of the composition root, so the public package still imports no concrete adapter. A constructor that fails returns a nil `Sink` interface, never a typed nil. `logging.New` keeps working unchanged.

## Consequences

- Positive: a client has an easy, typed way to generate each adapter, and may implement its own `Sink`.
- Negative: `Sink` and `SinkEntry` are now public, so the shape of the outbound port is part of the API.
