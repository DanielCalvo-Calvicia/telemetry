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

> **Update 2026-10-28 (T-02.2):** the schema now exists: `occurred_at` and `recorded_at` are required in [contracts/schemas/envelope.schema.json](../../contracts/schemas/envelope.schema.json), both UTC with exactly six decimals and the zone `Z` (an offset or fewer decimals is rejected), and `resource` has the optional `time_zone`, a name that is only for display. The schema documents that only the library sets `recorded_at`; the code enforces it in T-03.3.

> **Update 2026-10-29 (T-03.1):** the instant is now a value object, `UTCInstant` (Go) and `UtcInstant` (Python): UTC only, years 0001 to 9999, microsecond precision (nanoseconds are cut, not rounded). `FormatUTCTime` and `format_utc_time` delegate to it, so the format is written once. Year 0 is no longer accepted in Go (it was, by an oversight, in T-02.5); Python never could hold it. `resource.time_zone` is a plain name checked by pattern and still never used to compute a time.

> **Update 2026-10-28 (T-02.5):** the fixed time format is implemented in both languages: `FormatUTCTime` (Go) and `format_utc_time` (Python) write `YYYY-MM-DDTHH:MM:SS.ffffffZ`, UTC only, exactly six decimals, truncating (not rounding) to microseconds; an instant that is not UTC is rejected with `event_schema_invalid`, never converted.
