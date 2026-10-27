# ADR-004 — The library never reads the environment

**Status:** Accepted (2026-10-19, the build guide of the library)
**Related:** [ADR-003](ADR-003-public-surface-is-pkg-logging.md), [ADR-007](ADR-007-minimum-level-follows-the-environment.md)

## Context

The shared configuration standard loads the whole environment of a process once, in the composition root. A library that read variables by itself would break that rule and would hide settings from the consumer.

## Decision

The library never calls `os.Getenv`, `envconfig.Process` or reads a `.env` file. `Config` (`infrastructure/config`) holds the fields and their env tags and does no loading. The consumer embeds `logging.Config` in its configuration struct and loads it in its single pass; `app_env` is an argument of `New`.

## Consequences

- Positive: the consumer sees and controls every setting; the library is easy to test (a `Config` value is enough).
- Negative: the consumer must embed `Config` and pass `app_env`; a consumer that forgets a variable gets the default of the tag.
