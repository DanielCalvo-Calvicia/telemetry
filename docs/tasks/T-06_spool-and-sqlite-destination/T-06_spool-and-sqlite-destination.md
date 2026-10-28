# T-06 — The spool (`durable` mode) and the SQLite destination

| | |
|---|---|
| Status | todo |
| Kind | epic |
| Parent | project |
| Depends on | T-05 |
| Blocks | T-07 to T-13 |

> Created on 2026-10-28 from [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 and the answers of [Q03](../../setup/questionnaires/Q03_delivery-receiver-and-spool.md) (Q03-2 the spool is a SQLite file, Q03-5 quota and directory), [Q04](../../setup/questionnaires/Q04_devices-and-hardware.md) (Q04-5 the spool stays until the project cleans it, Q04-6 SQL starts with SQLite), [Q07](../../setup/questionnaires/Q07_open-points-after-the-answers.md) (Q07-11 the quota, Q07-12 two different SQLite uses), [Q08](../../setup/questionnaires/Q08_last-points-before-rewriting-the-proposal.md) (Q08-5 the destination file may be read by a client) and [Q09](../../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (Q09-3 one module per adapter, Q09-4 a pure-Go driver, Q09-5 the layout is part of `contracts/` and the reader never writes). Q09 is the latest answer where two disagree: Q08-4 asked for the cgo driver and Q09-4 chose pure Go.

## Goal

The two uses of SQLite decided in Q07-12, Q08-5 and Q09-5: the internal spool that makes `durable` delivery at-least-once, and the SQLite destination, a place where events end and that a reader (a UI or another process) can read. Each is an optional module in Go (a pure-Go driver, D-01, Q09-4) and an extra in Python (the standard `sqlite3`).

## What it keeps and what it changes

- Keeps: nothing of SQLite exists in the repository today. The library has no disk buffer; ADR-005 lists that as a consequence ("no queue, no retry and no disk buffer"), and T-05 adds the queue and the retry in memory.
- Adds: `contracts/storage/` with two layouts; a spool adapter in each language and the `durable` mode on top of it; a SQLite destination in each language with a layout a reader can rely on; tests that read the files of one language with the other.
- The two files are different files: the spool is internal and the project never reads it; the destination file is a contract for a reader (Q07-12, Q09-5). The destination has no spool in front of it, because writing to a local file is already durable (REQ-OUT-009).
- The SQLite modules are not part of the core: a project that does not import them never downloads the driver (ADR-013).

## Subtasks

| Order | Task | Status |
|---|---|---|
| 1 | [T-06.1 Layouts in `contracts/storage/`](T-06.1_layouts-in-contracts-storage.md) | todo |
| 2 | [T-06.2 Choose the Go driver](T-06.2_choose-the-go-driver.md) | awaiting-user |
| 3 | [T-06.3 Spool adapter in Go](T-06.3_spool-adapter-in-go.md) | todo |
| 4 | [T-06.4 Spool adapter in Python](T-06.4_spool-adapter-in-python.md) | todo |
| 5 | [T-06.6 SQLite destination in Go and Python](T-06.6_sqlite-destination-in-go-and-python.md) | todo |
| 6 | [T-06.5 `durable` mode](T-06.5_durable-mode.md) | todo |
| 7 | [T-06.7 Cross tests of the files](T-06.7_cross-tests-of-the-files.md) | todo |

Order: T-06.1 and T-06.2 only need T-02.1, so they can start before the rest of T-05 is done; T-06.2 waits for the maintainer. T-06.3 needs both. T-06.4 needs only T-06.1 and runs in parallel with T-06.3. T-06.6 needs the layout and T-05.3, and runs in parallel with the spool adapters (its Go side also needs the driver of T-06.2). T-06.5 needs both spool adapters and T-05.3. T-06.7 comes last.

## Left out on purpose (no over-engineering)

Another SQL dialect (D-03: added when a project needs it); a reader that writes to the destination file (Q09-5: the reader never writes); sharing a file over a network (WAL needs the same machine); encryption at rest and compression; automatic cleanup by the library (the project cleans, Q04-5); a spool in front of the SQLite destination; schema migrations beyond a version check.

## Acceptance criteria (draft)

A device that loses its network keeps its events in the spool and sends them when it is back, after a restart too; a spool written by Go is read by Python and the other way round; a UI can open the SQLite destination file read-only while the writer writes, without disturbing it; the core still builds without the driver.
