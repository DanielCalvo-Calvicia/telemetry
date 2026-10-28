# Q09 — How adapters are chosen, the Go modules, the SQLite file and what "v1.0" means

**Status: answered by the maintainer on 2026-10-28 and recorded in [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) version 0.2 and in ADR-011 to ADR-013.** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). Some answers of [Q08](Q08_last-points-before-rewriting-the-proposal.md) conflict with each other or with the proposal. These eight questions settle them, so PR-001 can be rewritten as version 0.2 without guessing.

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

---

## A. Time

1. **Which option for the two times?** *(Q08-1, still to be confirmed.)* The two times are the generation time and the recording time, both in UTC.
   Options: (a) `occurred_at` (when the data was generated at the source) and `recorded_at` (when the library accepted it, set only by the library); `timestamp` is renamed `occurred_at`; the library adds `clock_source` and `clock_synced` when it knows them (my default); (b) the same, plus an optional `time_zone` in `resource` for display only; (c) keep the name `timestamp` for the generation time and add only `recorded_at`.
   *Default: (a).*
   A:B

---

## B. Adapters and Go modules

2. **How does a project choose its adapters?** *(Q08-2: one import path per adapter; Q08-3: every adapter is its own module.)* If an adapter is its own module, the core cannot import it. So the choice cannot be a string in the configuration (`Kind: "sqlite"`): someone has to import the adapter and give it to the core.
   Options: (a) the project imports the adapters it needs and passes them to the core in code, for example `telemetry.New(cfg, sqlite.NewDestination(...), http.NewDestination(...))`; the configuration only carries the settings of each adapter (my default: it is explicit, it has no hidden global state (REQ-ARC-006) and it is what separate modules need); (b) every adapter registers itself when it is imported (`import _ ".../sqlite"`) and the configuration chooses by name (shorter for the project, but it is global state and an import that is easy to forget); (c) both.
   Python follows the same rule so both languages stay identical: the project imports `telemetry.adapters.sqlite` and passes it in.
   *Default: (a).*
   A:Default

3. **How are the Go modules laid out and released?** *(Q08-3 answer C.)* Every adapter has its own `go.mod`, so every adapter has its own tag (for example `go/pkg/spool/sqlite/v0.1.0`) and pins a minimum version of the core. The core is released first. For local work and CI the repository commits a `go.work` file so all modules are tested together.
   Options: (a) strictly one module per adapter, as you answered (my default); (b) one module per dependency: a single `sqlite` module holds the SQLite spool and the SQLite destination, a single `otel` module holds its logs, metrics and traces (fewer modules and tags, same separation for the project, because it imports only the dependency it needs).
   *Default: (a).*
   A:Default

4. **The SQLite driver for Go.** *(Q08-4: the cgo driver, because speed matters.)* cgo needs a C compiler to build, makes cross-compiling to a Raspberry Pi harder, and does not work with `CGO_ENABLED=0` builds (for example minimal container images). Because the SQLite adapter is its own module, only a project that imports it pays that cost. For batched inserts the limit is usually disk sync and the SQLite settings, not the driver, so the difference may be small.
   Options: (a) the cgo driver, and a small benchmark of cgo against a pure-Go driver before the first release; the result goes in an ADR, and the choice changes only if the speed difference is small (my default); (b) the cgo driver, no benchmark; (c) pure Go.
   Also answer: which Go projects will import the SQLite adapters, and do they build for ARM or in minimal images?
   *Default: (a).*
   A:C

5. **How is the SQLite destination file shared with a reader?** *(Q08-5: "internal, but some client may read it"; clarified by the maintainer on 2026-10-27: the client is a UI or another process, not the project that uses the telemetry library.)* So the file has two sides: the project that uses the library **writes** it, and a different program (a UI, a report tool, another service on the same machine) **reads** it. That reader was written by someone who does not see the library code. For the reader the layout is a contract: if it changes, the reader breaks. It also means the layout cannot be internal (option c of Q08-5), and the Go and Python adapters cannot write different layouts, or every reader would have to understand both.
   Reading a file that another process is writing needs a few rules, and I propose them with the layout:
   - the file is opened in **WAL mode**, so a reader can read while the writer writes, without blocking it;
   - the reader opens the file **read-only** and never deletes or changes rows; retention (deleting old rows) is done only by the writer side;
   - the layout version is stored **inside the file** (`PRAGMA user_version`), so a reader can check it before it reads;
   - the reader and the writer must be on the **same machine**: WAL does not work over a network share;
   - the table has indexes on `occurred_at`, `recorded_at`, `event_type` and actor id, so a UI can filter and sort without scanning everything.
   Options: (a) the layout is part of `contracts/`: one `events` table with the whole envelope as JSON plus those indexed columns, identical in Go and Python, versioned, with the rules above (my default: it is the only option that fits a reader that is a separate program); (b) each language has its own documented layout and the reader handles both (the UI would have to know two layouts for the same events); (c) internal, with no reader allowed (this contradicts your clarification).
   Also answer: does the reader need to **write** anything (for example "this event was seen" or "delete these rows")? If yes, the writer and the reader would both write the file, and that needs more rules.
   *Default: (a), and the reader never writes.*
   A:The reader never writes.Default

---

## C. What "v1" means

6. **When does the library reach `1.0`?** *(Q02-5: all capabilities in v1, one by one; REQ-CNF-008.)* The proposal says the overall maturity is that of the least complete capability. With logs, metrics, traces, executions, health, devices, AI and audit in scope, `1.0` would wait for all of them, and the first projects would use `0.x` for a long time.
   Options: (a) every capability has its own level (`stable` or `beta`); the library reaches `1.0` when the core is stable (envelope, logs, executions, faults, delivery and one destination per delivery mode); the rest can be `beta` inside `1.x`, and REQ-CNF-008 is changed (my default); (b) `1.0` only when all capabilities are stable, as the proposal says; (c) stay in `0.x` until you decide.
   *Default: (a).*
   A:B ,if not all is working there is no v1, its just a beta version. A full v1 is when all processes works.

---

## D. Small corrections and the next step

7. **Do you accept these three corrections to PR-001?**
   - The README and the proposal say which module path is the old one (`github.com/DanielCalvo-Calvicia/telemetry`, tags `v0.1.0` and `v0.2.0`) and which is the new one, so they are not mixed up.
   - A section "Known risks" says that, with no receiver and no Go pilot, the HTTP contract and the Go side are tested only against fakes and the cross tests.
   - A rule says that for a measurement `occurred_at` equals `observed_at`, so the two can never differ.
   Options: (a) yes (my default); (b) change something (write what).
   *Default: (a).*
   A:Default

8. **What is written once you answer?** *(Q08-10, which you answered with a question.)*
   Options: (a) PR-001 version 0.2 with every decision of Q01–Q09 marked as decided, and these ADRs: the adoption of the standard, the two times of every event, and the modules and the choice of adapters (which replace ADR-002 and ADR-003 by a dated note) (my default); (b) the same plus a task plan in `docs/tasks/` with the build order of Q07-16; (c) only PR-001 version 0.2.
   *Default: (a). Annex A of PR-001 says the task plan is handled separately, so it waits until you ask for it.*
   A:Before anything, do you thing there is any open decisions to be made, or confusing thing or anything it might get wrong?
