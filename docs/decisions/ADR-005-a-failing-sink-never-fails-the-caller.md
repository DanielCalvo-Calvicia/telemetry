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

> **Update 2026-11-03 (T-04.7):** the rule stays and is implemented by the destinations and the pipeline of the new core: a destination that fails never fails the caller and never stops another destination, and a log call returns nothing. What goes with the removed library is its mechanism: the report of a failing sink to a console fallback, the synchronous send and the "never retries" text. The queue, the retry policy and the diagnostics of T-05 replace them; until then a destination sends once, synchronously, and the failure is in the result of the event.

> **Update 2026-11-03 (T-05.1):** the rule stays and is now stronger: a log call only enqueues and a worker per destination sends, so a slow or failing destination cannot delay the caller or another destination at all. The synchronous send and the "never retries" text are gone: the call no longer waits up to 3 s per sink. The retry by delivery mode is T-05.3. What a send answered is no longer in the result of the call (*accepted* means queued); it is counted per destination (T-05.5).

> **Update 2026-11-03 (T-05.3):** the retry the first note waited for exists: with the `retry` mode a worker sends a batch again after a transient failure (exponential backoff with jitter, `Retry-After`, at most 5 attempts by default), never delaying the caller or another destination; `best_effort` still sends once. See [architecture section 2.28](../architecture/architecture.md).