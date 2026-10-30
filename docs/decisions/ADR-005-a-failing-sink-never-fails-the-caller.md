# ADR-005 — A failing sink never fails the caller

**Status:** Accepted (carried over from the source service, Q18-15)
**Related:** [ADR-006](ADR-006-masking-before-every-sink.md)

## Context

A log call sits in the middle of business code. A receiver that is down or slow must not break a request or delay it for long.

## Decision

`LoggerService` sends the entry to every sink; when a sink fails, the entry is **dropped for that sink**, the other sinks still get it, and an ERROR entry is sent to the **fallback sink** (the console). The report keeps the identifiers of the dropped entry so both can be correlated, and never repeats its message or fields (they may hold sensitive data). A failure of the fallback itself is ignored: there is nowhere left to report it. The HTTP sinks time out after 3 s and never retry. The only error a log call returns is a failure to build the entry.

> **Update 2026-10-30 (T-03.7):** in the new core the rule covers every step, not only the sinks. The services return their failures as values with a code of the catalog (`event_build_failed`, `masking_failed`, `limits_failed`, `limit_exceeded`, ...) and a recovered panic becomes such a value without the panic value in it. Errors are raised only at construction and at start.

## Consequences

- Positive: a broken receiver costs a request at most 3 s per sink and never an error.
- Negative: entries are lost when a receiver is down (there is no queue, no retry and no disk buffer). Delivery is best effort; a service that needs guaranteed delivery needs a different design.
