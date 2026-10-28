# T-05 — Delivery, buffering and failures

| | |
|---|---|
| Status | todo |
| Kind | epic |
| Parent | project |
| Depends on | T-04 |
| Blocks | T-06 to T-13 |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 (sections 10 and 12) and the answers of [Q03](../../setup/questionnaires/Q03_delivery-receiver-and-spool.md) (Q03-3 modes per profile, Q03-4 queue, batch and flush defaults, Q03-6 no synchronous send, Q03-7 deduplication by the receiver), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (Q07-7 the Python API is synchronous with its own worker thread) and [Q08](../../setup/questionnaires/Q08_last-points-before-rewriting-the-proposal.md) (Q08-9 limits). Related decisions: [ADR-005](../../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md) and [ADR-011](../../decisions/ADR-011-adopt-the-telemetry-standard-go-python.md).

## Goal

Sections 10 and 12 of the specification: the bounded queue, the worker, the delivery modes `best_effort` and `retry`, the failure matrix, `flush` and `shutdown`, and the diagnostics. The spool (`durable` mode) is T-06.

## What it keeps and what it changes

- Keeps: the rule that a failing destination never fails the caller and never stops the others (ADR-005); the 3 s network timeout.
- Changes: today a log call sends to each sink at once and the call waits up to 3 s per sink; there is no queue, no retry and no buffer (ADR-005, "Negative"). The synchronous send goes away: a call enqueues and returns, and a worker sends in batches (REQ-DLV-002). ADR-005 gets a dated note for this.
- New: three delivery modes (two here, `durable` in T-06.5), drop policies and priorities, `flush` and `shutdown` with a result, and per-destination diagnostics.

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-05.1 Bounded queue and worker](T-05.1_bounded-queue-and-worker.md) | todo |
| 2 | [T-05.2 Backpressure and priorities](T-05.2_backpressure-and-priorities.md) | todo |
| 3 | [T-05.3 Delivery modes `best_effort` and `retry`](T-05.3_delivery-modes-best-effort-and-retry.md) | todo |
| 4 | [T-05.4 `flush` and `shutdown`](T-05.4_flush-and-shutdown.md) | todo |
| 5 | [T-05.5 Diagnostics](T-05.5_diagnostics.md) | todo |
| 6 | [T-05.6 Failure-matrix tests](T-05.6_failure-matrix-tests.md) | todo |

Order: T-05.1 first. Then T-05.2, T-05.3, T-05.4 and T-05.5 are independent of each other and can run in parallel, each on the queue and worker of T-05.1 (T-05.2 and T-05.5 touch the same counters, so they agree on the drop reasons first). T-05.6 goes last because it exercises the modes (T-05.3) and the counters (T-05.5). T-06 can start when T-05.3 is done.

## Left out on purpose (no over-engineering)

Exactly-once delivery (REQ-DLV-001); a synchronous send mode (Q03-6); a circuit breaker; a dead-letter store for rejected batches; adaptive batch sizes; any priority that implies a delivery guarantee (REQ-DLV-009); the spool (T-06).

## Acceptance criteria (draft)

A slow or failing HTTP destination never blocks the caller or the other destinations, every drop is counted by destination and reason, `flush` and `shutdown` report what was sent and dropped within their timeout, and the same fake-receiver scenarios give the same counters in Go and in Python.
