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

> **Update 2026-10-30 (T-03.6):** still true in the new core and now tested: a test in each language reads the sources and fails on any mention of the environment. The consumer loads the values with its own tool using the `TELEMETRY_*` names of [the schema](../../contracts/config/config.schema.json); the env tags carry **no `default=`**, because a default in a tag would override the profile when the environment is loaded after the profile is chosen (the defaults live in the code, in one place). `app_env` of the old library is not a field of the new model.

> **Update 2026-10-28:** confirmed for the new standard (Q06-3 of [PR-001](../proposals/PR-001-telemetry-standard-go-python.md)): the library reads no environment and no file; the consumer loads its configuration and passes it in. The shared model is a schema and the profiles are functions.
