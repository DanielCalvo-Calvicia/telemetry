# ADR-007 — The minimum level follows the environment

**Status:** Accepted (carried over from the source service, Q18-14, Q19-2)
**Related:** [ADR-004](ADR-004-library-never-reads-the-environment.md)

## Context

A level variable per service is one more setting to forget, and a production service logging DEBUG is noisy and costly.

## Decision

There is no log-level variable. `app_env` (an argument of `New` and `NewWithSinks`) selects the minimum level: `development` lets DEBUG and above through, `production` lets INFO and above through. TRACE is never sent. An `app_env` with no level is an error at construction.

## Consequences

- Positive: production is quiet by default and development is verbose by default, with nothing to configure.
- Negative: a consumer cannot debug production by raising the level; it has to deploy with `development`. A third environment (for example `staging`) needs a change of `MinimumLevelByEnvironment` in `domain/rules.go`.
