# Q03 — Delivery, the HTTP receiver and the spool

**Status: answered by the maintainer on 2026-10-27; the points left open are in [Q07](Q07_open-points-after-the-answers.md).** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). This questionnaire settles D-01 and the delivery defaults of section 10 of the specification.

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

---

1. **Who receives the HTTP events?** The library is not a backend (1.3) but the default destination is HTTP (REQ-OUT-002). Today the receivers are a generic HTTP endpoint and the `add_log` endpoint of aws_microservice.
   Options: (a) the existing receivers stay, the new HTTP schema is defined by the contract, and a receiver for it is a separate project, out of this proposal (my default); (b) the proposal includes a reference receiver for tests; (c) the new schema is also sent to aws_microservice, which is changed to accept it.
   *Default: (a).*
   A:Default

2. **D-01: spool format.**
   Options: (a) SQLite (one file, transactions, both languages can read it; it adds a dependency to the durable mode only) (my default); (b) segmented files (no dependency, but more code to get right: corruption, compaction, locking).
   *Default: (a).*
   A:Default

3. **Which delivery mode is the default of each profile?** The modes are `best_effort`, `retry` and `durable`.
   Options: (a) `best_effort` for `microservice`, `retry` for `worker` and `cronjob`, `durable` for `robotics` and `offline_device` (my default); (b) `retry` everywhere and `durable` only when asked; (c) write another table.
   *Default: (a).*
   A:Default

4. **Queue, batch and flush defaults.** Today there is no queue: a call sends at once, with a 3 s timeout and no retry.
   Options: (a) a queue of 10,000 events, a batch of 100, a flush every 2 s and a timeout of 3 s (my default; they are only defaults and every one is configurable); (b) write other numbers; (c) decide with measurements in the pilot.
   *Default: (a).*
   A:Default

5. **Spool quota and where it is stored.**
   Options: (a) 256 MiB per destination, under a directory the consumer gives (the library never picks a path by itself) (my default); (b) write another quota; (c) no default: the consumer must set it to use `durable`.
   *Default: (a).*
   A:Default

6. **Does today's synchronous send stay as a mode?** Today a call sends at once and the error of the sink is only logged.
   Options: (a) it becomes `best_effort` with a queue and the synchronous send is dropped (my default, because REQ-DLV-002 forbids blocking the caller); (b) keep a synchronous mode for short programs, such as a cronjob that exits right after logging.
   *Default: (a), and `shutdown` flushes before the program exits.*
   A:Default

7. **Deduplication.** The library gives at-least-once and the receiver deduplicates by `event_id` (REQ-DLV-001). Is that agreed for every receiver you have?
   Options: (a) yes (my default); (b) the library should also try to avoid duplicates (it cannot guarantee it); (c) write which receivers cannot deduplicate.
   *Default: (a).*
   A:Default
