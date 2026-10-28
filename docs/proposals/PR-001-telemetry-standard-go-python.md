# PR-001 — The new telemetry standard: one contract for Go and Python, devices and AI agents

| | |
|---|---|
| Status | **Accepted by the maintainer on 2026-10-28 (version 0.2).** The decisions of the questionnaires Q01 to Q09 and the four answers of [T-01.1](../tasks/T-01_repository-foundation/T-01.1_accept-pr-001-version-0-2.md) are written into it. Nothing in it is built: the library today is the Go logging library described in [README.md](../../README.md). |
| Layers | all (domain, application, infrastructure, composition root), plus a new `contracts/` directory and a Python implementation |
| Languages | Go (>= 1.25) and Python (>= 3.10) |
| Related | [ADR-001](../decisions/ADR-001-adopt-shared-go-standard.md), [ADR-002](../decisions/ADR-002-one-module-capabilities-side-by-side.md), [ADR-003](../decisions/ADR-003-public-surface-is-pkg-logging.md), [ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md), [ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md), [ADR-006](../decisions/ADR-006-masking-before-every-sink.md), [architecture](../architecture/architecture.md), [requirements](../requirements/requirements.md) |
| Replaces | when built, [ADR-002](../decisions/ADR-002-one-module-capabilities-side-by-side.md) and [ADR-003](../decisions/ADR-003-public-surface-is-pkg-logging.md), by [ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md). The decisions are recorded in [ADR-011](../decisions/ADR-011-adopt-the-telemetry-standard-go-python.md), [ADR-012](../decisions/ADR-012-two-times-in-every-event.md) and ADR-013; they were accepted with this proposal on 2026-10-28. |
| Decided in | [Q01](../setup/questionnaires/Q01_scope-repository-layout-and-rollout.md) to [Q09](../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) (2026-10-27 and 2026-10-28) |

## Why

Today the library only logs: console, HTTP and AWS HTTP sinks, masking and a call context (`pkg/logging`). Projects that run on hardware (sensors, actuators, robots) and projects that run AI agents need to record much more than logs (metrics, traces, executions, health, device readings, model calls) and they are written in Go **and** in Python. If each project builds its own recording and delivery code, the data of an agent in Go and of sensor software in Python cannot be shown in one interface.

This proposal defines a single, versioned, language-independent **contract** (`contracts/`) and two implementations of the same architecture, so that everything common is identical in Go and Python.

## What the proposal contains

| Topic | Where |
|---|---|
| A passive library that centralizes logs, metrics, traces, executions, health, device data, AI agent data and generic reports | sections 1 and 6 |
| Architecture: Clean + Hexagonal, identical layers, names and adapters in both languages | section 3 |
| Distribution: Go core module plus one module per adapter, chosen in code; Python package with extras, installed from Git | section 4 |
| The contract: event envelope, identity, severity, time and identifiers, attributes | section 5 |
| Outbound adapters, the SQLite destination and the HTTP default destination | section 7 |
| Device and hardware module (`device.*`) | section 8 |
| AI agent module (`ai.*`) | section 9 |
| Delivery modes, queues, priorities, spool, shutdown and the failure matrix | section 10 |
| Security and privacy, self-observability | sections 11 and 12 |
| Parity between languages and conformance tests | section 13 |
| Configuration, versioning, definition of done | sections 14, 15 and 17 |

## How it relates to what exists today

The proposal is the **target**. Until it is built, the library stays what [README.md](../../README.md) describes. When it is built, the new Go module `github.com/DanielCalvo-Calvicia/telemetry/go` replaces the current one. The old module path `github.com/DanielCalvo-Calvicia/telemetry` (tags `v0.1.0` and `v0.2.0`) stays available; it is a different path, so the two never clash.

| Today (built) | In this proposal | Decided in |
|---|---|---|
| One Go module, public surface `pkg/logging` ([ADR-002](../decisions/ADR-002-one-module-capabilities-side-by-side.md), [ADR-003](../decisions/ADR-003-public-surface-is-pkg-logging.md)) | The old surface disappears, with no deprecation period: its only consumer takes it from the published package. A core module plus one module per adapter, with public packages `go/pkg/<functionality>/<adapter>` ([ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md)) | Q01-3, Q01-5, Q08-2, Q08-3, Q09-3 |
| The library never reads the environment ([ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md)) | **Stays.** No environment and no file: the consumer loads its configuration and passes it in | Q06-3 |
| A failing sink never fails the caller; the send is synchronous, 3 s timeout, no retry ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)) | The rule stays. The synchronous send goes away: a bounded queue and a worker, with three delivery modes (section 10) | Q03-6 |
| Masking before every sink ([ADR-006](../decisions/ADR-006-masking-before-every-sink.md)) | Stays. The new redaction list is added to the current detectors; nothing is removed | Q06-5 |
| An entry with fixed fields (`Application`, `Category`, `Function`, `Message`) | A `log.entry.recorded` event: category, function and message in `data`, the application in `resource.service_name` | Q02-4 |
| The AWS HTTP sink ([ADR-008](../decisions/ADR-008-aws-http-sink-adapts-to-add-log.md)) | Stays as an adapter in its own module; it translates the `log.*` event to the `add_log` contract | Q07-10 |
| No metrics (planned) | Metrics are built in their turn (see the build order in 13.3) | Q07-16 |

## Decisions taken

The pending decisions of the first draft, and what the maintainer answered:

| # | Decision | Source |
|---|---|---|
| D-01 | The spool is a SQLite file. Go uses a pure-Go driver (no cgo); Python uses the standard `sqlite3` | Q03-2, Q09-4 |
| D-02 | The contract is JSON Schema | Q02-1 |
| D-03 | The SQL destination starts with SQLite; another dialect is added when a project needs it | Q04-6 |
| D-04 | `event_id` is a UUIDv7 | Q02-2 |
| D-05 | Closed: the library has no hardware drivers; the consumer keeps its own | Q04-2, Q04-3 |
| D-06 | Generic callbacks, Langfuse, LangChain and LlamaIndex. The two frameworks exist only in Python; in Go they are "not applicable" | Q05-2, Q07-14 |
| D-07 | English for everything in the repository | Q06-1 |
| D-08 | Python 3.10 is the minimum (the Raspberry Pi of the pilot runs 3.11.4) | Q06-2, Q07-6 |
| D-09 | Langfuse: its SDK in Python and its HTTP/OTLP API in Go, with the same observable behavior. Support in Go is checked when the AI work starts and written as an ADR | Q05-3, Q07-15 |

Other decisions:

| Topic | Decision | Source |
|---|---|---|
| Repository | One repository with `contracts/`, `go/` and `python/`. The Go module lives under `go/` and its tags are `go/vX.Y.Z`; it starts at `go/v0.3.0` | Q01-2, Q07-1, Q07-2 |
| Distribution | Python is installed from the Git repository (no PyPI for now). The generated code of both languages is committed, so a Python project never needs Go | Q07-3, Q07-4 |
| Pilots | Both known projects are Python (hardware on a Raspberry Pi, an AI agent that uses Langfuse). There is no Go pilot: Go is built to the same specification and tested with the cross tests | Q01-1, Q07-5 |
| Python | The API is synchronous and the library runs its own worker thread. PEP 8 and the Go layers are the rules until the Python standard of the organization can be read | Q07-7, Q07-8 |
| Receiver | A separate project, later. Meanwhile the tests use a fake receiver | Q03-1, Q07-9 |
| Spool | 256 MiB per destination by default, under a directory the consumer gives, and a public cleanup function. The spool stays until the project cleans it, within the quota | Q03-5, Q04-5, Q07-11 |
| SQLite destination | A destination written to a local file, like the AWS HTTP sink but to SQLite. Its layout is part of `contracts/`; a reader (a UI or another process) only reads | Q07-12, Q08-5, Q09-5 |
| Times | Every event has `occurred_at` and `recorded_at`, both UTC, and `resource` may have a `time_zone` for display only | Q08-1, Q09-1 |
| Adapters | One `go.mod` per adapter; the project imports the adapters it needs and passes them in code | Q08-2, Q08-3, Q09-2, Q09-3 |
| Defaults | Queue 10,000 events, batch 100, flush every 2 s, timeout 3 s; 32 attributes, key 64 and value 256 characters, event 64 KiB, HTTP batch 1 MiB; the delivery mode per profile (REQ-CFG-004) | Q03-3, Q03-4, Q08-9 |
| HTTP | A bearer token and an API-key header; the body is `{"schema_version", "events": [...]}`, optional gzip | Q08-6, Q08-7 |
| Backends | None today: the OpenTelemetry and Prometheus adapters are built when a project has a backend | Q08-8 |
| Audit | The contract is written; the adapter comes later | Q06-6 |
| Tests | Python has the same 95% coverage gate; the cross tests run in CI | Q06-7 |
| Releases | One tag per language (`go/...` and `python-vX.Y.Z`); the contract is versioned by `schema_version` | Q06-8 |
| Public names | The Go core is `go/pkg/telemetry`; Python is `telemetry.<functionality>.<adapter>`, installed as `telemetry` | T-01.1 |
| Old library | The root `go.mod` of the old library stays beside `go/go.mod` until T-04.7 removes it; `go.work` joins them | T-01.1 |
| Contract releases | Every contract change, even one that only adds an optional field, is implemented in both languages and released in both; one language never releases it alone | T-01.6 |
| Contract changes | The maintainer decides, by pull request and a dated note in an ADR | Q01-4 |
| Scope | All event types are in v1, built one at a time. This proposal stays the umbrella; one proposal per area is written when it is accepted | Q02-5, Q01-6 |
| `1.0` | Only when every capability works. Until then the library is a beta (`0.x`); `schema_version` starts at `0.1` and reaches `1.0` with the library | Q09-6, Q02-6 |

## Known risks

- **No receiver and no Go pilot.** The HTTP contract and the whole Go side are tested only against fakes and the cross tests until a real receiver and a Go project exist.
- **Many Go modules.** Every adapter has its own `go.mod` and its own tag. The core is released first and a `go.work` file is committed for local work; releases are slower than with one module. The rule that `internal/` packages of the core are importable from a nested module (the rule is by path) is to be verified when ADR-013 is built.
- **The pure-Go SQLite driver.** The maintainer wanted speed. Pure Go was chosen for a simple build (no C compiler, easy cross-compiling to a Raspberry Pi); a benchmark can reopen the choice.
- **The Python standard is not available yet.** The Python rules are written from PEP 8 and the Go layers and are reviewed against it later.
- **A large scope.** `1.0` waits until every capability works, so the first projects use `0.x` for a long time.

## Still to decide

Nothing from the first questionnaires. The four points of T-01.1 were answered on 2026-10-28: the proposal is accepted as it is; the Python names are `telemetry.<functionality>.<adapter>`, installed as `telemetry`; the public package of the Go core is `go/pkg/telemetry` (the entry point `New`, `Shutdown` and the shared types); and the old root `go.mod` stays beside `go/go.mod` until T-04.7 removes the old library. The open points left are the questions written in the tasks themselves.

## Documentation to update when it is accepted

Following the rule of `CLAUDE.md`: [architecture](../architecture/architecture.md) (layout and sections 2, 3 and 7), [api](../api/api.md), [environment variables](../configuration/environment-variables.md), [requirements](../requirements/requirements.md), [security](../security/security.md), [monitoring](../operations/monitoring.md), [deployment](../deployment/deployment.md), the status table of [README.md](../../README.md) and the index of [docs/README.md](../README.md).

---

## Specification

The text below is the normative draft. It uses the keywords of RFC 2119 and stable identifiers `REQ-<AREA>-<nnn>`. In it, "section N" refers to the numbered sections of this specification, not to the sections of this proposal.

### 1. Purpose and scope

#### 1.1 What it is

Telemetry is a **passive library** that projects import. Its job is to centralize in one place everything a program
needs to record, either to process it later or to watch it in real time:

- logs, errors and failures;
- metrics and traces;
- executions (cronjobs, workers, tasks, missions);
- health status and heartbeats;
- sensor values and actuator changes;
- AI agent executions, model calls and tools;
- generic operational reports.

The main project imports only the modules it needs and uses them. The library saves that project from implementing its
own recording and delivery infrastructure.

#### 1.2 Passivity principle

The library is **agnostic** about the project that uses it:

- **It does not decide what is captured.** The consuming project decides.
- **It does not decide the data policy** (what is sensitive, how long it is kept). The library offers the mechanisms
  (redaction, limits, classification) and the consumer configures them.
- **It does not decide the destination.** The consumer chooses the outbound adapters.
- **It does not start activity by itself.** No implicit network, no threads or processes that start unless the consumer
  builds and starts the library.

#### 1.3 Non-goals

The library is **not**:

- a backend, collector or receiver of data;
- a user interface or a query engine (a future UI will consume the contracts of this document);
- a safety, flight-control or autonomous-navigation system;
- an audit system with legal guarantees of immutability (REQ-SEC-006).

#### 1.4 Relation to device control

From v1 the library includes device modules (section 8) and AI agent modules (section 9), because it will be used by
hardware and by agents and everything must be recorded in the same flow as the rest of the data. These modules
**record and report**; the physical safety restrictions always belong to the consumer's code (REQ-DEV-001).

---

### 2. Normative conventions

The words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT** and **MAY** are interpreted as in RFC 2119.

- Every requirement carries a stable identifier `REQ-<AREA>-<nnn>`.
- Every requirement must be linked, in `contracts/capabilities/registry.yaml`, to at least one conformance test.
- Text without an identifier is context, not a requirement.

---

### 3. Architecture principles

**REQ-ARC-001.** Dependencies MUST point in the direction `public API / inbound adapters -> application -> domain`.
Outbound adapters implement the outbound ports defined by the application layer.

**REQ-ARC-002.** The domain MUST NOT depend on HTTP, databases, the file system, OpenTelemetry, drivers, AI frameworks
or the environment.

**REQ-ARC-003.** The wiring of dependencies MUST happen only in a composition root, with constructor injection: the one
of the core for its own adapters (console, file, HTTP) and the consumer's own for the optional adapters it imports and
passes in (REQ-ARC-013).

**REQ-ARC-004.** Abstractions MUST represent capabilities of the platform, not replicate methods of third-party
libraries.

**REQ-ARC-005.** The core MUST have a minimal set of external dependencies. Every adapter with its own dependencies
(OpenTelemetry, Prometheus, SQL drivers, hardware, AI frameworks) MUST be installed separately from the core
(section 4). The adapter code is part of the library.

**REQ-ARC-006.** The library MUST NOT use mutable global state. If a convenience global instance is offered, it MUST be
optional and documented.

**REQ-ARC-007.** The library MUST NOT read environment variables or configuration files at all (ADR-004). The consumer
loads its own configuration and passes it in.

#### 3.1 Identical internal structure in both languages

Go and Python are two implementations of the **same** architecture. The internal structure is normative and must be
practically a copy.

**REQ-ARC-008.** Both implementations MUST have architecture tests that verify the direction of dependencies
(REQ-ARC-001/002/003), and both MUST keep a coverage of 95% or more, without the folders that do real I/O (`external/`),
as a CI gate.

**REQ-ARC-009.** Go and Python MUST have the same layers, the same directory and package names, and the same file
names. Only the differences the language forces are allowed (`__init__.py`, the `_test.go` suffix, `pkg/` versus
`src/`, the file extension).

**REQ-ARC-010.** Public names (types, functions, methods, options) MUST be equivalent through a mechanical rule
documented in `contracts/conventions/naming.md`: Go uses exported identifiers in `PascalCase`; Python uses `snake_case`
for functions and `PascalCase` for classes. A concept MUST NOT be renamed in one language. The JSON fields are
`snake_case` in both languages (the Go struct tags do the mapping) and the Go code follows the Go standard of the
organization.

**REQ-ARC-011.** Each adapter MUST be a subdirectory with the same name in both languages (for example
`outbound/ai_telemetry/langfuse/`, `outbound/ai_runtime/langchain/`). The only exception is an adapter for a framework
that exists in one language (REQ-AI-011).

**REQ-ARC-012.** The public packages MUST follow `<functionality>/<adapter>` in both languages:
`go/pkg/<functionality>/<adapter>` in Go and `telemetry.<functionality>.<adapter>` in Python. A project imports only the
packages it needs. The public package of the core itself, with `New`, `Shutdown` and the shared types, is `go/pkg/telemetry`.

**REQ-ARC-013.** The project MUST choose its adapters by importing them and passing them to the core in code (for
example `telemetry.New(config, sqlite.NewDestination(...), http.NewDestination(...))`). The configuration carries only
the settings of each adapter, never an adapter name. Adapters MUST NOT register themselves on import.

**REQ-ARC-014.** The adapters of the core (console, file, HTTP) live in the core module under
`internal/infrastructure/outbound/<functionality>/<adapter>/`. Every optional adapter is a module of its own at
`go/pkg/<functionality>/<adapter>/` and keeps the same folders inside (`external/`, `error/`, `helpers/`). In Python the
same folders are under `python/src/telemetry/`.

Reference internal structure (the root is `go/internal/` in Go and `python/src/telemetry/internal/` in Python):

```text
domain/
  base_value_objects/
  value_objects/
  entities/
  operations/
application/
  dtos/
  mappers/
  ports/
    inbound/
    outbound/
  services/
infrastructure/
  config/
  inbound/
    sdk/
    cli/
  outbound/
    logging/
    metrics/
    tracing/
    reports/
    device_telemetry/
    ai_runtime/
    ai_telemetry/
    audit/
    shared/
    local_spool/
composition_root/
  config/
  containers/
  dependencies/
```

The tests root (`go/internal/tests/` and `python/internal/tests/`) is also identical:
`architecture/`, `fixtures/`, `contract/`, `domain/`, `application/`, `infrastructure/`, `composition_root/`, `e2e/`.

The organization's Go architecture standard is the base reference. Python replicates it.

---

### 4. Distribution and modularity

**REQ-DST-001.** Go MUST be distributed as one core module (`github.com/DanielCalvo-Calvicia/telemetry/go`) and one
module per adapter, each with its own `go.mod` and its own tag (`go/vX.Y.Z` for the core,
`go/pkg/<functionality>/<adapter>/vX.Y.Z` for an adapter). Each adapter pins a minimum version of the core, and the core
is released first. A `go.work` file is committed for local work and CI. Importing the core MUST NOT pull in the
dependencies of any adapter.

**REQ-DST-002.** Python MUST be distributed as one package with optional *extras* (for example `telemetry[sqlite]`,
`telemetry[otel]`, `telemetry[prometheus]`, `telemetry[ai]`). It is installed from the Git repository, for example `pip
install "telemetry[sqlite] @ git+https://github.com/DanielCalvo-Calvicia/telemetry@python-vX.Y.Z#subdirectory=python"`;
publishing it to a package index is out of scope for now.

**REQ-DST-003.** The library MUST support Go >= 1.25 and Python >= 3.10. Changing these minimums MUST be treated as a
compatibility change (section 15).

**REQ-DST-004.** The device and AI modules are part of the library and of every release. The third-party dependencies of
their adapters (Langfuse SDK, AI frameworks) MUST be installed separately (an own module in Go, an extra in Python), so
that a project installs only the ones it uses.

**REQ-DST-005.** The code generated from `contracts/` for both languages MUST be committed, so that installing the
Python package never needs Go. CI MUST fail when the generated code differs from the contract (REQ-CNF-002).

---

### 5. Common contract

The `contracts/` directory is the source of truth between languages. A future UI or backend is built against these
contracts.

#### 5.1 Event envelope

**REQ-CTR-001.** Every emitted event MUST use a versioned envelope with these fields:

| Field | Required | Description |
|---|---|---|
| `schema_version` | yes | Version of the contract (`MAJOR.MINOR`). |
| `event_id` | yes | Unique identifier of the event (section 5.4). |
| `event_type` | yes | Type in `domain.entity.action` notation, from a registered list. |
| `occurred_at` | yes | UTC instant when the data was generated at the source, RFC 3339 with the `Z` zone (section 5.4). |
| `recorded_at` | yes | UTC instant when the library accepted the event. Only the library sets it (section 5.4). |
| `severity` | yes | Value of the enum in section 5.3. |
| `resource` | yes | Identity of the emitting process (section 5.2). |
| `actor` | no | The concrete subject the event refers to: device, agent, subsystem (section 5.2). |
| `context` | no | Correlation: `trace_id`, `span_id`, `execution_id`, `correlation_id`. |
| `data` | yes | Payload typed by `event_type`. It may be empty. |
| `attributes` | no | Key-value extensions of the consumer (section 5.5). |

**REQ-CTR-002.** Each `event_type` MUST have a JSON schema for `data` registered in `contracts/schemas/`. An event
whose `data` does not validate MUST be rejected at the source with a classified error, without throwing to the hot-path
caller (REQ-DLV-002).

**REQ-CTR-003.** The JSON schema is the canonical definition. The Go and Python types MUST be derived from it or
validated against it.

**REQ-CTR-004.** Two implementations MUST produce the same canonical JSON for the same logical event (section 13).

#### 5.2 Identity (resource)

The set is kept small to avoid overlap. Each concept has a single field.

| Field | Required | Meaning |
|---|---|---|
| `service_name` | yes | Logical name of the application or service. |
| `instance_id` | yes | Identifier of this running instance (process/pod). Generated at start if not supplied. |
| `environment` | no | Environment (`prod`, `staging`, ...). |
| `service_version` | no | Version of the application. |
| `host` | no | Host name. |
| `time_zone` | no | IANA time zone of the source (for example `Europe/Madrid`). It is only for display and is never used to compute a time. |

`actor` (a field of the envelope, not of `resource`) describes the concrete subject of the event with a **single**
structure for sensors, actuators, agents and subsystems:

| Field of `actor` | Required | Meaning |
|---|---|---|
| `kind` | yes | Class of subject (`device`, `agent`, `subsystem`, ...), from `contracts/enums/`. |
| `id` | yes | Identifier of the subject. |
| `type` | no | Concrete type (sensor model, agent type). |
| `version` | no | Firmware, agent or model version. |

**REQ-CTR-005.** The identity of the resource (process), of the actor, of the execution and the correlation MUST be
separate concepts. One process can host several actors (several agents, several devices).

**REQ-CTR-006.** `execution_id`, `request_id` and any high-cardinality identifier MUST NOT be used as a metric
attribute (REQ-MET-003).

#### 5.3 Severity

Values: `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `CRITICAL`. The semantics of each one is documented in
`contracts/enums/severity.md`. Implementations MUST map them to the native level of their logging system and to
OpenTelemetry.

#### 5.4 Time and identifiers

**REQ-CTR-007.** The clock and the identifier generator MUST sit behind ports (`ClockPort`, `IDGeneratorPort`) to allow
deterministic tests.

**REQ-CTR-008.** Every event MUST carry two instants, both in UTC and in the fixed format of REQ-CNF-003: `occurred_at`,
when the data was generated at the source (the consumer MAY give it; otherwise it is the time of the call), and
`recorded_at`, when the library accepted the event (set only by the library, from its `ClockPort`; the consumer MUST NOT
set it). When the data carries `observed_at`, `occurred_at` MUST equal it. The library MUST add the attributes
`clock_source` and `clock_synced` when it knows them, because on a device without a real-time clock both instants can be
wrong until it syncs. The time the receiver stores the event (`received_at`) belongs to the receiver and is not in the
envelope. Durations MUST be measured with a monotonic clock where the runtime supports it, without mixing them with
wall-clock time.

**REQ-CTR-009.** `event_id` MUST be a UUIDv7 (D-04). It MUST NOT be derived from a timestamp alone.

#### 5.5 Extension attributes

**REQ-CTR-010.** `attributes` MUST accept only scalar values and lists of scalars, with configurable limits on the
number of keys, key length and value length (defaults: 32 keys, 64 characters per key, 256 characters per value).
Excesses are truncated or dropped according to a documented policy and counted in diagnostics.

---

### 6. Observability capabilities (core)

The registry `contracts/capabilities/registry.yaml` lists each capability with: identifier, public API, schemas,
configuration, error semantics, ports and adapters, Go status, Python status and conformance tests.

#### 6.1 Logging

**REQ-LOG-001.** MUST offer structured logging with JSON output and human-readable console output.

**REQ-LOG-002.** MUST integrate with native logging: `log/slog` in Go and `logging` in Python.

**REQ-LOG-003.** MUST support configurable field redaction and level filtering.

**REQ-LOG-004.** MUST allow several independent destinations at the same time (section 7).

**REQ-LOG-005.** MUST attach `trace_id`, `span_id` and `execution_id` when they exist in the context.

**REQ-LOG-006.** By default MUST NOT record request bodies or arbitrary payloads.

**REQ-LOG-007.** A log call MUST produce a `log.entry.recorded` event whose `data` holds the category, the function and
the message, and whose `resource.service_name` is the application. Extra fields go to `attributes`.

#### 6.2 Metrics

**REQ-MET-001.** MUST support counters, up/down counters, gauges, histograms and duration measurement.

**REQ-MET-002.** Each metric MUST have a name, a description and a unit.

**REQ-MET-003.** MUST apply a configurable cardinality limit per metric. When it is exceeded, new series are grouped
under an `overflow` value and the event is recorded in diagnostics.

**REQ-MET-004.** Metrics MUST NOT be used as a store of raw high-frequency readings. Measurement events
(`device.measurement.recorded`, section 8.2) exist for that.

#### 6.3 Traces and context propagation

**REQ-TRC-001.** MUST support spans with parent/child relations, error status and duration.

**REQ-TRC-002.** MUST propagate context with the W3C Trace Context standard for HTTP, so that Go and Python correlate
with each other.

**REQ-TRC-003.** Incoming trace context MUST be validated and MUST NOT be treated as authorization.

#### 6.4 Executions

**REQ-EXE-001.** MUST offer one shared execution abstraction for cronjobs, workers, tasks, AI executions and missions.

**REQ-EXE-002.** States: `started`, `in_progress`, `completed`, `failed`, `cancelled`, `timed_out`, `retrying`,
`degraded`.

**REQ-EXE-003.** The terminal states are `completed`, `failed`, `cancelled` and `timed_out`. The valid transitions are
defined in `contracts/enums/execution_transitions.yaml` and both implementations MUST reject the invalid ones.

**REQ-EXE-004.** An execution event MUST distinguish the state of the execution from the delivery state of the report.

**REQ-EXE-005.** An execution MUST support `execution_id`, `parent_execution_id`, attempt number, progress, error
classification and cancellation reason.

#### 6.5 Health and heartbeats

**REQ-HLT-001.** MUST offer liveness status, readiness, health per dependency and per subsystem, degraded status, last
successful operation and last heartbeat.

**REQ-HLT-002.** MUST NOT require an HTTP server. It offers interfaces that the consumer exposes through its own
adapter.

**REQ-HLT-003.** A failure of the telemetry exporter MUST NOT by itself mark the application as unhealthy.

#### 6.6 Failures and generic reports

**REQ-FLT-001.** MUST offer a failure classification model (code, category, severity, recoverable or not) defined in
`contracts/enums/`.

**REQ-RPT-001.** MUST offer a generic report (`report.*`) with a typed payload for operational events that do not fit
another capability.

---

### 7. Outbound adapters

Each capability emits its data through **outbound ports**. Each port can have several adapters active at once and the
choice belongs to the consumer.

#### 7.1 Ports

`LogSinkPort`, `MetricsSinkPort`, `TraceSinkPort`, `EventSinkPort`, `DeviceTelemetrySinkPort`, `AITelemetrySinkPort`,
`LocalSpoolPort`, `ClockPort`, `IDGeneratorPort`, `ConfigurationPort` (it hands the core the configuration the consumer passed in; it reads no file and no environment, REQ-ARC-007).

**REQ-OUT-001.** Ports with identical semantics MAY share a generic abstraction. Payloads that are semantically
different MUST NOT be forced into an untyped interface.

#### 7.2 Planned adapters

| Adapter | Logs | Events/reports | Metrics | Traces | Notes |
|---|:-:|:-:|:-:|:-:|---|
| Console | yes | yes | - | - | Core. |
| Local file (JSON Lines) | yes | yes | - | - | Core. Configurable rotation and quota. |
| HTTP (own schema) | yes | yes | - | - | Core. **Default destination.** |
| AWS HTTP (`add_log` of aws_microservice) | yes | - | - | - | Optional, own module. Translates the `log.*` event to the `add_log` contract (ADR-008). |
| OpenTelemetry (OTLP) | yes | - | yes | yes | Optional, own module. Built when a project has a backend. |
| Prometheus | - | - | yes | - | Optional, own module. Built when a project has a backend. |
| SQL (SQLite first) | - | yes | - | - | Optional, own module. A destination like the AWS HTTP sink, but writing to a local SQLite file (REQ-OUT-009). Another dialect is added when a project needs it. |
| Langfuse | - | yes (AI) | - | yes (AI) | Included in v1 (REQ-AI-009). |

**REQ-OUT-002.** HTTP is the default destination: it is in the core and the profiles use it. Any other destination is
added by importing its adapter and passing it to the core (REQ-ARC-013); the HTTP destination always needs its URL in
the configuration (REQ-OUT-005).

**REQ-OUT-003.** Each capability MUST be routable to different destinations and to several destinations at once. Using
the same destination for several capabilities MUST be an explicit configuration decision.

**REQ-OUT-004.** The OpenTelemetry and Prometheus adapters MUST use the native protocol of the backend. They MUST NOT
send OTLP data with the own schema.

**REQ-OUT-005.** The HTTP adapter MUST support: configurable URL, method, configurable authentication, TLS
verification, timeout, retries with exponential backoff and jitter, batches, payload size limit, optional compression
and response validation. It MUST NOT include fixed URLs in code.

**REQ-OUT-006.** The shared HTTP infrastructure (client, TLS, authentication, retries, rate limits, response
classification) MUST be implemented once per language and reused by all HTTP adapters. Capability-specific adapters
only contribute the semantics of the payload.

**REQ-OUT-007.** Adding a new adapter MUST require only implementing the corresponding port, without touching the domain
or the application.

**REQ-OUT-008.** The authentication of the HTTP destination MUST support a bearer token (`Authorization` header) and an
API key in a custom header first. The value always comes from the consumer's configuration, never from a file of the
repository.

**REQ-OUT-009.** The SQLite destination MUST write one `events` table: the whole envelope as JSON plus indexed columns
(`occurred_at`, `recorded_at`, `event_type`, `severity`, actor id, `event_id`). The layout is part of `contracts/`,
identical in Go and Python and versioned. The file MUST be opened in WAL mode and carry its layout version in `PRAGMA
user_version`. A reader (a UI or another process on the same machine) opens it read-only and never writes; retention
(deleting old rows) is done only by the writer, and the rows stay until the project cleans them. Because writing to a
local file is already durable, this destination does not use the spool.

**REQ-OUT-010.** The body of an HTTP batch MUST be one JSON object `{"schema_version": "0.1", "events": [ ... ]}` with
the envelopes in the array. Gzip MAY be used (`Content-Encoding: gzip`).

---

### 8. Devices and hardware

#### 8.1 Scope

This section covers sensors, actuators, robots and any other hardware that uses the library. It is **independent** of
section 9 (AI agents): the two domains have their own events, schemas, enums and ports.

The device module is part of the library from v1 and of every release. It does **not** contain hardware drivers: the
project that owns the hardware keeps its own drivers and objects and reports through the library (see 8.4).

**REQ-DEV-001.** The device module MUST NOT implement physical safety restrictions. Those restrictions belong to the
consumer's code or to a dedicated safety subsystem. Telemetry is not a safety mechanism.

**REQ-DEV-002.** Device events MUST use the `device.*` namespace. Their schemas MUST be in
`contracts/schemas/device/` and their enums in `contracts/enums/device/`. Device events MUST NOT be reused for AI
agents, or the other way around.

**REQ-DEV-003.** Device events MAY be combined with the common events `execution.*`, `fault.*` and `report.*`
(section 6), identifying the device through `actor` with `kind=device` (section 5.2).

#### 8.2 Device contracts

The contracts are **generic for all hardware**: there is no schema per sensor model or per actuator type.
Specialization is expressed with `kind`, `name` and `attributes`, with values registered in
`contracts/enums/device/`.

**REQ-DEV-004.** There MUST NOT be schemas specific to a sensor type, actuator type or manufacturer. Any new hardware
MUST be describable with the families of this section without modifying any schema.

| `event_type` | What it represents | Main fields of `data` |
|---|---|---|
| `device.measurement.recorded` | A sensor reading: temperature, voltage, RPM, GPS position, IMU values. | `name`, `value` (number, boolean or text), `unit`, `quality`, `observed_at`, `sensor_id`, `kind` |
| `device.state.observed` | Observed state of a subsystem or component. | `component`, `state`, `previous_state`, `observed_at`, `source` |
| `device.actuator.changed` | Observed change in an actuator (LED, motor, servo, digital output). | `actuator_id`, `property`, `previous_value`, `new_value`, `cause_command_id`, `observed_at` |
| `device.command.requested` | A command requested from the device. | `command_id`, `target`, `operation`, `requested_state`, `requested_at`, `deadline` |
| `device.command.completed` | Result of executing the command according to the consumer's own driver code. | `command_id`, `execution_status`, `started_at`, `completed_at`, `error`, `observed_state` |
| `device.connectivity.changed` | Change in the connectivity of the device. | `link`, `status`, `previous_status`, `observed_at` |
| `device.health.reported` | Health per subsystem (not a single boolean). | `subsystems` (list of `component`, `status`, `detail`), `observed_at` |

**REQ-DEV-005.** The allowed values of `kind` (for example `temperature`, `voltage`, `gnss`, `imu`, `rpm`), of
`quality` (`good`, `uncertain`, `bad`, `stale`), of `execution_status` (`succeeded`, `failed`, `timed_out`,
`cancelled`, `rejected`) and of `status` MUST be in `contracts/enums/device/`. Adding a value is a MINOR change of the
contract (REQ-VER-003).

#### 8.3 Semantic rules

**REQ-DEV-006.** Device reports MUST distinguish four separate things:

1. the command requested (`device.command.requested`);
2. the result of executing the command according to the consumer's own driver code (`device.command.completed`);
3. the observed physical state (`device.state.observed`, `device.actuator.changed`, `device.measurement.recorded`);
4. the delivery result of the telemetry (diagnostics, section 12).

A successful command does not prove that the physical state was reached.

**REQ-DEV-007.** Measurements MUST include value, unit, observation time (`observed_at`), quality and source. The library MUST NOT require a
common set of quantities from all devices.

**REQ-DEV-008.** A sensor reading and an actuator change MUST be distinct events (`device.measurement.recorded` versus
`device.actuator.changed` and `device.command.*`).

**REQ-DEV-009.** For high-frequency readings, batch sending and configurable sampling MUST be offered. The data MUST
go to an events or SQL destination, not to metrics (REQ-MET-004).

**REQ-DEV-010.** A command event MUST carry `deadline` when the consumer has one. A command MUST have a unique
`command_id` to correlate the request, the result and the observed state. The library records the command; the
consumer's own code executes it and honors the deadline.

#### 8.4 Source helpers (v1)

The library does not talk to hardware. The project that owns the hardware has its own drivers and objects, and builds
the telemetry from them (decision of the maintainer, 2026-10-27, see [Q04](../setup/questionnaires/Q04_devices-and-hardware.md)).
What differs between sources is the **input**: a GPIO reading carries a pin and a level, an I2C reading a bus, an
address and a register. What must not differ is the **output**: every source produces the same generic events of 8.2,
so one UI can show all of them without one view per source.

**REQ-DEV-011.** The library MUST offer one typed source helper per kind of source. A helper takes the data that is
specific to that source and fills the generic event (`kind`, `name`, `value`, `unit`, `quality`, `observed_at` and the
source details in `attributes`). The first helpers are:

| Source | Specific input of the helper |
|---|---|
| GPIO | pin, level, pull |
| I2C | bus, address, register |
| SPI | bus, chip select |
| Serial | port, baud rate |
| Camera | resolution, frame format |
| Motor controller | motor id, speed, direction |
| Vendor SDK (generic) | free `attributes` |

**REQ-DEV-012.** A source helper MUST NOT read or drive hardware, and MUST NOT depend on any hardware library. Adding
a helper MUST NOT change the event schemas (REQ-DEV-004). Go and Python MUST have the same helpers with the same names
and fields (REQ-ARC-010).

---

### 9. AI agents

#### 9.1 Scope

This section covers AI agents, model invocations, tools and multi-agent systems. It is **independent** of section 8
(devices): the two domains have their own events, schemas, enums and ports.

The AI module is part of the library from v1 and of every release. Third-party SDKs and frameworks are installed
separately (REQ-DST-004).

**REQ-AI-001.** AI events MUST use the `ai.*` namespace. Their schemas MUST be in `contracts/schemas/ai/` and their
enums in `contracts/enums/ai/`. AI events MUST NOT be reused for devices, or the other way around.

**REQ-AI-002.** AI events MAY be combined with the common events `execution.*`, `fault.*` and `report.*` (section 6),
identifying the agent through `actor` with `kind=agent` (section 5.2). The identity of the agent MUST be independent
of that of the process and of the model. One process can host several agents and several model providers.

#### 9.2 AI contracts

The contracts are **generic for all agents**: there is no schema per framework or per provider. Specialization is
expressed with `attributes` and values registered in `contracts/enums/ai/`.

**REQ-AI-003.** There MUST NOT be schemas specific to a framework, provider or model. Any new agent or provider MUST
be describable with the families of this section without modifying any schema.

| `event_type` | What it represents | Main fields of `data` |
|---|---|---|
| `ai.model.invoked` | Start of a model invocation. | `invocation_id`, `model`, `provider`, `parent_execution_id`, `attempt` |
| `ai.model.completed` | Successful end of an invocation. | `invocation_id`, `duration_ms`, `usage` (`input_tokens`, `output_tokens`, `total_tokens`), `finish_reason` |
| `ai.model.failed` | Failure of an invocation. | `invocation_id`, `duration_ms`, `error` (`code`, `category`, `retryable`), `attempt` |
| `ai.tool.called` | A call from an agent to a tool. | `call_id`, `tool_name`, `parent_execution_id`, `attempt` |
| `ai.tool.completed` | End of a tool call. | `call_id`, `duration_ms`, `outcome`, `error` |
| `ai.agent.handoff` | Handoff of work between agents. | `from_agent_id`, `to_agent_id`, `reason`, `parent_execution_id`, `child_execution_id` |
| `ai.approval.requested` | Request for approval from a human or a policy. | `approval_id`, `subject`, `reason` |
| `ai.approval.resolved` | Resolution of an approval. | `approval_id`, `decision`, `decided_by_kind` |
| `ai.policy.denied` | A policy denied an action. | `policy`, `subject`, `reason` |

**REQ-AI-004.** The life cycle of an agent execution MUST use the common `execution.*` events (section 6.4), with
`parent_execution_id` to express relations between agents and subagents.

**REQ-AI-005.** The allowed values of `outcome`, `finish_reason`, `decision` and the AI error categories MUST be in
`contracts/enums/ai/`. Adding a value is a MINOR change of the contract (REQ-VER-003).

#### 9.3 Semantic rules

**REQ-AI-006.** By default MUST record only metadata. Full prompts, tool results and images MUST require explicit
activation, have a size limit and go through redaction (REQ-SEC-001).

**REQ-AI-007.** MUST also cover retries, timeouts and cancellations (through `attempt` and the states of
`execution.*`).

**REQ-AI-008.** Telemetry MUST NOT grant permissions to run tools or to control devices.

**REQ-AI-012.** The library reports usage as token counts only. It never calculates a cost: the receiver does.

#### 9.4 AI adapters (v1)

**REQ-AI-009.** v1 MUST include:

| Adapter | Location | Function |
|---|---|---|
| Generic callbacks | `outbound/ai_runtime/` | Common API for own agents and local inference engines. |
| LangChain | `outbound/ai_runtime/langchain/` | Instrumentation of the framework. Python only (REQ-AI-011). |
| LlamaIndex | `outbound/ai_runtime/llamaindex/` | Instrumentation of the framework. Python only (REQ-AI-011). |
| Hosted model providers | `outbound/ai_runtime/` | Capture of usage, duration and normalized errors. |
| Langfuse | `outbound/ai_telemetry/langfuse/` | AI telemetry destination (the existing Python agent already uses it). |

**REQ-AI-010.** The Langfuse adapter MUST exist in both languages with the same observable behavior. In Python it uses
the Langfuse SDK and in Go its HTTP/OTLP API (D-09). Support in Go is checked when the AI work starts and the result is
written as an ADR.

**REQ-AI-011.** An adapter for a framework that exists in only one language (LangChain and LlamaIndex, Python only) MUST
be marked "not applicable" in the other language in the capabilities registry, and that does not block REQ-CNF-007.

---

### 10. Delivery, buffering and failures

#### 10.1 Delivery modes

Each destination is configured with **one delivery mode**:

| Mode | Behavior | What it guarantees | What it does NOT guarantee |
|---|---|---|---|
| `best_effort` | Bounded in-memory queue, one send attempt (or the configured retries). | That the caller does not block. | Delivery if the process or the destination goes down. |
| `retry` | Bounded in-memory queue with retries and backoff up to a limit. | Retry on transient failures. | Delivery after a process restart. |
| `durable` | Persistent local spool, sending with retries and resending after reconnection. | **At-least-once** as long as the spool is not full or corrupted. | Exactly-once. Duplicates may arrive. |

**REQ-DLV-001.** The library MUST NOT claim exactly-once. In `durable` mode, a timeout after the server processed the
request can produce a resend. `event_id` lets the receiver deduplicate, but deduplication is the receiver's
responsibility.

**REQ-DLV-002.** Hot-path emission operations (log, metric, event) MUST NOT block the caller waiting for network or
disk. After enqueueing, they MUST return immediately. There is no synchronous send mode: the consumer calls `shutdown`,
which flushes, before the process exits (REQ-DLV-010). In Python the API is synchronous and the library runs its own
worker thread; it also works inside an `asyncio` program.

**REQ-DLV-003.** The library MUST NOT report successful delivery before the destination confirms it.

**REQ-DLV-004.** Destinations MUST have timeouts. No network operation MUST wait indefinitely.

**REQ-DLV-005.** A failure of one destination MUST NOT interrupt the other destinations or the consuming process.

#### 10.2 Queues and backpressure

**REQ-DLV-006.** Every queue MUST be bounded. The size, the batch size and the flush interval MUST be configurable.
Defaults: a queue of 10,000 events, a batch of 100 events, a flush every 2 s and a network timeout of 3 s.

**REQ-DLV-007.** When a queue fills, the policy is configurable per destination: `drop_newest`, `drop_oldest` or
`block_with_timeout`. The default MUST be `drop_newest`. `block_with_timeout` MUST have a mandatory timeout and MUST
NOT be the default.

**REQ-DLV-008.** Every drop MUST be counted per destination and reason (section 12).

#### 10.3 Priorities

Three classes are defined. Each event is assigned to a class by its `event_type` and the assignment is configurable.

| Class | Examples | Default policy when the queue is full | Default policy when the spool is full |
|---|---|---|---|
| `critical` | Serious failures, faults, state changes | Drops those of lower classes first and takes their place. | Replaces the oldest of lower classes; if there are none, drops the oldest `critical` and counts it. |
| `normal` | Executions, reports, logs at INFO or above | `drop_newest`. | Rejects the new event and counts it. |
| `bulk` | DEBUG, TRACE, high-frequency readings | `drop_newest`. | Rejects the new event and counts it. |

**REQ-DLV-009.** Priority MUST NOT imply a delivery guarantee.

#### 10.4 Local spool (durable mode)

**REQ-SPL-001.** The spool MUST have a configurable disk quota (default 256 MiB per destination) and a documented policy
for the full state (previous table). It lives under a directory the consumer gives; the library never picks a path by
itself. The spool is kept until the events are delivered or the project cleans it, and the library MUST offer a public
cleanup function.

**REQ-SPL-002.** On corrupted data, the library MUST isolate the damaged records, continue with the valid ones and
count the discarded ones. It MUST NOT fail entirely because of one corrupted record.

**REQ-SPL-003.** If storage is unavailable at start, the library MUST degrade to `retry` mode for that destination,
record the change in diagnostics and continue.

**REQ-SPL-004.** The spool is a SQLite file (D-01). Go uses a pure-Go driver, without cgo, and Python the standard
`sqlite3`. Go and Python MUST be able to read the same spool.

#### 10.5 Shutdown

**REQ-DLV-010.** The library MUST offer `flush` and `shutdown` with a timeout. When they finish, they MUST return a
result that states how many events were sent, left in the spool or were dropped.

#### 10.6 Failure matrix

| Failure | Required behavior |
|---|---|
| Network down / timeout | According to the delivery mode of the destination; without blocking the caller. |
| HTTP 429 | Retry honoring `Retry-After` if present, or backoff. |
| HTTP 5xx | Retry with backoff and jitter. |
| HTTP 4xx (except 408 and 429) | Do not retry. Count as a permanent rejection and drop or set the batch aside. |
| Invalid response | Classify as a transient failure and retry with a limit. |
| Invalid configuration | Fail at start with a clear error (REQ-CFG-002). |
| Full queue | Policy of section 10.2. |
| Full disk quota | Policy of section 10.3. |
| Corrupted spool | REQ-SPL-002. |
| Serialization failure | Drop the event, count the failure, do not propagate to the hot path. |
| Invalid measurement | Reject at the source with a classified error. |
| Failure at shutdown | Result of REQ-DLV-010; it does not throw. |

---

### 11. Security and privacy

The library is passive: it offers mechanisms and the consumer defines the policy.

**REQ-SEC-008.** The security scope of v1 is minimal: redaction of secrets by field name and by pattern, size limits
and TLS. More advanced validations (for example detection of secrets in every variable) will be added later.

**REQ-SEC-001.** MUST offer configurable redaction (by field name and by pattern) applicable to all capabilities
before any output.

**REQ-SEC-002.** There MUST be a default redaction list (for example `password`, `token`, `authorization`, `api_key`,
`secret`), active if the consumer does not replace it. It is added to the detectors of ADR-006 (keys, tokens, JWTs,
private keys, passwords, credential pairs); nothing is removed.

**REQ-SEC-003.** MUST support optional data classification per field and rules per class (drop, redact, allow).

**REQ-SEC-004.** MUST apply configurable limits on the size of the payload, of the event and of the attributes.
Defaults: one event up to 64 KiB and one HTTP batch up to 1 MiB.

**REQ-SEC-005.** TLS verification MUST be on by default and turning it off MUST require explicit configuration.

**REQ-SEC-006.** Audit events (`audit.*`) MUST be a type separate from diagnostic logs and MAY be routed to their own
destination. The library MUST document that HTTP or file do not guarantee immutability or legal retention.

**REQ-SEC-007.** Configuration secrets (credentials, tokens) are supplied by the consumer in the configuration it passes
in, from its own environment or provider; the library never reads them by itself (REQ-ARC-007). They MUST NOT appear in
logs or diagnostics.

---

### 12. Self-observability

**REQ-DGN-001.** The library MUST expose, per destination: queue depth, dropped events (by reason), export failures,
retries, export latency, batch size, spool usage and quota, last successful send, configuration errors, serialization
failures and the result of the last flush.

**REQ-DGN-002.** Diagnostics MUST be available as a snapshot and through optional callbacks.

**REQ-DGN-003.** The library MUST avoid loops: telemetry failures MUST NOT generate events that can in turn fail and
generate more events. Diagnostics use their own internal channel with a rate limit.

---

### 13. Parity between languages and conformance

#### 13.1 General rule

Everything common is **identical** in Go and Python. Data from an agent in Go and from sensor software in Python are
mixed in one interface, so any difference, however small, produces display errors.

**Identical in both languages:**

- the envelope and all the event schemas;
- the enums, the event types and the error codes;
- the configuration model: option names, the env tags of the configuration (the consumer loads them), defaults, profiles and validation errors;
- the execution transitions;
- the layer structure, the package names and the adapter names (REQ-ARC-009..011);
- the spool format;
- the observable behavior of delivery and retries.

**May vary** only what the language forces: syntax, the naming conventions of section 3.1, the third-party libraries
each adapter uses and the exact instants of the retry jitter.

#### 13.2 Rules

**REQ-CNF-001.** The `contracts/` directory MUST be the single source of truth. Nobody writes by hand copies of enums,
event types, error codes or configuration schemas.

**REQ-CNF-002.** Enums, event types, error codes and the configuration schema MUST be generated into code in each
language from `contracts/`. The generator is a small Go program in this repository and the generated code is committed
(REQ-DST-005). CI MUST fail if the generated code does not match the contract.

**REQ-CNF-003.** JSON serialization MUST be byte-for-byte identical between languages for the same logical event, using
the JSON Canonicalization Scheme (RFC 8785). `occurred_at` and `recorded_at` MUST use the fixed format
`YYYY-MM-DDTHH:MM:SS.ffffffZ` (UTC, six decimals).

**REQ-CNF-004.** The validation and configuration error codes MUST be identical. The message text MAY vary.

**REQ-CNF-005.** There MUST be a set of shared fixtures (`integration-tests/fixtures/`) that both implementations run
with the same expected results.

**REQ-CNF-006.** The cross Go<->Python tests MUST cover at least byte-for-byte serialization, context propagation,
reading the same spool and interpreting the same configuration.

**REQ-CNF-007.** A capability is marked as supported in the registry only when both implementations pass its
conformance tests.

**REQ-CNF-008.** The overall maturity of the library MUST be that of the least complete capability. Until every
capability is stable the library is a beta (`0.x`); `1.0` is released only when every capability works (decision of the
maintainer, Q09-6).

#### 13.3 Maturity levels

Each capability in the registry has a level: `stable` or `beta`. All the capabilities of this document are part of the
scope of v1 and are built one at a time; the next one starts when the previous one is finished. The build order is: the
core and the delivery (envelope, logs, spool, HTTP), then executions and faults, then devices, then AI, then metrics and
traces, then audit.

---

### 14. Configuration

**REQ-CFG-001.** The configuration MUST have a single logical model shared by both languages.

**REQ-CFG-002.** It MUST be validated at start. An invalid configuration MUST fail with an error that states the field
and the reason.

**REQ-CFG-003.** Precedence, from lowest to highest: the defaults, the profile (a function that returns a configuration)
and the options given in code. The library reads no file and no environment (REQ-ARC-007); a consumer that wants them
loads them itself and passes the values in.

**REQ-CFG-004.** There MUST be base profiles (`microservice`, `cronjob`, `worker`, `robotics`, `offline_device`) that
are only defaults and can be overridden. Their delivery mode is `best_effort` for `microservice`, `retry` for `cronjob`
and `worker`, and `durable` for `robotics` and `offline_device`.

**REQ-CFG-005.** No versioned file MUST contain real credentials.

---

### 15. Versioning and compatibility

**REQ-VER-001.** The library and the contract follow semantic versioning and are versioned separately: the version of
the Go core module (tag `go/vX.Y.Z`), of every Go adapter module (tag `go/pkg/<functionality>/<adapter>/vX.Y.Z`), of the
Python package (tag `python-vX.Y.Z`) and `schema_version`. `schema_version` starts at `0.1` and reaches `1.0` with the
library.

**REQ-VER-002.** Each package MUST declare which contract versions it supports.

**REQ-VER-003.** A compatible change of the contract (new optional fields) increments MINOR. Removing or changing the
meaning of a field increments MAJOR.

**REQ-VER-004.** A consumer MUST ignore unknown fields from later MINOR versions.

**REQ-VER-005.** Changes to the public API, the configuration or the delivery behavior that break compatibility MUST be
documented as incompatible changes.

---

### 16. Decisions

The nine decisions that were pending in the first draft are closed; they are in "Decisions taken" at the top of this proposal (D-01 to D-09). What is still to decide is in "Still to decide".

---

### 17. Definition of done (per capability)

A capability is complete when:

1. its contract and its schemas are documented;
2. its public API exists in Go and Python;
3. the domain and application logic has tests;
4. the adapters have tests, including the failure cases of section 10.6;
5. its configuration is validated;
6. its security requirements (section 11) are covered;
7. it passes the cross-conformance tests (section 13);
8. it has documentation and usage examples;
9. its status appears in the capabilities registry.

---

### Annex A. Outside this specification

This document defines only the **contract** of the library. The following are out of scope and are handled separately:

- the task plan and the implementation phases (they are in [docs/tasks/](../tasks/README.md));
- the CI and publishing configuration;
- the instructions for code assistants.

---

### Annex B. Example device events (illustrative, to be edited with the real projects)

These examples were written on 2026-10-27 (and updated on 2026-10-28 for the two times) from the envelope of 5.1 and the device contracts of 8.2, **before** any real
project was looked at. They are **not normative**: the first real hardware project (a Raspberry Pi with sensors and
actuators, in Python) will correct the fields, the `kind` values and the units. Names, ids and values are placeholders.

Rules they follow:

- `attributes` holds only scalars and lists of scalars (REQ-CTR-010), so the details of a source are **flat keys**
  (`source_type`, `i2c_bus`, `i2c_address`), never a nested object.
- Every source produces the same `data` fields; only the source keys in `attributes` change (REQ-DEV-011).
- `event_id` values are UUIDv7-looking placeholders. `occurred_at` and `recorded_at` are both UTC in the fixed format of REQ-CNF-003; when the data carries `observed_at`, `occurred_at` equals it (REQ-CTR-008).
- The `kind` values `digital_input` and `position` are proposed additions to `contracts/enums/device/` (REQ-DEV-005).

**B.1 Measurement from an I2C sensor** (`device.measurement.recorded`)

```json
{
  "schema_version": "0.1",
  "event_id": "0194f0a2-7c1e-7b3a-9d10-3f5a6c2b8e01",
  "event_type": "device.measurement.recorded",
  "occurred_at": "2026-10-27T09:15:02.118000Z",
  "recorded_at": "2026-10-27T09:15:02.120000Z",
  "severity": "INFO",
  "resource": { "service_name": "sensor-hub", "instance_id": "hub-01" },
  "actor": { "kind": "device", "id": "temp-01", "type": "temperature_sensor" },
  "context": { "execution_id": "run-0001" },
  "data": {
    "name": "room_temperature",
    "value": 21.4,
    "unit": "celsius",
    "quality": "good",
    "observed_at": "2026-10-27T09:15:02.118000Z",
    "sensor_id": "temp-01",
    "kind": "temperature"
  },
  "attributes": { "source_type": "i2c", "i2c_bus": 1, "i2c_address": "0x48", "i2c_register": "0x00" }
}
```

**B.2 Measurement from a GPIO input** (`device.measurement.recorded`): same `data`, different source keys.

```json
{
  "schema_version": "0.1",
  "event_id": "0194f0a2-7c1e-7b3a-9d10-3f5a6c2b8e02",
  "event_type": "device.measurement.recorded",
  "occurred_at": "2026-10-27T09:15:02.169000Z",
  "recorded_at": "2026-10-27T09:15:02.170000Z",
  "severity": "INFO",
  "resource": { "service_name": "sensor-hub", "instance_id": "hub-01" },
  "actor": { "kind": "device", "id": "door-01", "type": "contact_sensor" },
  "data": {
    "name": "door_closed",
    "value": true,
    "unit": "bool",
    "quality": "good",
    "observed_at": "2026-10-27T09:15:02.169000Z",
    "sensor_id": "door-01",
    "kind": "digital_input"
  },
  "attributes": { "source_type": "gpio", "gpio_pin": 17, "gpio_level": "high", "gpio_pull": "up" }
}
```

**B.3 State observed** (`device.state.observed`)

```json
{
  "schema_version": "0.1",
  "event_id": "0194f0a2-7c1e-7b3a-9d10-3f5a6c2b8e03",
  "event_type": "device.state.observed",
  "occurred_at": "2026-10-27T09:15:09.950000Z",
  "recorded_at": "2026-10-27T09:15:10.000000Z",
  "severity": "INFO",
  "resource": { "service_name": "sensor-hub", "instance_id": "hub-01" },
  "actor": { "kind": "device", "id": "pump-01", "type": "pump" },
  "data": {
    "component": "pump",
    "state": "running",
    "previous_state": "idle",
    "observed_at": "2026-10-27T09:15:09.950000Z",
    "source": "gpio"
  }
}
```

**B.4 Actuator changed** (`device.actuator.changed`)

```json
{
  "schema_version": "0.1",
  "event_id": "0194f0a2-7c1e-7b3a-9d10-3f5a6c2b8e04",
  "event_type": "device.actuator.changed",
  "occurred_at": "2026-10-27T09:15:12.395000Z",
  "recorded_at": "2026-10-27T09:15:12.400000Z",
  "severity": "INFO",
  "resource": { "service_name": "sensor-hub", "instance_id": "hub-01" },
  "actor": { "kind": "device", "id": "servo-01", "type": "servo" },
  "data": {
    "actuator_id": "servo-01",
    "property": "angle",
    "previous_value": 0,
    "new_value": 90,
    "cause_command_id": "cmd-0042",
    "observed_at": "2026-10-27T09:15:12.395000Z"
  },
  "attributes": { "source_type": "gpio", "gpio_pin": 18 }
}
```

**B.5 Command requested and completed** (`device.command.requested`, `device.command.completed`): the request, the
result according to the consumer's driver code, and the observed state (B.4) are three different events (REQ-DEV-006).

```json
{
  "schema_version": "0.1",
  "event_id": "0194f0a2-7c1e-7b3a-9d10-3f5a6c2b8e05",
  "event_type": "device.command.requested",
  "occurred_at": "2026-10-27T09:15:12.100000Z",
  "recorded_at": "2026-10-27T09:15:12.105000Z",
  "severity": "INFO",
  "resource": { "service_name": "sensor-hub", "instance_id": "hub-01" },
  "actor": { "kind": "device", "id": "servo-01", "type": "servo" },
  "data": {
    "command_id": "cmd-0042",
    "target": "servo-01",
    "operation": "set_angle",
    "requested_state": 90,
    "requested_at": "2026-10-27T09:15:12.100000Z",
    "deadline": "2026-10-27T09:15:13.100000Z"
  }
}
```

```json
{
  "schema_version": "0.1",
  "event_id": "0194f0a2-7c1e-7b3a-9d10-3f5a6c2b8e06",
  "event_type": "device.command.completed",
  "occurred_at": "2026-10-27T09:15:12.400000Z",
  "recorded_at": "2026-10-27T09:15:12.410000Z",
  "severity": "INFO",
  "resource": { "service_name": "sensor-hub", "instance_id": "hub-01" },
  "actor": { "kind": "device", "id": "servo-01", "type": "servo" },
  "data": {
    "command_id": "cmd-0042",
    "execution_status": "succeeded",
    "started_at": "2026-10-27T09:15:12.200000Z",
    "completed_at": "2026-10-27T09:15:12.400000Z",
    "observed_state": 90
  }
}
```

**B.6 Connectivity changed** (`device.connectivity.changed`)

```json
{
  "schema_version": "0.1",
  "event_id": "0194f0a2-7c1e-7b3a-9d10-3f5a6c2b8e07",
  "event_type": "device.connectivity.changed",
  "occurred_at": "2026-10-27T09:19:59.800000Z",
  "recorded_at": "2026-10-27T09:20:00.000000Z",
  "severity": "WARN",
  "resource": { "service_name": "sensor-hub", "instance_id": "hub-01" },
  "actor": { "kind": "device", "id": "hub-01", "type": "raspberry_pi" },
  "data": { "link": "wifi", "status": "down", "previous_status": "up", "observed_at": "2026-10-27T09:19:59.800000Z" }
}
```

**B.7 Health reported** (`device.health.reported`): health per subsystem, not one boolean.

```json
{
  "schema_version": "0.1",
  "event_id": "0194f0a2-7c1e-7b3a-9d10-3f5a6c2b8e08",
  "event_type": "device.health.reported",
  "occurred_at": "2026-10-27T09:29:59.900000Z",
  "recorded_at": "2026-10-27T09:30:00.000000Z",
  "severity": "INFO",
  "resource": { "service_name": "sensor-hub", "instance_id": "hub-01" },
  "actor": { "kind": "device", "id": "hub-01", "type": "raspberry_pi" },
  "data": {
    "subsystems": [
      { "component": "i2c_bus_1", "status": "ok", "detail": "" },
      { "component": "servo_driver", "status": "degraded", "detail": "one retry in the last minute" }
    ],
    "observed_at": "2026-10-27T09:29:59.900000Z"
  }
}
```

**B.8 A fast sensor.** A microcontroller sensor that samples every 50 ms produces 20 readings per second. Each reading
is an event like B.1, and the library sends them in batches with configurable sampling (REQ-DEV-009), to an events
destination and never to metrics (REQ-MET-004).
