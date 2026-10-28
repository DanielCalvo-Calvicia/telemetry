# ADR-012 — Two times in every event: when it was generated and when it was recorded

**Status:** Accepted (2026-10-28, with [PR-001](../proposals/PR-001-telemetry-standard-go-python.md) version 0.2; decided by the maintainer in Q08-1 and Q09-1). Nothing is built.
**Related:** [PR-001](../proposals/PR-001-telemetry-standard-go-python.md) (5.1, 5.4, REQ-CTR-008), [ADR-011](ADR-011-adopt-the-telemetry-standard-go-python.md)

## Context

An event has one moment when the data was generated at the source and another when it reached the system. With only the generation time, the system loses when it received the data. With only the recording time, the real order of what happened is lost, even when the identifiers give an order. A local time zone would make logs hard to compare on a screen. A device without a real-time clock can have both times wrong until it syncs.

## Decision

Every event of every capability has two mandatory instants, both in UTC and in the fixed format `YYYY-MM-DDTHH:MM:SS.ffffffZ`:

- `occurred_at`: when the data was generated at the source. The consumer may give it; otherwise it is the time of the call. When the data carries `observed_at`, `occurred_at` equals it. It replaces the old `timestamp`.
- `recorded_at`: when the library accepted the event. Only the library sets it, from its clock port.

`resource` may carry an optional `time_zone` (IANA name), for display only; it is never used to compute a time. The library adds the attributes `clock_source` and `clock_synced` when it knows them. The time the receiver stores the event (`received_at`) belongs to the receiver and is not in the envelope.

## Consequences

- Positive: the real order of events and the delay between generation and recording are both visible; the logs of every language compare on one scale.
- Negative: every event is a little bigger; every example and schema carries two times; both instants come from the same device clock, so they are both wrong when the clock is.
