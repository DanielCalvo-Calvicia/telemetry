# PR-001 — The new telemetry standard: one contract for Go and Python, devices and AI agents

| | |
|---|---|
| Status | **Proposed, draft v0.1 (2026-10-27).** Not accepted by the maintainer. Nothing in it is built: the library today is the Go logging library described in [README.md](../../README.md). |
| Layers | all (domain, application, infrastructure, composition root), plus a new `contracts/` directory and a Python implementation |
| Languages | Go (>= 1.25) and Python (>= 3.10) |
| Related | [ADR-001](../decisions/ADR-001-adopt-shared-go-standard.md), [ADR-002](../decisions/ADR-002-one-module-capabilities-side-by-side.md), [ADR-003](../decisions/ADR-003-public-surface-is-pkg-logging.md), [ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md), [ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md), [ADR-006](../decisions/ADR-006-masking-before-every-sink.md), [architecture](../architecture/architecture.md), [requirements](../requirements/requirements.md) |
| Replaces | nothing yet. If accepted, it would be recorded in a new ADR and would update the ADRs it changes with dated notes. |

## Why

Today the library only logs: console, HTTP and AWS HTTP sinks, masking and a call context (`pkg/logging`). Projects that run on hardware (sensors, actuators, robots) and projects that run AI agents need to record much more than logs (metrics, traces, executions, health, device readings, model calls) and they are written in Go **and** in Python. If each project builds its own recording and delivery code, the data of an agent in Go and of sensor software in Python cannot be shown in one interface.

This proposal defines a single, versioned, language-independent **contract** (`contracts/`) and two implementations of the same architecture, so that everything common is identical in Go and Python.

## What the proposal contains

| Topic | Where |
|---|---|
| A passive library that centralizes logs, metrics, traces, executions, health, device data, AI agent data and generic reports | sections 1 and 6 |
| Architecture: Clean + Hexagonal, identical layers, names and adapters in both languages | section 3 |
| Distribution: Go core module plus optional submodules; Python package with extras | section 4 |
| The contract: event envelope, identity, severity, time and identifiers, attributes | section 5 |
| Outbound adapters and the HTTP default destination | section 7 |
| Device and hardware module (`device.*`) | section 8 |
| AI agent module (`ai.*`) | section 9 |
| Delivery modes, queues, priorities, spool, shutdown and the failure matrix | section 10 |
| Security and privacy, self-observability | sections 11 and 12 |
| Parity between languages and conformance tests | section 13 |
| Configuration, versioning, definition of done | sections 14, 15 and 17 |

## How it relates to what exists today

The proposal is the **target**. It does not change the current library until it is accepted and broken into tasks. The points where it differs from the present code and its ADRs are the ones to settle first:

| Today (built) | In this proposal | To decide |
|---|---|---|
| One Go module, public surface `pkg/logging` by aliases (ADR-002, ADR-003) | Go core module plus independent optional submodules (REQ-DST-001) | how `pkg/logging` is kept for current consumers |
| The library never reads the environment (ADR-004) | no reads outside the configuration layer (REQ-ARC-007), a configuration file and profiles (REQ-CFG-003/004) | whether the file and the profiles are loaded by the consumer or by the library |
| A failing sink never fails the caller; 3 s timeout, no retry (ADR-005) | delivery modes `best_effort`, `retry` and `durable`, bounded queues, retries with backoff (section 10) | the migration of the HTTP sinks to the shared HTTP infrastructure (REQ-OUT-006) |
| Masking before every sink (ADR-006) | redaction by field name and by pattern, data classification, size limits (section 11) | which detectors of ADR-006 become the default redaction list (REQ-SEC-002) |
| Entry with fixed fields, `Category`, `Function`, `Message` | versioned event envelope with JSON schemas (section 5) | the mapping of `logging.Entry` to a `log.*` event |
| No metrics (planned, `internal/metrics`, `pkg/metrics`) | metrics with cardinality limits (6.2) | whether the planned metrics package follows this section |

## Decisions to take

The pending decisions D-01 to D-09 are in section 16 of the specification below. Besides them, accepting this proposal needs:

1. confirmation that the repository grows to hold the `contracts/` directory and a `python/` implementation (or that they live in other repositories);
2. the split into tasks and phases, which is **outside** this document (Annex A);
3. a new ADR that records the acceptance, and dated notes in the ADRs listed in "Related".

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

**REQ-ARC-003.** The wiring of dependencies MUST happen only in the composition root, with constructor injection.

**REQ-ARC-004.** Abstractions MUST represent capabilities of the platform, not replicate methods of third-party
libraries.

**REQ-ARC-005.** The core MUST have a minimal set of external dependencies. Every adapter with its own dependencies
(OpenTelemetry, Prometheus, SQL drivers, hardware, AI frameworks) MUST be installed separately from the core
(section 4). The adapter code is part of the library.

**REQ-ARC-006.** The library MUST NOT use mutable global state. If a convenience global instance is offered, it MUST be
optional and documented.

**REQ-ARC-007.** The library MUST NOT read environment variables outside the configuration layer.

#### 3.1 Identical internal structure in both languages

Go and Python are two implementations of the **same** architecture. The internal structure is normative and must be
practically a copy.

**REQ-ARC-008.** Both implementations MUST have architecture tests that verify the direction of dependencies
(REQ-ARC-001/002/003).

**REQ-ARC-009.** Go and Python MUST have the same layers, the same directory and package names, and the same file
names. Only the differences the language forces are allowed (`__init__.py`, the `_test.go` suffix, `pkg/` versus
`src/`, the file extension).

**REQ-ARC-010.** Public names (types, functions, methods, options) MUST be equivalent through a mechanical rule
documented in `contracts/conventions/naming.md`: Go uses exported identifiers in `PascalCase`; Python uses
`snake_case` for functions and `PascalCase` for classes. A concept MUST NOT be renamed in one language.

**REQ-ARC-011.** Each adapter MUST be a subdirectory with the same name in both languages (for example
`outbound/ai_telemetry/langfuse/`, `outbound/device_control/gpio/`).

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
    device_control/
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

**REQ-DST-001.** Go MUST be distributed as a core module plus independent optional submodules, so that importing the
core does not pull in adapter dependencies.

**REQ-DST-002.** Python MUST be distributed as one package with optional *extras* (for example `telemetry[otel]`,
`telemetry[sql]`, `telemetry[prometheus]`, `telemetry[device]`, `telemetry[ai]`).

**REQ-DST-003.** The library MUST support Go >= 1.25 and Python >= 3.10. Changing these minimums MUST be treated as a
compatibility change (section 15).

**REQ-DST-004.** The device and AI modules are part of the library and of every release. The third-party dependencies
of their adapters (Langfuse SDK, hardware libraries, AI frameworks) MUST be installed separately (a submodule in Go,
an extra in Python), so that a project installs only the ones it uses.

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
| `timestamp` | yes | UTC instant, RFC 3339 with the `Z` zone. |
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

**REQ-CTR-008.** Event timestamps MUST be UTC. Durations MUST be measured with a monotonic clock where the runtime
supports it, without mixing them with wall-clock time.

**REQ-CTR-009.** `event_id` MUST be globally unique with practical probability (UUIDv7 or another documented format).
It MUST NOT be derived from a timestamp alone.

#### 5.5 Extension attributes

**REQ-CTR-010.** `attributes` MUST accept only scalar values and lists of scalars, with configurable limits on the
number of keys, key length and value length. Excesses are truncated or dropped according to a documented policy and
counted in diagnostics.

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
`LocalSpoolPort`, `ClockPort`, `IDGeneratorPort`, `ConfigurationPort`.

**REQ-OUT-001.** Ports with identical semantics MAY share a generic abstraction. Payloads that are semantically
different MUST NOT be forced into an untyped interface.

#### 7.2 Planned adapters

| Adapter | Logs | Events/reports | Metrics | Traces | Notes |
|---|:-:|:-:|:-:|:-:|---|
| Console | yes | yes | - | - | Core. |
| Local file (JSON Lines) | yes | yes | - | - | Core. Configurable rotation and quota. |
| HTTP (own schema) | yes | yes | - | - | Core. **Default destination.** |
| OpenTelemetry (OTLP) | yes | - | yes | yes | Optional. |
| Prometheus | - | - | yes | - | Optional. |
| SQL | - | yes | - | - | Optional. Concrete dialects to be decided (section 16). |
| Langfuse | - | yes (AI) | - | yes (AI) | Included in v1 (REQ-AI-009). |

**REQ-OUT-002.** The default destination of all events MUST be HTTP. Any other destination is enabled by explicit
configuration.

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

**REQ-OUT-007.** Adding a new adapter MUST require only implementing the corresponding port, without touching the
domain or the application.

---

### 8. Devices and hardware

#### 8.1 Scope

This section covers sensors, actuators, robots and any other hardware that uses the library. It is **independent** of
section 9 (AI agents): the two domains have their own events, schemas, enums and ports.

The device module is part of the library from v1 and of every release. Third-party hardware libraries are installed
separately (REQ-DST-004).

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
| `device.command.completed` | Result of executing the command according to the driver. | `command_id`, `execution_status`, `started_at`, `completed_at`, `error`, `observed_state` |
| `device.connectivity.changed` | Change in the connectivity of the device. | `link`, `status`, `previous_status`, `observed_at` |
| `device.health.reported` | Health per subsystem (not a single boolean). | `subsystems` (list of `component`, `status`, `detail`), `observed_at` |

**REQ-DEV-005.** The allowed values of `kind` (for example `temperature`, `voltage`, `gnss`, `imu`, `rpm`), of
`quality` (`good`, `uncertain`, `bad`, `stale`), of `execution_status` (`succeeded`, `failed`, `timed_out`,
`cancelled`, `rejected`) and of `status` MUST be in `contracts/enums/device/`. Adding a value is a MINOR change of the
contract (REQ-VER-003).

#### 8.3 Semantic rules

**REQ-DEV-006.** Device reports MUST distinguish four separate things:

1. the command requested (`device.command.requested`);
2. the result of executing the command according to the driver (`device.command.completed`);
3. the observed physical state (`device.state.observed`, `device.actuator.changed`, `device.measurement.recorded`);
4. the delivery result of the telemetry (diagnostics, section 12).

A successful command does not prove that the physical state was reached.

**REQ-DEV-007.** Measurements MUST include value, unit, timestamp, quality and source. The library MUST NOT require a
common set of quantities from all devices.

**REQ-DEV-008.** A sensor reading and an actuator change MUST be distinct events (`device.measurement.recorded` versus
`device.actuator.changed` and `device.command.*`).

**REQ-DEV-009.** For high-frequency readings, batch sending and configurable sampling MUST be offered. The data MUST
go to an events or SQL destination, not to metrics (REQ-MET-004).

**REQ-DEV-010.** A command MUST support `deadline`, and adapters MUST honor it. A command MUST have a unique
`command_id` to correlate the request, the result and the observed state.

#### 8.4 Device adapters (v1)

**REQ-DEV-011.** v1 MUST include adapters in `outbound/device_control/` and `outbound/device_telemetry/` for:

| Adapter | Location |
|---|---|
| GPIO | `device_control/gpio/` |
| I2C | `device_control/communication/i2c/` |
| SPI | `device_control/communication/spi/` |
| Serial communication | `device_control/communication/serial/` |
| Camera | `device_control/camera/` |
| Motor controller | `device_control/motor/` |
| Sensors | `device_control/sensor/` |
| Vendor SDK (generic interface) | `device_control/vendor_sdk/` |

Each adapter MUST translate the generic operations to the hardware library of the language, normalize its errors and
exist in Go and Python with the same name and observable behavior.

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

#### 9.4 AI adapters (v1)

**REQ-AI-009.** v1 MUST include:

| Adapter | Location | Function |
|---|---|---|
| Generic callbacks | `outbound/ai_runtime/` | Common API for own agents and local inference engines. |
| LangChain | `outbound/ai_runtime/langchain/` | Instrumentation of the framework. |
| LlamaIndex | `outbound/ai_runtime/llamaindex/` | Instrumentation of the framework. |
| Hosted model providers | `outbound/ai_runtime/` | Capture of usage, duration and normalized errors. |
| Langfuse | `outbound/ai_telemetry/langfuse/` | AI telemetry destination (the existing Python agent already uses it). |

**REQ-AI-010.** The Langfuse adapter MUST exist in both languages with the same observable behavior. The integration
route in each language (Langfuse SDK or HTTP/OTLP API) is decided in D-09.

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
disk. After enqueueing, they MUST return immediately.

**REQ-DLV-003.** The library MUST NOT report successful delivery before the destination confirms it.

**REQ-DLV-004.** Destinations MUST have timeouts. No network operation MUST wait indefinitely.

**REQ-DLV-005.** A failure of one destination MUST NOT interrupt the other destinations or the consuming process.

#### 10.2 Queues and backpressure

**REQ-DLV-006.** Every queue MUST be bounded. The size, the batch size and the flush interval MUST be configurable.

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

**REQ-SPL-001.** The spool MUST have a configurable disk quota and a documented policy for the full state (previous
table).

**REQ-SPL-002.** On corrupted data, the library MUST isolate the damaged records, continue with the valid ones and
count the discarded ones. It MUST NOT fail entirely because of one corrupted record.

**REQ-SPL-003.** If storage is unavailable at start, the library MUST degrade to `retry` mode for that destination,
record the change in diagnostics and continue.

**REQ-SPL-004.** The spool format (SQLite or segmented files) is pending a decision (section 16). Go and Python MUST
be able to read the same spool.

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
`secret`), active if the consumer does not replace it.

**REQ-SEC-003.** MUST support optional data classification per field and rules per class (drop, redact, allow).

**REQ-SEC-004.** MUST apply configurable limits on the size of the payload, of the event and of the attributes.

**REQ-SEC-005.** TLS verification MUST be on by default and turning it off MUST require explicit configuration.

**REQ-SEC-006.** Audit events (`audit.*`) MUST be a type separate from diagnostic logs and MAY be routed to their own
destination. The library MUST document that HTTP or file do not guarantee immutability or legal retention.

**REQ-SEC-007.** Configuration secrets (credentials, tokens) MUST be injectable from the environment or a provider and
MUST NOT appear in logs or diagnostics.

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
- the configuration model: option names, environment variables, defaults, profiles and validation errors;
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
language from `contracts/`. CI MUST fail if the generated code does not match the contract.

**REQ-CNF-003.** JSON serialization MUST be byte-for-byte identical between languages for the same logical event,
using the JSON Canonicalization Scheme (RFC 8785). Timestamps MUST use the fixed format
`YYYY-MM-DDTHH:MM:SS.ffffffZ` (UTC, six decimals).

**REQ-CNF-004.** The validation and configuration error codes MUST be identical. The message text MAY vary.

**REQ-CNF-005.** There MUST be a set of shared fixtures (`integration-tests/fixtures/`) that both implementations run
with the same expected results.

**REQ-CNF-006.** The cross Go<->Python tests MUST cover at least byte-for-byte serialization, context propagation,
reading the same spool and interpreting the same configuration.

**REQ-CNF-007.** A capability is marked as supported in the registry only when both implementations pass its
conformance tests.

**REQ-CNF-008.** The overall maturity of the library MUST be that of the least complete capability.

#### 13.3 Maturity levels

Each capability in the registry has a level: `stable` or `beta`. All the capabilities of this document are part of the
scope of v1.

---

### 14. Configuration

**REQ-CFG-001.** The configuration MUST have a single logical model shared by both languages.

**REQ-CFG-002.** It MUST be validated at start. An invalid configuration MUST fail with an error that states the field
and the reason.

**REQ-CFG-003.** Precedence, from lowest to highest: defaults, profile, configuration file, environment variables,
options in code.

**REQ-CFG-004.** There MUST be base profiles (`microservice`, `cronjob`, `worker`, `robotics`, `offline_device`) that
are only defaults and can be overridden.

**REQ-CFG-005.** No versioned file MUST contain real credentials.

---

### 15. Versioning and compatibility

**REQ-VER-001.** The library and the contract follow semantic versioning and are versioned separately: version of the
Go module, of the Python package and `schema_version`.

**REQ-VER-002.** Each package MUST declare which contract versions it supports.

**REQ-VER-003.** A compatible change of the contract (new optional fields) increments MINOR. Removing or changing the
meaning of a field increments MAJOR.

**REQ-VER-004.** A consumer MUST ignore unknown fields from later MINOR versions.

**REQ-VER-005.** Changes to the public API, the configuration or the delivery behavior that break compatibility MUST be
documented as incompatible changes.

---

### 16. Pending decisions

| # | Decision | Options | Impact |
|---|---|---|---|
| D-01 | Spool format | SQLite, segmented files | REQ-SPL-004 |
| D-02 | Contract format | JSON Schema (proposed), protobuf | section 5 |
| D-03 | SQL dialects of the adapter | To be defined according to the consuming projects | section 7.2 |
| D-04 | Event identifier format | UUIDv7 (proposed) | REQ-CTR-009 |
| D-05 | Concrete hardware library of each driver (GPIO, I2C, SPI, serial, camera, motor) in each language | According to the real hardware of the projects | REQ-DEV-011 |
| D-06 | Confirm the initial list of AI frameworks | Langfuse, LangChain, LlamaIndex (proposed) | REQ-AI-009 |
| D-09 | How to integrate Langfuse in each language | SDK, or Langfuse HTTP/OTLP API (check support in Go) | REQ-AI-010 |
| D-07 | Language of the documentation | Spanish / English | Everything |
| D-08 | Exact minimum of Python | 3.10 (proposed) | REQ-DST-003 |

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

- the task plan and the implementation phases;
- the CI and publishing configuration;
- the instructions for code assistants.
