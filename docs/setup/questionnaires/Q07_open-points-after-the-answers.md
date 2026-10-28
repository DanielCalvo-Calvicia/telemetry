# Q07 — Open points after the answers to Q01–Q06

**Status: answered by the maintainer on 2026-10-27; the last points are in [Q08](Q08_last-points-before-rewriting-the-proposal.md).** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). Your answers to [Q01](Q01_scope-repository-layout-and-rollout.md) to [Q06](Q06_security-configuration-and-release.md) settled most of the decisions. While I read them, some answers contradict the text of PR-001 or each other, and some left a practical question open. Each question below says which answer it comes from.

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

---

## A. Repository, module and distribution

1. **Where is `go.mod`, and what is the new import path?** *(Q01-2, Q01-3, Q06-8.)* Today `go.mod` is at the repository root and the module is `github.com/DanielCalvo-Calvicia/telemetry`, with tags `v0.1.0` and `v0.2.0`. You decided one repository with `contracts/`, Go and Python, and that `pkg/logging` disappears. The proposal places Go under `go/` and tags `go/vX.Y.Z`. A Go module in a subfolder gets a new import path (`.../telemetry/go`) and needs the `go/` tag prefix.
   Options: (a) move Go to `go/`; the module becomes `github.com/DanielCalvo-Calvicia/telemetry/go` and tags are `go/v0.3.0`, `go/v1.0.0` (my default: it keeps the layout symmetric with `python/` and with the proposal); (b) keep `go.mod` at the root, `contracts/` and `python/` beside it, and tags stay `vX.Y.Z` for Go (no change for Go consumers, but the root is shared by three things); (c) write another layout.
   *Default: (a).*
   A:Default

2. **What version does the new Go module start at?** *(Q01-3, Q01-5.)* The old API disappears and its only consumer takes it from the published package.
   Options: (a) `v0.3.0` (still `v0.x`, breaking changes allowed, my default); (b) `v1.0.0` when the first stable capability is ready; (c) `v2.0.0`, to make the break explicit (needs `/v2` in the import path).
   *Default: (a).*
   A:Default

3. **How does a Python project install the library?** *(Q01-2: "the languages will import this project".)* The repository is public.
   Options: (a) PyPI under a name you own (`pip install telemetry...`) (my default: the normal way, versioned); (b) straight from GitHub (`pip install "git+https://github.com/.../telemetry@python-v0.1.0#subdirectory=python"`), with no PyPI; (c) a private package index (write which).
   *Default: (a), and until the first stable release (b).*
   A:B, i dont own any name (i dont know even what it is)

4. **Is the generated code committed?** *(Q02-3.)* The generator is written in Go. A Python project that installs the package must not need Go.
   Options: (a) the generated code of both languages is committed, and CI fails if it differs from the contract (my default); (b) it is generated at build or install time (needs Go at install: not acceptable for Python).
   *Default: (a).*
   A:Default

---

## B. Pilots, parity and Python

5. **Which Go project is a pilot?** *(Q01-1, Q04-1, Q05-1.)* Your two known projects (the Raspberry Pi hardware and the AI agent with Langfuse) are Python. The parity rule says Go and Python are identical and tested against each other (REQ-CNF-006). Without a Go pilot, the Go side is built for a user that does not exist yet.
   Options: (a) one Go service of yours as the Go pilot for logs, executions and delivery (my default; write which); (b) Go is built to the same specification, tested only with the cross tests, and the first real Go user comes later; (c) Python first and Go afterwards (this drops REQ-CNF-007 for a while).
   *Default: (a).*
   A:B

6. **What is the lowest Python version that really runs on your hardware?** *(Q06-2: "3.1X.X, some hardware may use old versions".)* "3.1X" does not name a minimum. It also contradicts "old versions": 3.10 is from 2021. For reference, the Python of Raspberry Pi OS is 3.7 on Buster, 3.9 on Bullseye and 3.11 on Bookworm, so an older board may not reach 3.10.
   Write the lowest `python3 --version` you must support, or the OS of the oldest device.
   *Default: 3.10, closed only after you check the devices.*
   A:Default, My Pi runs at 3.11.4

7. **Do your Python projects use `asyncio`, threads, or plain synchronous code?** *(Q03-6.)* The library sends in the background (REQ-DLV-002). In Python that needs either a thread, or `asyncio`. It decides the shape of the API (`tel.info(...)` or `await tel.info(...)`).
   Options: (a) plain synchronous calls; the library runs its own worker thread (my default: it works everywhere, including in an `asyncio` program); (b) an extra `asyncio` API besides the synchronous one; (c) write what your projects use.
   *Default: (a).*
   A:Default

8. **Is there a Python architecture standard like the Go one?** *(Q02-7: "the Go must follow the standard".)* I understand "the standard" as the organization's Go Clean/Hexagonal standard. REQ-ARC-009 says Python replicates the same layers and names, but names in Python are `snake_case` functions and `PascalCase` classes and I have no written Python standard.
   Options: (a) Python follows the same layers and folder names, with PEP 8 naming, and I write the Python rules into the architecture document (my default); (b) there is a Python standard I must read first (write where).
   *Default: (a).*
   A:Default, but it exists but its not reachable right now.

---

## C. Delivery and receiver

9. **Who builds the receiver of the new HTTP events, and when?** *(Q03-1 default.)* You accepted that the receiver is a separate project. Until it exists, nothing can be tested end to end, and the existing endpoints (a generic HTTP endpoint, and the `add_log` endpoint of aws_microservice) do not accept the new envelope.
   Options: (a) the receiver is built later, in its own proposal; meanwhile the tests use a fake receiver inside the test code (my default); (b) the receiver is built first, because the pilots need it; (c) the existing `add_log` endpoint is adapted to accept the envelope.
   *Default: (a).*
   A:Default

10. **What happens to the existing AWS HTTP sink?** *(Q01-3: `pkg/logging` disappears; Q03-1.)* The current library sends to aws_microservice (CloudWatch Logs). The proposal does not list it among the adapters. If it disappears, the project that uses it loses that route.
    Options: (a) it stays as an adapter of the new library (`outbound/logging/aws_http/`) and translates the `log.*` event to the `add_log` contract (my default); (b) it is dropped; the CloudWatch route moves behind the new receiver; (c) decide when the receiver exists.
    *Default: (a).*
    A:Default

11. **A spool with no quota.** *(Q04-5: the spool stays "as long as needed" and the project cleans it.)* REQ-SPL-001 requires a configurable quota and a rule for the full state, so a device that never cleans up cannot fill its disk and break itself.
    Options: (a) a configurable quota that is **large by default** (my default: 256 MiB, as in Q03-5), a public cleanup function the project can call, and the full-state rule of 10.3; (b) no quota at all (the disk can fill); (c) no quota by default, and the library warns in diagnostics when the spool passes a size you set.
    *Default: (a).*
    A:Default

12. **What is the "SQLite" adapter for?** *(Q04-6: "start with sqlite".)* There are now two different uses of SQLite, and I want to be sure which one you mean. The spool (Q03-2) is the internal queue, and the project never reads it. An SQL destination (7.2) stores the delivered events as rows in a database that someone can query.
    Options: (a) SQLite for the spool only; there is no SQL destination in v1; (b) both: the spool, and an SQL destination that writes events to a local SQLite file a project can query on the device (my default, because you answered "SQLite" to a question about SQL destinations); (c) the SQL destination only.
    *Default: (b).*
    A:B, there will be different sqlites depending on the usage. Like one for spool and another for storage.
    > **Update 2026-10-28:** Clarified by Q08-5: the storage SQLite is a destination, not a second file of the library. Q09-5 settles its layout.

13. **Do devices without a real-time clock send wrong timestamps?** *(Q04-5, offline devices.)* A Raspberry Pi has no battery clock. After a boot without network its UTC time can be wrong until it syncs, and events are written with that time (REQ-CTR-008).
    Options: (a) the library records the clock source and a `clock_synced` flag as an attribute, and the receiver decides what to do with an unsynced time (my default); (b) the library never writes an event before the consumer says the clock is synced; (c) not a concern for your devices.
    *Default: (a).*
    A:Default, bot values must be stored, not only for devices, in all calls, because they can be on UTC or another timeframe.
    > **Update 2026-10-28:** Superseded by Q08-1 and Q09-1: the two times are the generation time and the recording time, both in UTC. The local time and the offset are dropped; `time_zone` is only for display.

---

## D. AI and the scope of v1

14. **LangChain and LlamaIndex have no Go version of the same kind.** *(Q05-2: "all three, I prefer more and unused".)* LlamaIndex is a Python and TypeScript framework. The proposal requires every adapter to exist in both languages with the same name and behavior (REQ-ARC-011, REQ-AI-009). That cannot hold for a framework that only exists in Python.
    Options: (a) the framework adapters (LangChain, LlamaIndex) are Python-only; in Go the capability is marked "not applicable" in the registry and does not block REQ-CNF-007 (my default); (b) Go also gets an adapter for the nearest Go framework (write which); (c) drop the framework adapters from v1 and keep generic callbacks and Langfuse.
    *Default: (a).*
    A:Default

15. **Who checks Langfuse support for Go?** *(Q05-3 default: SDK in Python, HTTP/OTLP in Go "if Go has no official SDK; must be checked".)* I have not checked it.
    Options: (a) I check it when the AI work starts and write the result as an ADR (my default); (b) you already know the answer (write it).
    *Default: (a).*
    A:Default

16. **In which order are the capabilities built?** *(Q02-5: all in v1, one by one; the next starts when the previous one is done.)* The order decides what the pilots can use first.
    Options: (a) core and delivery (envelope, logs, spool, HTTP), then executions and faults, then devices, then AI, then metrics and traces, then audit (my default: your two real projects are devices and AI, so they come early and logs are the base of all); (b) AI first, then devices; (c) write your own order.
    *Default: (a).*
    A:Default

---

## What I will change in PR-001 once you answer

These parts of the proposal now disagree with your answers and will be corrected:

| Your answer | Text of PR-001 that changes |
|---|---|
| Q06-3: ADR-004 stays (the library never reads the environment) | REQ-ARC-007 and REQ-CFG-003 (the file and the environment in the precedence list), and the profiles as functions |
| Q01-3, Q01-5: `pkg/logging` disappears, no deprecation period | the "How it relates to what exists today" table |
| Q03-6: no synchronous mode | section 10.1 and the failure matrix |
| Q04-6: SQLite first | already recorded in 7.2 and D-03; may change with question 12 |
| Q05-2: all three frameworks | REQ-AI-009; may change with question 14 |
| Q06-1: English only | D-07 is closed |
| Q02-5: all event types in v1, one by one | section 13.3 (an order of capabilities, question 16) |
