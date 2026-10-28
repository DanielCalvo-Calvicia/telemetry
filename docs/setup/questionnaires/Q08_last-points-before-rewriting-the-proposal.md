# Q08 — The last points before PR-001 is rewritten

**Status: answered by the maintainer on 2026-10-27, except questions 1 and 10; the conflicts between the answers are in [Q09](Q09_adapters-modules-and-v1-meaning.md).** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). With the answers to [Q01](Q01_scope-repository-layout-and-rollout.md) to [Q07](Q07_open-points-after-the-answers.md) the big decisions are taken. These ten questions are what is still missing to rewrite PR-001 as version 0.2 without guessing. Question 1 comes from an answer of Q07 that **changes the envelope**, so it is the one to read first.

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

Note for Q07-8: the Python standard that exists but is not reachable now is welcome later. Until then the Python rules are written from PEP 8 and from the Go layers, and they are reviewed against the standard when you can share it.

---

## A. Time

1. **How are the two times stored in every event?** *(Q07-13, clarified by the maintainer on 2026-10-27.)* The two times are **not** UTC and local. They are two different moments: (1) when the log or the data was **generated**, and (2) when it was **added** (recorded). Keeping only the generation time loses when the system actually received it. Keeping only the recording time loses the real order of what happened, even when ids give an order. And a local time zone would make logs hard to compare on a screen. So both times are in UTC, in the same fixed format of REQ-CNF-003 (`YYYY-MM-DDTHH:MM:SS.ffffffZ`), and both are mandatory in every event of every capability.
   Today the envelope has one `timestamp`, which means "when it happened" (REQ-CTR-001, REQ-CTR-008). The rewrite adds the second one.
   Options: (a) two fields: `occurred_at` (when the data was generated at the source; the consumer may give it, otherwise it is "now" at the call) and `recorded_at` (when the library accepted the event; only the library sets it, from its clock port, so the consumer cannot change it). `timestamp` is renamed `occurred_at`. For `device.measurement.recorded`, `occurred_at` equals `data.observed_at`. The library adds the attributes `clock_source` and `clock_synced` when it knows them, because on a device without a real-time clock both times can be wrong until it syncs (my default); (b) the same, and `resource` also gets an optional `time_zone` (IANA name) that is only for display; both times stay UTC; (c) keep the name `timestamp` for the generation time and add only `recorded_at`.
   What the library cannot give: the time the **receiver** stored the event. That is the receiver's own `received_at` and lives outside the envelope (the receiver is a separate project, Q07-9). The three times together tell the whole path: generated, recorded by the library, received by the backend.
   *Default: (a).*
   A:Both times are needed (generated and recorded), as explained above. The choice between (a), (b) and (c) is still to be confirmed.
   > **Update 2026-10-28:** Settled by Q09-1: option (b), with `occurred_at`, `recorded_at` and an optional `time_zone` for display.

---

## B. Names, modules and drivers

2. **What are the public names?** *(Q01-3: `pkg/logging` disappears.)* The old surface was `pkg/logging`. The new one needs a name in each language.
   Options: (a) Go: package `telemetry` under `go/pkg/telemetry`, imported as `github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry`; Python: the import name is `telemetry` (`import telemetry`) and the install name is `telemetry` too, from git (Q07-3) (my default: one name everywhere); (b) another name (write it).
   *Default: (a).*
   A:i dont like that i would like that the import from go is more like 'github.com/DanielCalvo-Calvicia/telemetry/go/pkg/<functionality/adapter>/<subfunctionality/adapter>'. This way it only import what it needs.

3. **Which Go adapters get their own `go.mod`?** *(REQ-DST-001.)* A separate module keeps heavy dependencies away from projects that do not use them, but every module has its own tag (`go/spool/sqlite/v0.1.0`) and its own `go.mod`, which adds work at every release.
   Options: (a) the core (envelope, console, file, HTTP, the standard library only) is one module; only the adapters with a heavy dependency are separate modules: the SQLite spool and storage, OpenTelemetry, Prometheus and Langfuse (my default); (b) everything in one module and the dependencies are accepted (this breaks REQ-DST-001); (c) every adapter is its own module.
   *Default: (a).*
   A:C, i need clear separation between adapters, so the client just selects the one it needs

4. **Which SQLite driver in Go?** *(Q03-2, Q07-12.)* Python has `sqlite3` built in. Go has two kinds of driver: one needs a C compiler (cgo), which complicates cross-compiling to a Raspberry Pi and building small images; the other is pure Go, with no C needed, but a little slower.
   Options: (a) pure Go, no cgo (my default); (b) the cgo driver; (c) the one you already use in another project (write which).
   *Default: (a).*
   A:B, in most cases i need speed so it must be very very fast.
   > **Update 2026-10-28:** Superseded by Q09-4: pure Go, no cgo.

5. **What does the SQLite destination write, and who may read it?** *(Q07-12, clarified by the maintainer on 2026-10-27.)* The "storage" SQLite is **a destination**, one more way to send the events, like the AWS HTTP sink but writing to a SQLite file instead of calling a service. It is the SQL adapter of 7.2, with SQLite as its first dialect (D-03). It is different from the spool: the spool is the internal queue that holds events until **any** destination confirms them, and only the library reads it. The SQLite destination is a place where events end.
   Because it is a destination, it is configured like the others: it can be one of several active destinations at once (REQ-OUT-003), it gets the same masking and the same limits, a failure in it never stops the other destinations (REQ-DLV-005), and a project that does not use it does not install the SQLite code (an optional module in Go, an extra in Python). Writing to a local file is already durable, so it does not need the spool in front of it.
   What is left to decide is the shape of what it writes:
   Options: (a) one `events` table: the whole envelope as JSON plus indexed columns (`occurred_at`, `recorded_at`, `event_type`, severity, actor id, `event_id`); the layout is part of `contracts/`, identical in Go and Python and versioned, so any tool can read the file; a retention setting deletes old rows (default: keep until the project cleans it, as you said for the spool) (my default); (b) one table per event family (`log`, `device`, `ai`, ...) with typed columns; easier to query, but a new event type means a new table and a contract change in both languages; (c) the layout is internal to the destination and may change in any release; nobody should read the file.
   *Default: (a).*
   A:C ,but some client may read it
   > **Update 2026-10-28:** Superseded by Q09-5: the layout is a contract in `contracts/`, and the reader only reads.

---

## C. The HTTP contract

6. **Which authentication does the HTTP destination support first?** *(REQ-OUT-005.)* The receiver does not exist yet (Q07-9), so the contract of the request is defined now.
   Options: (a) a bearer token in the `Authorization` header and a custom header with an API key; the value always comes from the consumer's configuration, never from a file in the repository (my default); (b) basic authentication too; (c) mutual TLS too (needs certificates on the devices).
   *Default: (a).*
   A:Default

7. **What does the body of a batch look like?** *(REQ-OUT-005, the new HTTP schema.)*
   Options: (a) one JSON object `{"schema_version": "0.1", "events": [ ... ]}` with the envelopes in the array, optional gzip (`Content-Encoding: gzip`) (my default); (b) one event per line (NDJSON), optional gzip; (c) one event per request (no batch; simpler, very slow for 20 readings per second).
   *Default: (a).*
   A:For now given that i dont have any, lets make the Default

---

## D. Backends and limits

8. **Which metrics and tracing backends do you run today?** *(Q02-5, Q07-16: metrics and traces come fifth in the order.)* If you run no OpenTelemetry collector or Prometheus, the metrics and traces adapters have no user yet.
   Options: (a) none today; the contract and the in-library API are built in their turn, and the OpenTelemetry and Prometheus adapters only when a project has a backend (my default); (b) OpenTelemetry (write what receives it); (c) Prometheus (write what scrapes it); (d) both.
   *Default: (a).*
   A:Default

9. **Size limits: do these numbers work?** *(REQ-SEC-004, REQ-CTR-010.)* They are defaults, every one configurable.
   Options: (a) up to 32 attributes per event, key up to 64 characters, value up to 256 characters, one event up to 64 KiB, one HTTP batch up to 1 MiB (my default); (b) write other numbers.
   *Default: (a).*
   A:Default

---

## E. Next step

10. **What do you want written once you answer?**
    Options: (a) PR-001 version 0.2 with every decision of Q01–Q08 marked as decided, and a new ADR that records the adoption of the standard (my default: the proposal and the ADR must say the same thing, as `CLAUDE.md` asks); (b) the same plus a task plan in `docs/tasks/` with the order of question 16 of Q07; (c) only PR-001 version 0.2.
    *Default: (a). Annex A of PR-001 says the task plan is handled separately, so it waits until you ask for it.*
    A:Before anything, do you thing there is any open decisions to be made, or confusing thing or anything it might get wrong?
    > **Update 2026-10-28:** Superseded by Q09-8. The default (a) was applied: PR-001 version 0.2 and ADR-011 to ADR-013.
   
