# Architecture

Status as of 2026-10-27. Everything described here exists in the repository unless it says **planned**.

## 1. Purpose and responsibility

`telemetry` is a public Go library (`github.com/DanielCalvo-Calvicia/telemetry`) that gives a Snow Factory service one logger with the same behaviour everywhere: structured entries, a minimum level that follows the environment, masking of sensitive data, and several sinks (console, a generic HTTP receiver, and the `add_log` endpoint of aws_microservice, which stores the entry in CloudWatch Logs).

It is a library, not a service: there is no `cmd/`, no image and no manifest ([ADR-010](../decisions/ADR-010-no-deployment-release-by-tags.md)). A consumer imports `pkg/logging`, builds a logger and logs.

The library does one thing: take an entry from the caller, make it safe, stamp it and hand it to every sink. It never decides where the logs go (the consumer does) and never reads the process environment ([ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md)).

## 2. Structure (layers exist)

One Go module, capabilities side by side under `internal/` ([ADR-002](../decisions/ADR-002-one-module-capabilities-side-by-side.md)). Logging is the only capability today; metrics is **planned** (it would be `internal/metrics` with the same three layers).

**Started (T-01.2, 2026-10-28):** beside this module there is a second Go module, `go/` (`github.com/DanielCalvo-Calvicia/telemetry/go`), for the new standard ([PR-001](../proposals/PR-001-telemetry-standard-go-python.md), [ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md)). It has only the empty layers `domain/`, `application/`, `infrastructure/` and `composition_root/` and the folders of its tests; nothing of the new library is built. `go.work` at the root joins the two modules, and the old library stays untouched until T-04.7 removes it. `integration-tests/` exists with a README and is empty (planned); `contracts/` has its layout (2.3, T-02.1). `python/` holds the skeleton of the Python package (T-01.3, below). Checked on Go 1.25.5: a nested adapter module whose path is under `go/pkg/` can import the `internal/` packages of the core, and a module with an unrelated path cannot (the result is recorded in ADR-013).

```text
telemetry/
├── go.mod, go.sum, go.work, LICENSE, SECURITY.md, README.md, CLAUDE.md, .env.example, .gitignore
├── go/                            # the new Go core: its own go.mod; internal/{domain,application,infrastructure,composition_root,tests} (empty layers)
├── contracts/                     # the contract: schemas/, enums/, events/, errors/, config/, capabilities/, conventions/, storage/ (layout only)
├── tools/contract_generator/      # the generator of the code of both languages from contracts/ (its own module)
├── python/, integration-tests/    # the new standard (python/ is the skeleton; integration-tests/ is planned, a README)
├── .github/workflows/ci.yml
├── docs/                          # this documentation
├── pkg/logging/                   # the ONLY public API: logging.go, sinks.go, example_test.go, sinks_test.go
├── examples/aws_http_smoke/       # a sample service that logs in production to the add_log endpoint (run by hand)
└── internal/
    ├── logging/
    │   ├── domain/                # entities, value_objects, operations (masking), rules, errors
    │   ├── application/           # call_depth, dtos, ports (inbound, outbound), services, errors
    │   └── infrastructure/
    │       ├── config/            # TelemetryConfig (the env tags; no loading)
    │       └── outbound/          # console, http_sink, aws_http, systemclock, uuidgen
    ├── composition_root/
    │   ├── containers/            # logging_container.go (the only place that wires the logger)
    │   └── dependencies/          # one file per outbound adapter
    └── tests/                     # mirrors the code tree; black-box (package x_test)
```

The dependency rule is the standard one: domain imports nothing from the other layers; application imports domain and defines the ports; infrastructure implements the ports; only `internal/composition_root` imports the concrete outbound adapters; consumers import only `pkg/logging` ([ADR-001](../decisions/ADR-001-adopt-shared-go-standard.md), [ADR-003](../decisions/ADR-003-public-surface-is-pkg-logging.md)).

### 2.1 The Python package (started, T-01.3)

`python/` is an installable package `telemetry` (Python 3.10 or newer; the Raspberry Pi of the pilot runs 3.11.4) with the same layers and folder names as the Go core. Only the skeleton exists: nothing of the new library is built.

```text
python/
├── pyproject.toml                     # setuptools; the extras sqlite, otel, prometheus, ai (empty for now); coverage and ruff settings
├── src/telemetry/                     # the package, py.typed
│   └── internal/{domain,application,infrastructure,composition_root}/
└── internal/tests/                    # unittest tests mirroring the code; the same folders as go/internal/tests
```

- **Install:** from the Git repository, not from a package index (`pip install "telemetry[sqlite] @ git+https://github.com/DanielCalvo-Calvicia/telemetry@python-vX.Y.Z#subdirectory=python"`), as [ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md) and Q07-3 decided. The core has no third-party dependency (REQ-ARC-005).
- **Rules until the Python standard of the organization can be read** (Q07-8): the layers and folder names of the Go core; PEP 8 names (`snake_case` for functions, variables, modules and arguments, `PascalCase` for classes); every public class and function has a docstring; tests are black-box and mirror the code tree.
- **Tools chosen on 2026-10-28 (the maintainer), with the reason:** build backend **setuptools** (already installed with Python tooling, supports `[project]` and extras, no new tool to learn); tests with **unittest** from the standard library plus **coverage.py** for the 95% gate (the fewest dependencies; the coverage tool is the only one); lint and format with **ruff** (one development-only tool). T-01.5 runs them in CI.

### 2.2 The architecture tests (built, T-01.4)

The direction of the dependencies is a test in both languages (REQ-ARC-008), so a wrong import fails before review. They run on the new core (`go/`, `python/`), not on the old library of the root module.

| Rule | What it forbids |
|---|---|
| `domain-isolated` | the domain importing the application, the infrastructure, the composition root, or an I/O, database, telemetry-backend or AI-framework package (REQ-ARC-001, REQ-ARC-002) |
| `application-imports-only-domain` | the application importing anything but the standard library, the domain and itself |
| `infrastructure-not-composition-root` | the infrastructure importing the composition root |
| `adapters-wired-only-by-the-composition-root` | any package outside a composition root importing a concrete outbound adapter (REQ-ARC-003) |
| `io-only-in-external` | a file of an adapter outside its `external/` folder importing an I/O package (network, files, processes, databases) |
| `consumer-never-imports-internal` | a consumer (`examples/`) importing an `internal/` package |

- **Where they live.** Go: `go/internal/tests/architecture/` (`dependency_direction_test.go`, `composition_root_test.go`, `internal_visibility_test.go`, `helpers_test.go`) over the rule engine in `go/internal/tests/architecture/checker/`, which reads the imports with `go/parser` of the standard library. Python: `python/internal/tests/architecture/` (`test_dependency_direction.py`, `test_composition_root.py`, `test_internal_visibility.py`, `helpers.py`) over `checker.py`, which reads the imports with the standard `ast` module. Neither adds a dependency.
- **Planted violations.** Each rule has a fixture tree that breaks it and a clean tree, in `go/internal/tests/fixtures/architecture/testdata/<case>/` (`testdata` is ignored by the Go tool, so the files are never compiled) and `python/internal/tests/fixtures/architecture/<case>/` (excluded from ruff). A test fails if the planted violation is not reported, and another if the clean tree is not accepted.
- **A failure names the file and the import:** `domain-isolated: internal/domain/zz_planted.go imports "net/http"`.
- **Not checked yet.** The adapter folders `outbound/<functionality>/<adapter>` do not exist, so the rules about adapters pass on the empty layers and are proved by the fixtures; they start to bite with T-04.

### 2.3 The contract folder (started, T-02.1)

`contracts/` is the single source of truth between the two languages (REQ-CNF-001). Its layout, the naming rule, the envelope schema, the first event type (`log.entry.recorded`), the registry of event types and the catalog of error codes exist (T-02.1 to T-02.3); the generator that turns them into code exists (2.4, T-02.4).

| Folder | What it will hold |
|---|---|
| `schemas/` (`device/`, `ai/`) | the JSON Schema (draft 2020-12) of the envelope (**written**: `envelope.schema.json` and `common/` for `resource`, `actor`, `context` and `attributes`) and, later, of the `data` of each event type |
| `enums/` (`device/`, `ai/`) | severity and the actor kinds (**written**), then the execution states and transitions and the allowed values of the device and AI domains |
| `events/` | the registry of event types (`registry.yaml`, **written** with `log.entry.recorded`) |
| `errors/` | the catalog of error codes, identical in both languages (`catalog.yaml`, **written**: the validation and configuration codes) |
| `config/` | the schema of the configuration: options, defaults, profiles |
| `capabilities/registry.yaml` | the registry of capabilities and their conformance tests |
| `conventions/naming.md` | the mechanical naming rule: Go `PascalCase` exported, Python `snake_case` functions and `PascalCase` classes, JSON `snake_case` |
| `storage/` | the layouts of the files a reader opens (the SQLite destination, the spool) |

A change is approved by the maintainer by pull request and a dated ADR note, and is implemented and released in both languages. The generated code arrives with T-02.4.

**How an event type is registered.** An event type `<domain>.<entity>.<action>` has a schema for its `data` at `schemas/<domain>/<entity>_<action>.schema.json` and an entry in `events/registry.yaml` (type, schema, capability, the `schema_version` since which it exists). Each type is added with its capability, one at a time. T-02.4 will fail when a registered type has no schema or a schema is not registered. Error codes are in `errors/catalog.yaml`: a stable lowercase `snake_case` code with a category (`enums/failure_category.json`) and a default message; each capability adds its own.

### 2.4 The contract generator (built, T-02.4)

`tools/contract_generator/` is a Go module of its own (decided by the maintainer on 2026-10-28), joined to the others by `go.work`. Its two third-party libraries (`gopkg.in/yaml.v3` and `github.com/santhosh-tekuri/jsonschema/v6`) live only there, so the core module keeps its minimal dependency set (REQ-ARC-005). It follows the layers of the standard: `internal/domain` (the contract model, the naming and the rules of the check), `internal/application` (the ports and the use case: read, check, render, write), `internal/infrastructure` (the command line, the reader of the contract folder, the Go and Python renderers, the file writer; the I/O is in `external/` folders) and `internal/composition_root`; `main.go` only calls it.

- **What it does.** It checks the contract (every file is valid, every registered event type has its schema and the other way round, the error codes and enum values are unique) and stops with an error that names the file; if the check passes it writes the enums, the event types, the error codes and the envelope types of both languages, and (T-02.6) the validators of the data of each event type (2.6). It is deterministic.
- **Where the output goes** (decided on 2026-10-28): the domain layer; the validators go to `domain/operations/event_validation/` of each language (four output flags in all). Go: `go/internal/domain/value_objects/{enums,event_types,error_codes,envelope}_generated.go` (package `value_objects`); Python: `python/src/telemetry/internal/domain/value_objects/{enums,event_types,error_codes,envelope}_generated.py`, with a hand-written `__init__.py`. The output is **committed**, so a Python install never needs Go, and ruff does not lint the Python files.
- **Regenerate:** `cd tools/contract_generator && go run . --contracts ../../contracts --go-out ../../go/internal/domain/value_objects --python-out ../../python/src/telemetry/internal/domain/value_objects --go-validation-out ../../go/internal/domain/operations/event_validation --python-validation-out ../../python/src/telemetry/internal/domain/operations/event_validation`. The CI job `generated-code` runs it and fails if anything changed or appeared (see the deployment document).
- **Names.** The words of a name are the same in both languages (`naming.md`): Go fields are `EventId`, `TraceId`, `InstanceId`, not the `ID` form of Go style, and an optional number, integer or boolean is a pointer in Go so `omitempty` does not drop a `false` or a `0`. Python dataclasses list the required fields first, then the optional ones.
- **Not generated yet.** The configuration schema (T-03.6). The generator understands only the subset of JSON Schema the contract uses (see its README).

### 2.5 Canonical JSON (built, T-02.5)

The serialization that makes the same event give the same bytes in Go and Python (REQ-CNF-003, REQ-CTR-004) is a pure operation of the domain layer in each language, with the standard library only (REQ-ARC-005): Go `go/internal/domain/operations/canonical_json/` (`CanonicalJSON`, `FormatFloat`, `FormatInt`, `FormatUTCTime`) and Python `python/src/telemetry/internal/domain/operations/canonical_json/` (`canonical_json`, `format_float`, `format_int`, `format_utc_time`). It follows the JSON Canonicalization Scheme (RFC 8785):

- keys are sorted by UTF-16 code units (so a character outside the BMP sorts by its surrogate pair), with no insignificant whitespace;
- strings are escaped as the RFC says: only `"`, `\` and the control characters; nothing else, not `/`, not non-ASCII; the output is UTF-8; a lone surrogate or invalid UTF-8 is rejected;
- numbers use the ECMAScript form (shortest round-trip digits, exponent form below `1e-6` and from `1e21`, `-0` prints `0`); NaN and the infinities are rejected;
- **an integer beyond ±2^53-1 is rejected** (`limit_exceeded`): the maintainer was not asked, this is the plan's default and it is one constant per language (`MaxExactInteger`, `MAX_EXACT_INTEGER`);
- `occurred_at` and `recorded_at` are formatted as `YYYY-MM-DDTHH:MM:SS.ffffffZ`: UTC only, exactly six decimals, truncation (not rounding) to microseconds; a non-UTC instant is rejected (`event_schema_invalid`), never converted.

The Go function walks the value with `reflect` and honours the `json` tags (`-`, `omitempty`), so an int and a float stay different; the Python function accepts a dict or the generated dataclass (an optional field that is `None` is omitted). The errors are small classified errors that carry a code of the catalog (`domain/errors.go`, `domain/errors.py`); the full error model is T-03.7. The shared fixtures and the comparison script are in `integration-tests/` and `integration-tests/fixtures/canonical_json/`.

### 2.6 Validation at the source (built, T-02.6)

An event whose `data` does not match the schema of its `event_type` is rejected before it goes anywhere (REQ-CTR-002). The validator is a pure operation of the domain layer, `go/internal/domain/operations/event_validation/` (`ValidateEvent(event_type, data)`) and `python/src/telemetry/internal/domain/operations/event_validation/` (`validate_event(event_type, data)`), with the standard library only.

- **No JSON Schema library in the core** (the task's default, option (b); the maintainer was not asked, REQ-ARC-005). The generator emits a thin validator per event type (`event_validators_generated.go/.py`) that calls a small set of hand-written rules (`rules.go`, `rules.py`); the registry is read at generation, never at run time.
- **The result is a value, never an exception.** It returns nothing when valid, otherwise a classified error: `event_type_unknown` if the type is not registered, `event_schema_invalid` if the data does not match. The message names the event type and the field path, for example `log.entry.recorded: data.message: required field is missing`, and **never contains the value** (a value may be a secret). A hostile field name is cut to 64 characters and control characters become `?`.
- **It never panics or raises, on any input.** The code is written without failure paths (type switches only, no deep recursion), and as a last resort `Safely` (Go, `recover`) and `run_safely` (Python, `except Exception`) turn anything unexpected into `event_schema_invalid` ("validation failed"). About 35 hostile inputs are tested: `nil`/`None`, a string, a number, a list, maps with non-string keys, a cyclic structure, 100,000 levels of nesting, huge integers, NaN, a channel, a function, a one-million-character key.
- **What it checks.** For the data schema of each registered type and, recursively, the common objects it references: the types `string`, `number`, `integer`, `boolean` and a free object; `required`; `additionalProperties: false`; a `$ref` to an enum, to a common object and to `attributes` (scalars and lists of scalars, with the limits of that schema); `minLength` and `maxLength`. The order is fixed so both languages report the same first problem: the missing required fields (alphabetical), then the declared fields (alphabetical), then the smallest unknown field.
- **What it refuses to generate.** Any other keyword in a schema that an event type reaches (`pattern`, `format`, an inline `enum`, `minimum`, ...) stops the generator, naming the file and the property, so a constraint is never silently ignored.
- **Not wired yet.** Nothing calls `ValidateEvent` yet: the building of an event (T-03.3) does, once per event, after masking is decided and before the queue. The rejection counter is planned.

### 2.7 The capabilities registry and its check (built, T-02.7)

`contracts/capabilities/registry.yaml` lists the capabilities of the standard and, for **every requirement of the specification**, either the conformance tests that prove it or the task that will write them (section 2 of PR-001, REQ-CNF-007, REQ-CNF-008). Its fields and states are in [contracts/capabilities/README.md](../../contracts/capabilities/README.md). `tools/registry_check/` is the check: a Go module of its own (the same layers as the generator, `gopkg.in/yaml.v3` its only library) run in CI by the job `registry-check`.

- **The source of the requirement ids is the specification** (PR-001, the bold `**REQ-XXX-nnn.**` definitions: 130 ids in 20 areas), through the flag `--spec`. The plan's default, the requirements document, lists only the requirements whose state moved, so it cannot be the source.
- **What it fails on:** a requirement of the specification that no capability lists; an id that is not in the specification; a requirement with neither tests nor a pending task; a listed test file that does not exist; a pending task that does not exist in `docs/tasks`; a capability marked `supported` while a language is not `passing` (or `not_applicable` without a reason) or while a requirement is only pending; an invalid level, status or state; a duplicated id (a requirement belongs to one capability only). A misspelled key fails too (strict YAML).
- **What it reports.** The uncovered requirements, one per line with their area and the task they wait for (`REQ-ARC-014 (ARC): pending T-03.9`), a separate list of those that **no task mentions yet** (`pending: unassigned`: allowed, never counted as covered), the counts, and the library maturity as the least complete capability (REQ-CNF-008). A pending item never counts as covered.
- **Today.** 17 capabilities, all `beta`, none `supported`; 130 requirements: **45 covered by real tests** (the architecture, canonical JSON, event validation, generator, shared-fixture, value-object, port, event-building, masking, limits, configuration, error-code, composition, logger, shared-HTTP, console and HTTP-destination requirements), **77 pending** on a task and **8 with no task yet**. Run it with `cd tools/registry_check && go run . --spec ../../docs/proposals/PR-001-telemetry-standard-go-python.md --registry ../../contracts/capabilities/registry.yaml --root ../.. --tasks ../../docs/tasks`.

### 2.8 The shared fixtures (built, T-02.8)

`integration-tests/fixtures/` holds the files that **both implementations read**; neither language owns them (REQ-CNF-005). `envelope/` and `events/` hold one case per file (a `kind` of `canonical`, `time` or `validation`, an `input`, and an `expected` text or an `error_code`), and a thin test in each language reads the folders, so adding a case needs no code change: Go `go/internal/tests/contract/fixtures_test.go`, Python `python/internal/tests/contract/test_fixtures.py`. A `canonical` case compares the **bytes** exactly, not parsed JSON; an unknown `kind` fails the test. `canonical_json/` and `event_validation/` hold the tables of the canonical JSON and of the validation at the source, moved here from `go/internal/tests/fixtures/`; the architecture fixtures (planted violations) stay with the Go tests. The expected texts of the first cases were computed with Node.js, not with the implementation under test. The cross comparison of the two languages with each other is `integration-tests/compare_canonical_json.py` today and T-12.1 later. The format is in [integration-tests/fixtures/README.md](../../integration-tests/fixtures/README.md).

### 2.9 The value objects of the domain (built, T-03.1)

The immutable types of the envelope, validated at construction, in both languages with the same names and the same accepted inputs (the shared tables of [integration-tests/fixtures/value_objects/](../../integration-tests/fixtures/value_objects)). They import the standard library and other domain packages only.

- **Base value objects** (`domain/base_value_objects/`, import-free of the rest of the domain; Go keeps its own `errors.go` and `rules.go` there): `Identifier` (at least one non-whitespace character, no control characters, valid UTF-8, never trimmed) and `HexDigits` (lowercase hexadecimal, exact width). The value objects below wrap them.
- **Value objects** (`domain/value_objects/`, hand-written next to the `*_generated` files): `EventID` (a UUIDv7 in lowercase; it validates, it never generates: generation is `IDGeneratorPort`, T-03.2), `UTCInstant` (UTC only, years 0001 to 9999, microsecond precision, `String()` is the contract format; `FormatUTCTime` and `format_utc_time` now delegate to it, so the format lives in one place), `ResourceIdentity` (`service_name` and `instance_id` required; `time_zone` for display only), `ActorIdentity`, `CorrelationContext` (W3C `trace_id` of 32 and `span_id` of 16 lowercase hexadecimal digits) and `EventAttributes` (scalars and flat lists of scalars; the limits of size are T-03.5). `Severity` and `ActorKind` stay the generated enums; this task adds `NewSeverity` and `NewActorKind` (Go) and `parse_severity` and `parse_actor_kind` (Python), which accept exactly the generated values.
- **The names do not repeat the generated wire types** (`Resource`, `Actor`, `Context`, `Attributes` of `envelope_generated`): those are the shapes of the JSON, these are the validated domain objects. The mapping between the two is T-03.3.
- **Absent is not empty.** An optional field that is present but empty is invalid (the schema says `minLength` 1). Go models it with functional options and `(value, ok)` accessors (`WithEnvironment("")` makes the field present, and the constructor then rejects it); Python uses `None` for absent.
- **Immutable.** Go: unexported fields, constructors and getters, no setter; `EventAttributes` copies the map and the lists on the way in and on the way out. Python: frozen dataclasses with slots; `EventAttributes` keeps a read-only mapping.
- **Errors are provisional.** Go sentinels live in `domain/errors.go` (the base ones in `base_value_objects/errors.go`), aliased in each file and returned unwrapped by the scalar objects; Python has an exception per rule (`InvalidValueError` family, with the default code `event_schema_invalid`). Their mapping to the codes of the catalog is T-03.7.

### 2.10 The outbound ports (built, T-03.2)

`application/ports/outbound/` holds one port per file in both languages with the same names (Go `PascalCase` methods are `snake_case` in Python; Python has no `context` argument and the API is synchronous). The application imports only the domain: no port mentions HTTP, SQLite or a vendor type. The DTOs the ports use are in `application/dtos/`.

| Port | Methods | Notes |
|---|---|---|
| `ClockPort` | `Now`, `MonotonicNanoseconds`, `ClockInfo` | the wall time as a `UTCInstant`; a monotonic reading that never goes back (the elapsed time of REQ-CTR-008 is the difference of two readings); `ClockInfo` says the source and whether it is synced, each possibly unknown (the keys are absent when unknown, never `false`) |
| `IDGeneratorPort` | `NewEventID`, `NewInstanceID` | a UUIDv7; the instance id of a resource when the consumer gives none |
| `ConfigurationPort` | `Configuration` | hands the core what the consumer passed in; it reads no file and no environment (REQ-ARC-007, ADR-004); `Configuration` holds the resource only, T-03.6 completes it |
| `LogSinkPort`, `EventSinkPort`, `MetricsSinkPort`, `TraceSinkPort`, `DeviceTelemetrySinkPort`, `AITelemetrySinkPort` | `Send(events)` returning a `SendResult` | receive events already validated and masked; **never fail the caller**: the failure is in the result (accepted, dropped, failed, reason; ADR-005) |
| `LocalSpoolPort` | `Append`, `ReadBatch`, `Acknowledge`, `Usage`, `Cleanup` | only the contract; the SQLite adapter is T-06 |

- **Decision, with a default (the maintainer was not asked).** The six sink ports have the **same method shape** (REQ-OUT-001: identical semantics may share an abstraction) but stay **six separate interfaces**, so an adapter declares which capability it serves and each can grow its own typed methods when T-10 gives metrics and traces their own payloads. No `Flush` or `Close` yet: the delivery tasks (T-05) add them.
- **Fakes live with the tests**, not in the library: `go/internal/tests/fakes/` and `python/internal/tests/fakes/`. Each records what it received (as a copy), returns a settable result, can be told to fail or to delay.
- **A test keeps the two languages equal**: each language compares the method names of every port with a table, so a port cannot change in one language only.
- **DTO files are named after the concept**, not `<dto>_inbound/outbound` as the organization's Go standard says, because this library has no `pkg/dtos` wire package to alias.
- **No adapter exists yet**: the adapters of the clock and of the generator are written in T-03.9, the sinks in T-04 and later.

### 2.11 Building an event (built, T-03.3)

`EventBuilderService` (`application/services/`, Go and Python) turns what a capability gives it (`NewEventInput`: event type, severity, data, optionally `occurred_at`, actor, context and attributes) into a complete envelope. It is injected with the clock, the id generator and the configuration ports and a function that validates the data (the composition root of T-03.9 passes `ValidateEvent`). The mappers of `application/mappers/` turn the validated value objects into the generated wire types.

**The order of the steps** (fixed, documented in `Build` / `build`):

1. **One read of the clock.** `recorded_at` is that reading, always: only the library sets it, and `NewEventInput` has **no field** for it, so a caller cannot supply one (REQ-CTR-008, ADR-012). `occurred_at` is the one the caller gave, otherwise the same reading. When `data` has `observed_at` (a UTC time of the contract format), `occurred_at` equals it and overrides a given one; an `observed_at` in another format is rejected with `event_schema_invalid` before any other port is called, and the message never holds the value.
2. **The event id** from `IDGeneratorPort` and **the resource** from `ConfigurationPort`.
3. **The clock attributes.** `clock_source` and `clock_synced` are added to `attributes` only when the clock reports them; an unknown flag is absent, never `false`. When the caller already gave one of these keys the clock value wins.
4. **The envelope** is assembled: `schema_version` is the constant `0.1` of the domain rules (hand-written for now: the generator does not emit it, see the task notes).
5. **The data is validated** against the schema of its type (T-02.6), last. An unknown type gives `event_type_unknown`, a data that does not match gives `event_schema_invalid`.

Masking (T-03.4) and the limits (T-03.5) come after, and T-03.9 wires the chain: **the envelope this service returns is not masked and must not reach a sink as it is** (ADR-006).

**Null versus empty** (decided here for both languages): `data` is never null (a nil input becomes `{}` and a copy of the caller's map is stored); `attributes` is omitted when empty; `actor` and `context` are omitted when absent, and a context with no field counts as absent.

**It never raises or panics** (ADR-005, REQ-DLV-002): a failure of a port, of the validator or any panic becomes a result. Go returns `(Envelope, error)`; Python returns a `BuildResult` holding the envelope or the error (it never raises). A failing port gives `EventBuildFailedError` / `ErrEventBuildFailed`, which has no catalog code yet (T-03.7 decides); a validation failure is a classified error with its code. **An event type that is not in the registry is rejected**: a consumer extension type is not allowed in version 0.1.

### 2.12 Masking (built, T-03.4)

The masker is a domain operation (`domain/operations/masking/`, Go and Python); `MaskingService` (`application/services/`) applies it to an envelope. The detectors of [ADR-006](../decisions/ADR-006-masking-before-every-sink.md) were **ported unchanged**, with the same pattern texts in both languages and the old library as the oracle of the shared tables (Q06-5: nothing is removed).

- **What is masked.** Only `data`, `attributes` and `context` of the envelope, at any depth. The event type, the ids, the times, the severity, the schema version, the resource and the actor are never changed (the resource and the actor are identities given by the code or the configuration). The replacement text is `*****MASKED*****` in both languages.
- **The order of the rules for a field:** a null stays null; the class rule of the field (by its name read in snake case); then the rule of the sensitive names; then the value by its type (text by the six detectors and the extra patterns, numbers and booleans kept, maps and lists walked).
- **The default list of names** (`password`, `token`, `authorization`, `api_key`, `secret` and the rest of the old list) is active unless the consumer **replaces** it with its own list (`FieldNames`); `ExtraFieldNames` adds names. The six detectors and the extra patterns are always on. **`body`, `content` and `payload` stay on the default list**, as in the old library: whether that fits the typed data of the new envelope is an open question for the maintainer.
- **Classification (REQ-SEC-003).** A field name can be given a class and a class a rule: `drop` removes the field, `redact` replaces its value, `allow` skips only the name rule (a secret written inside the text of the field is still hidden). A class with no rule is redacted, the safe default. A rule that is not one of the three, or a pattern that does not compile, is a configuration error (`config_field_invalid`), so the start fails instead of logging without the protection.
- **It never fails the call** (ADR-005): any failure, including a panic or an exception, returns an error and **no envelope**, so the caller drops the event; the unmasked envelope never comes back. The input is never modified.
- **Where the options come from:** the configuration (T-03.6) maps to `MaskerOptions`; the core reads no file and no environment.
- **Regular expressions in two engines.** Go RE2 and Python `re` differ, so the Python side compiles the shared pattern texts through one translation (ASCII word boundaries, the exact set of white space characters of RE2, the flags, the case folding of Go) and the shared tables, a differential test of random inputs and the old library catch a difference. A consumer's extra pattern must stay in the common subset (no look-around, no Go-only classes): a pattern valid in one engine and not in the other fails at start in that engine.
- **Not wired yet.** Nothing calls the service outside the tests: T-03.9 wires the chain (builder, validation, masking, limits, sinks). Until then the spying-sink test builds the chain inside the test.

### 2.13 Limits (built, T-03.5)

`Limits` (a value object) holds the five limits and `ApplyLimits` (a domain operation in `domain/operations/limits/`) applies them; `LimitsService` (`application/services/`) wraps it for the chain. The defaults are those of Q08-9: **32 attribute keys, a key of 64 characters, a value of 256 characters, an event of 64 KiB (65536 bytes) and an HTTP batch of 1 MiB (1048576 bytes)**. All are configurable through `NewLimits` / `Limits(...)`; a value that is not greater than zero, or a batch limit smaller than the event limit, is a configuration error (`config_field_invalid`). T-03.6 maps the configuration to it.

**The policy** (REQ-CTR-010, REQ-SEC-004), the same in both languages:

1. Only `attributes` are cut. The keys are taken in the **sorted order of the canonical form** (UTF-16 code units of the original key): a Go map has no insertion order, and this order makes Go and Python equal. The keys beyond the first 32 are **dropped**.
2. A key longer than 64 characters is **cut** to 64; if the cut key equals a key already kept, the second one is dropped (counted as dropped, not as truncated).
3. A string value longer than 256 characters is **cut** to 256; every string of a list is limited on its own. Numbers, booleans and null are never cut.
4. **Characters are Unicode code points**, not bytes and not UTF-16 units (257 emoji are cut to 256 emoji).
5. After the cuts the size of the event is the number of **bytes of its canonical JSON**. More than 65536 is **rejected** with `limit_exceeded` and counted; exactly 65536 is accepted. A half-event is not useful, so an event is never cut to fit. (This is the rule proposed by the task; the maintainer was not asked and it is the default.)
6. **The counters** (`attribute_keys_dropped`, `attribute_keys_truncated`, `attribute_values_truncated`, `events_rejected`) are plain integers returned with the result and **never hold the cut text**, which may be sensitive. They become diagnostics in T-05.5.

`fits_in_batch(batch_bytes, event_bytes)` is a pure function (the sum must not exceed the batch limit); the framing bytes of a batch belong to the batcher (T-04.4, T-05.1). The service **never raises or panics** (ADR-005): a failure returns an error and no envelope. It runs **after masking**, so a secret is never cut in half and left half visible (ADR-006), and before the sinks; T-03.9 wires the chain.

### 2.14 Configuration and profiles (built, T-03.6)

One logical model, [contracts/config/config.schema.json](../../contracts/config/config.schema.json), with five sections: `resource`, `delivery`, `limits`, `spool` and `masking`. Each language has its own model of it (`Config` in `go/internal/infrastructure/config`, the frozen dataclasses of `python/src/telemetry/internal/infrastructure/config`), the same names, and a test keeps the schema, the model, the table of the documents and `.env.example` equal. **The library never reads the environment or a file** (ADR-004, REQ-ARC-007): the consumer loads the values with its own tool using the names of `x-env` and passes them in; a test reads the sources of both languages and fails on any mention of the environment.

- **Precedence** (REQ-CFG-003): defaults, then the **profile**, then the options in code. A profile is a function that returns a `Config` (`microservice`: `best_effort`; `cronjob` and `worker`: `retry`; `robotics` and `offline_device`: `durable`) and changes **only the delivery mode**. The env tags have **no `default=`**: a default in a tag would override the profile when the consumer loads the environment after choosing it. The defaults live in one place, the code: queue 10000, batch 100, flush 2000 ms, timeout 3000 ms, the limits of T-03.5, spool quota 256 MiB.
- **Validation at start** (REQ-CFG-002): `Validate` (Go) and `validate_config` (Python) return **every problem together**, each with the **field path**, a fixed reason that never holds a configured value, and the code of the catalog (`config_field_invalid`, or `delivery_mode_unknown` for the delivery mode), in the order of the schema. The same problems and codes in both languages (REQ-CNF-004). A value of zero is invalid, so the zero `Config` is not usable: start from a profile.
- **The rules in short:** `resource.service_name` required; optional identifiers when present; `delivery` numbers greater than zero and the batch not greater than the queue; the five limits greater than zero and the batch limit not below the event limit; `spool.directory` required for `durable`; the extra masking patterns must compile and the class rules must be `drop`, `redact` or `allow`. An empty text means not set.
- **`durable`** passes the validation (with its spool directory); the composition root (T-03.9) refuses to start a durable destination without a `LocalSpoolPort`. Accepted by the maintainer? Not asked: it is the default of the task.
- **The ConfigurationPort adapter** is `ConfigurationProvider`: it validates, generates the instance id with the identifier generator port when the consumer gives none, builds the validated value objects (`ResourceIdentity`, `Limits`, the masking options) and hands the application a `Configuration` (a copy). The configuration has no adapter name and no secret (REQ-ARC-013, REQ-CFG-005).
- **Not generated.** The model is hand-written and checked against the schema; REQ-CNF-002 asks that the configuration schema be generated, so it stays open (see the task notes).

### 2.15 The error model (built, T-03.7)

**The code of an error.** Every failure of the core has a code of the catalog ([contracts/errors/catalog.yaml](../../contracts/errors/catalog.yaml)), **identical in Go and Python** (REQ-CNF-004); the text of a message may differ. The catalog has nine codes: `event_schema_invalid`, `event_type_unknown`, `limit_exceeded` (validation); `config_field_invalid`, `delivery_mode_unknown`, `instance_id_unavailable` (configuration); `event_build_failed`, `masking_failed`, `limits_failed` (internal). **T-03.7 added the category `internal` and four codes**, because the failures of the core's own steps (a port failed, the masking or the limits failed) had no code; they are what the diagnostics count as drops by reason (REQ-DGN-001, T-05.5). Adding them is a MINOR change of the contract.

- **Go.** Sentinels live in `domain/errors.go`, `domain/base_value_objects/errors.go` (its own, import-free of the domain) and `application/errors.go`, composed with `errors.Join` so `errors.Is` finds the root cause. A `*domain.ClassifiedError` carries the code and wraps the sentinel. **`value_objects.CodeOf(err)`** returns the code of any error of the core: a classified error wins, otherwise a table maps every sentinel of the domain to its code (it lives in `value_objects` because the generated constants are there and the domain cannot import them). The services return a `*ClassifiedError` for the failures of the application layer.
- **Python.** One root, `TelemetryError`, defined in `domain/base_value_objects/errors.py`; `ClassifiedError` (with `.code`) derives from it and every exception of the library derives from `TelemetryError` (a test walks the modules and fails otherwise). A chained cause (`raise ... from ...`) keeps the root cause, the role of `errors.Join`. **`error_code_of(error)`** reads the code of any exception of the core, following the cause chain.
- **Returned, not raised.** On the hot path the errors are **values**: the services return them (`(Envelope, error)` in Go, `BuildResult`, `MaskResult` and `LimitsResult` in Python) and they never fail the caller (ADR-005). They are raised only at construction and at start (an invalid configuration, REQ-CFG-002).
- **No value in a message.** A message names a field path or an event type (a name, cut to 64 characters, control characters replaced) and **never a value** the caller gave, because a value may be a secret. A test makes 29 real failures with a secret-looking value in the invalid input and fails if the message holds it. It found four leaks while it was written, now fixed (Python: the text of an invalid severity and of an invalid actor kind; Go: the pattern of an invalid extra masking pattern and the class name and rule of an invalid class rule). **A cause that a port itself puts in its own error text stays in the chain** (the library cannot clean an error that a consumer's adapter built), so an adapter must not put a value in its errors.
- **The sentinels and the codes are kept equal by a shared table**: `integration-tests/fixtures/errors/codes.json`, read by one test in each language, plus a check that the generated codes equal the catalog file.
- **Error translation of an adapter** stays in the adapter's `error/` folder; this layer defines only the sentinels of the domain and the application.

### 2.16 The call context (built, T-03.8)

`application/call_context/` (Go and Python) carries the identity and the correlation of a call through the program, migrated from the old `call_depth` package (which stays, unchanged, until T-04.7). A `CallContext` holds the `application`, the `instance_id`, the `execution_id`, the `correlation_id`, the `trace_id`, the `span_id` and the **indentation** of the call depth (it starts at 1 and grows by one at each deeper layer). Empty means not set and is never an error.

- **Go** stores it in `context.Context` under an unexported key type: `WithCallContext`, `FromContext`, `NextIndentation` (a copy one layer deeper; the original is never changed) and `NewRequestContext`. **Python** stores it in a `contextvars.ContextVar` (per-call storage, not shared mutable state: it works with threads, each of which starts with none, and with `asyncio` tasks, which inherit the context of their creator and cannot change each other's): `use_call_context(...)` and `deeper_indentation()` are context managers, `current_call_context()` reads it. The Python names map to the Go ones by that rule.
- **Validation** (`NewCallContext` / `new_call_context`): `trace_id` is 32 and `span_id` 16 lowercase hexadecimal digits and **neither may be all zeros** (W3C Trace Context); the other fields are identifiers (not blank, no control characters, never trimmed). All problems are reported together as one classified error with the code `event_schema_invalid`, naming the fields and never the values. An incoming id is **data, not authorization** (REQ-TRC-003): it is validated and never trusted. It never raises to the caller (Python returns a `CallContextResult`).
- **A new request** (`NewRequestContext` / `start_request`): the application and the instance id are validated, and the old per-request process id becomes the **correlation id**, a UUIDv7 generated with the identifier generator port (`NewEventID`; the port may get its own method later). A failing generator is an `event_build_failed` error with the cause in the chain and no context.
- **What goes to the envelope:** `ToCorrelationContext` / `to_correlation_context` maps only the four correlation ids to the correlation context of the event (REQ-LOG-005) and gives none when all four are empty. **The application and the instance are not part of the envelope context**: the resource comes from the configuration. They stay in the call context for the migration of the old consumers.
- **The indentation is not part of the envelope** either: it is stored here and the console adapter (T-04.2) decides how to draw it. This is the answer proposed by the task; the maintainer was not asked.
- **It carries only identifiers and the depth**, never a secret, a request body or a payload (REQ-LOG-006). The logger that attaches the ids to a log line is T-04.1; W3C propagation over HTTP is T-10.

### 2.17 The composition root and the public entry point (built, T-03.9)

**What a project does.** It builds the core **once**, in code, and passes the adapters it chose: Go `telemetry.New(config, telemetry.WithLogSink(sink), ...)` (package `go/pkg/telemetry`), Python `telemetry.new(config, telemetry.with_log_sink(sink), ...)`. The configuration carries only settings, never an adapter name, and nothing registers itself on import (REQ-ARC-013, ADR-013). Then it calls `Emit` and, at the end, `Shutdown`. A sink given twice is a second destination (REQ-OUT-003).

**The chain of one event** (`EventPipelineService`, `application/services/`): (1) the call context of the caller gives the event its correlation ids when the input has none (REQ-LOG-005); (2) the envelope is built and its data validated (T-03.3); (3) it is **masked** (T-03.4); (4) the **limits** are applied (T-03.5); (5) it is routed by the first segment of its event type (`log` to the log sinks, `metric` to metrics, `trace` to trace, `device` to the device sinks, `ai` to the AI sinks, anything else to the event sinks); (6) it is delivered to **every** destination of that kind. A step that fails drops the event and the result says why (the catalog code, or `no_destination` when the project gave no sink of that kind); a sink that fails or panics is counted and **never stops the others** (ADR-005). `Emit` returns an `EmitResult` (accepted, dropped, failed, reason) and never fails or panics. **No path reaches a sink without the masking** (ADR-006): a spy sink in the tests receives events driven through the whole core with every kind of secret in data, attributes and context, for the six kinds of sink.

**The wiring** is only in `composition_root/` (REQ-ARC-003): `config/` (the options), `dependencies/` (one file per adapter) and `containers/` (the `Core`). The start validates the configuration with **every problem together** (T-03.6) and a **`durable` mode without a spool port is refused** at start with the field `delivery.mode`, in the same list. Two cores in one process share nothing, and there is no global state (REQ-ARC-006): a source test fails on any `init()` in Go, any mutable package-level or module-level state outside a short explicit list of read-only tables, and any registration by import.

**The two adapters written here** (`infrastructure/outbound/shared/`): the **system clock** (UTC wall time, a monotonic reading; it reports no source and no sync state, so it adds no clock attribute) and the **UUIDv7 generator**. **The UUIDv7 is written by hand in both languages, with no third-party dependency**: a pure layout function (48 bits of milliseconds, version 7, variant 10, random bits) checked against hand-computed values, the I/O (the random source and the clock) in `external/`, no fallback to a weaker random source. This **deviates from the default of the task**, which kept `github.com/google/uuid`: Python has no UUIDv7 in its standard library before 3.14, so Python needed its own, and writing it also keeps the Go core free of any dependency (REQ-ARC-005; `go.mod` has no `require` line and `pyproject.toml` no runtime dependency, both checked by a test). The order of two ids of the same millisecond is not defined; the Go generator never lets the time prefix go back.

**The public surface** is thin and equal in both languages by a shared list of names (`integration-tests/fixtures/core/public_names.json`: 16 functions and 26 types; each language may export more). In Go the types are **aliases** of the internal ones (ADR-003), so the project never imports `internal/`. Every exported identifier has a doc comment, checked by a test; `go doc ./pkg/telemetry` shows them.

**What is still a stub.** `Shutdown` returns an empty result until T-05.4 gives it `flush` and `shutdown` (REQ-DLV-010). The sinks are called **synchronously** by the caller until T-05 adds the bounded queue; the spool port is accepted and required for `durable`, but delivery does not use it until T-06. No adapter for the sinks exists yet (T-04: console, file, HTTP) and no capability API beyond `Emit` (T-04.1 and the next epics).

### 2.18 The logger (built, T-04.1)

`LoggerPort` (`application/ports/inbound/`, Go and Python) has one method per severity of the contract: `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Critical` in Go and `trace`, `debug`, `info`, `warn`, `error`, `critical` in Python. A call takes a `LogEntry` (category, function, message and fields) and builds a **`log.entry.recorded`** event: `category` and `message` (and `function` when given) in `data`, the extra fields in `attributes`, the severity of the method; the application is `resource.service_name` from the configuration, the ids of the call context are attached by the pipeline (REQ-LOG-005), and the event goes through the whole chain of T-03.9 (validate, mask, limit, route to the log sinks). The core gives the logger: Go `core.Logger()`, Python `core.logger`; the public names are `Logger` and `LogEntry` in both.

- **A log call returns nothing and never fails** (ADR-005, REQ-DLV-002): no error, no exception, no panic. A rejected event is counted in the diagnostics (T-05.5). *This answers the open question of the task with its default; the maintainer was not asked.* A call at `CRITICAL` does not stop the process.
- **Fields to attributes** (one rule, the same in both languages, checked by 8000 random field sets with no difference): an empty key and a null value are dropped; a string, a boolean and a finite number are kept; an integer outside the exact range (2^53 - 1) becomes its decimal text and `NaN` and the infinities become the texts `NaN`, `+Inf` and `-Inf`; an error becomes its text; a list is kept as a list only when **every** element is a scalar by these rules; anything else (a map, a struct, a list with a map in it) becomes **its canonical JSON text** as a single string. Nothing is lost and nothing is nested (REQ-CTR-010); an unserializable value becomes `[unsupported value]`. The limits of T-03.5 apply afterwards. Exotic types (a time, a set) may convert differently between the languages; the shared cases do not cover them.
- **No automatic capture** (REQ-LOG-006): a log call records exactly what the caller passes. No request body, no payload and no traceback is added by the library.
- **Native logging** (REQ-LOG-002): in Go `SlogHandler` (`infrastructure/inbound/sdk/`, `NewSlogHandler(logger, default_category)`), in Python `LoggingHandler` (`logging.Handler`). The severity comes from the native level: Go below `-4` TRACE, `-4` to `-1` DEBUG, `0` to `3` INFO, `4` to `7` WARN, `8` to `11` ERROR, `12` and above CRITICAL; Python below 10 TRACE, 10 to 19 DEBUG, 20 to 29 INFO, 30 to 39 WARN, 40 to 49 ERROR, 50 and above CRITICAL. The attributes (`extra` in Python) named `category` and `function` set the category and the function; all the others are fields (a Go group is flattened as `group.key`); without a category the default category of the handler is used. **The handler does not read the source, the record's own time or an exception**: the time of the event is the time of the call from the clock port, and a traceback may hold a payload. They may come later as options. **Level filtering is not done here**: the handler always accepts; filtering is T-04.2.
- **Not done:** the minimum level (T-04.2), the sinks themselves (T-04.2 to T-04.6), the queue (T-05).

### 2.19 The shared HTTP client (built, T-04.4)

`infrastructure/outbound/shared/http_client/` (Go and Python, standard library only, no third-party dependency) holds the HTTP pieces that every HTTP adapter reuses (REQ-OUT-006): the settings and their validation, the batch body, the authentication header, the classification of a response, `Retry-After`, the backoff with jitter and a client that sends **one** body and returns a classified `Outcome`. The calls to `net/http` (Go) and `http.client` and `ssl` (Python) are only in `external/`, the translation of a transport failure in `error/`, the pure functions in `helpers/`. The adapters that use it are T-04.5 (the HTTP destination) and T-04.6 (AWS); **which destination retries is decided by the delivery mode in T-05.3**: the client sends once and offers `NextDelay`.

- **Settings** ([contracts/config/http_client.schema.json](../../contracts/config/http_client.schema.json), env names `TELEMETRY_HTTP_*`): `url` (required, http or https, **no URL is fixed in code**), `method` (POST, PUT or PATCH), `timeout_ms`, `compress`, `skip_tls_verification`, `bearer_token`, `api_key_header` and `api_key_value`, and the retry policy (`max_attempts`, `retry_base_delay_ms`, `retry_max_delay_ms`). They are validated with **every problem together**, each with its field and code, and **a message never holds a configured value**. **Authentication over plain `http` is refused unless the host is `localhost`, `127.0.0.1` or `::1`** (a token must not travel in clear text; this is a choice of this task, the maintainer was not asked). A port must be from 1 to 65535. Only one authentication at a time.
- **The batch body** is the canonical JSON of `{ "schema_version": "0.1", "events": [ ... ] }` (REQ-OUT-010), the same bytes in both languages. **Gzip** adds `Content-Encoding: gzip`; because two gzip encoders give different bytes, the compressed body is checked by decompressing it to the same text, not byte for byte (this deviates from the wording of the task). A body larger than the batch limit of T-03.5 (1 MiB by default) is **not sent**: it is a permanent `batch_too_large` outcome with no network call.
- **Classification** (the matrix of section 10.6): `2xx` success; `408`, `429`, `5xx` and a status that is not a valid HTTP status are **transient** (retry with a limit); any other `4xx` and any `3xx` are **permanent** (a redirect is never followed). A network error, a timeout, a refused TLS certificate and a body that cannot be read are transient, each with a fixed reason (`network_error`, `timeout`, `tls_verification_failed`, `response_unreadable`). A class and a reason are all an outcome holds.
- **Retry-After** is read as seconds or as an HTTP date (only the IMF-fixdate and the asctime forms, always GMT), is 0 for a date in the past and is cut to 24 hours so a server cannot make the library wait for ever. **The backoff** is the equal jitter: `ceiling = min(max, base * 2^attempt)`, `delay = floor(ceiling/2 + random * ceiling/2)`; the formula given the random is the same in both languages, the random instants may differ (section 13.1). `NextDelay` retries only a transient outcome while attempts remain and waits the larger of the backoff and the `Retry-After`. **Rate limiting is not built**; `Retry-After` is honored.
- **TLS verification is on by default** (REQ-SEC-005): a server with an untrusted certificate is refused and only the explicit `skip_tls_verification` turns it off. The client builds its transport without a proxy from the environment.
- **Secrets** (REQ-SEC-007): the token and the key have no default and never appear in an outcome or an error, even when the URL carries a secret in its query: transport errors are translated to fixed reasons and never printed.
- **The tests** run against a local server on `127.0.0.1`: every status, a `Retry-After`, a timeout, a closed port, a redirect that is not followed, the headers, gzip, TLS with a self-signed certificate (Python generates a throwaway one with `openssl` at test time and never commits a key) and concurrent senders.

### 2.20 The console destination (built, T-04.2)

`internal/infrastructure/outbound/logging/console/` (Go and Python; the reference tree of REQ-ARC-014, the folder `logging/` for the log adapters: *the answer to the open question of the task, the maintainer was not asked*) writes events to the standard output. It implements **both** the log and the event sink ports (they have the same method shape), with `external/` for the output, `error/` and `helpers/` for the two encoders. It is reached from a project through the public package **`go/pkg/logging/console`** (Go) and **`telemetry.logging.console`** (Python) (REQ-ARC-012): `console.NewDestination(settings)` returns the sink that the project passes to `telemetry.WithLogSink` / `telemetry.with_log_sink`; `NewDestinationWithWriter` takes another writer (tests, files, a pipe). The wiring goes through `composition_root/dependencies/`.

- **Settings** ([contracts/config/console.schema.json](../../contracts/config/console.schema.json), env names `TELEMETRY_CONSOLE_*`): `format` (`json`, the default, or `text`) and `minimum_severity` (the default is `INFO`). **The level no longer follows `APP_ENV`** (ADR-007 is for the old library): the consumer chooses it for each destination, and the library still reads no environment. They are validated with every problem together.
- **Level filtering** (REQ-LOG-003): an event below the minimum severity is not written. It counts as **accepted** in the result (the destination took it and its policy decided not to write it), so the rule that each event is counted once holds; whether the diagnostics should show the filtered events apart is for T-05.5. The order of the severities comes from the generated enum (`SeverityRank`, `SeverityAtLeast`), never repeated.
- **JSON** (REQ-LOG-001): one line per event, the **canonical JSON of the envelope** (T-02.5): the same bytes in both languages.
- **Text:** `occurred_at`, the severity padded to 8 characters, `[category]`, `function:` when there is one, the message, then the attributes sorted by key as `key=value` with each value in its JSON form. An event of another type shows its type and its data as canonical JSON. **Control characters** (a line break, a carriage return, a tab, the escape character, any character below U+0020 and DEL) in the category, the function, the message or an attribute key are written as escapes (`\n`, `\r`, `\t`, `\u001b`): one event can never split or colour a line (log injection). The line never holds more than the masked event holds. A test compared 6000 random events in both languages: **no difference** in either format.
- **Writing:** one `write` call per line, under a lock, so the lines of concurrent callers never interleave; the whole batch of one `Send` stays together. Python writes UTF-8 explicitly (a console encoding must not fail on non-ASCII text). A write failure counts the event as failed with the reason `write_failed` and the others are still written; a serialization failure is `serialization_failed`; a panic or an exception inside the destination is `destination_failed`. **Nothing reaches the log call** (ADR-005). The adapter's own `WriteEvent` / `write_event` returns or raises a classified error with the cause; **it has no catalog code yet** (the delivery codes arrive with T-05).
- **Not done:** the console as the fallback that reports a failing sink (ADR-005 of the old library; its role in the new design is T-05.5), colours, and the minimum severity of the other destinations (the file adapter T-04.3 and the HTTP destination T-04.5 take the same setting).

### 2.21 The file destination (built, T-04.3)

`internal/infrastructure/outbound/logging/file/` (Go and Python, standard library only) appends events to a local file as **JSON Lines**: one line per event, the canonical JSON of the masked envelope (the same bytes as the console's JSON format). It implements the log and the event sink ports, with all file-system I/O in `external/`, the error translation in `error/` and the **pure plans** in `helpers/`. It is public as `go/pkg/logging/file` (`NewDestination(settings)`, which returns a `*Destination` that is passed to `telemetry.WithLogSink`) and `telemetry.logging.file` (`new_destination(settings)`). The folder `logging/` and the policy below answer two open questions of the task with their defaults; the maintainer was not asked.

- **Settings** ([contracts/config/file.schema.json](../../contracts/config/file.schema.json), env names `TELEMETRY_FILE_*`): `directory` (**required: the library picks no path**), `file_name` (`telemetry.jsonl`, no path separator), `minimum_severity` (`INFO`), `max_file_bytes` (10 MiB), `max_files` (5, the current file included) and `quota_bytes` (50 MiB, not below the file size). The numbers have no default in the proposal: these are proposed values. Every problem is reported together.
- **At start** the directory is created if it is missing (owner only, `0700`) and the file is opened for append; if that fails the start fails with **one problem on `directory`** (`the directory cannot be written`), **with no path and no operating system text** (REQ-CFG-002).
- **Rotation by size.** Before a line is written, if the file is not empty and the line would pass `max_file_bytes`, the files rotate: the oldest rotated file (`name.<max_files - 1>`) is removed, the others move one up and the current file becomes `name.1`. A line larger than `max_file_bytes` is still written, to an empty file. With `max_files` 1 the rotation removes the file and starts again.
- **Quota.** After each line, while the total size of the files is over `quota_bytes`, the **oldest rotated file is deleted first**; the current file is never deleted (it may alone pass the quota). The two plans (`PlanRotation`, `PlanQuota`) are pure functions with shared tables and a reference simulation in the fixtures.
- **Counters.** The destination keeps counters of rotations, files deleted for the quota and write, rotation and quota errors (`Counters()` / `counters`); they become diagnostics in T-05.5.
- **Failures.** A write failure counts the event as failed with the reason `write_failed` (and the others still go on), a failed rotation `rotation_failed`, a serialization failure `serialization_failed`, a panic or an exception `destination_failed`; **nothing reaches the log call** (ADR-005) and a failing file never stops the other destinations. The classified errors have no catalog code yet (the delivery codes arrive with T-05).
- **The file is opened for each line and closed at once**: no handle is kept, so a rotation can rename on every platform and there is nothing to close. The cost is one open per event; the batched writing belongs to T-05. Lines of concurrent callers never interleave (one lock per batch, one write per line).
- **Security.** The files are created `0600` and hold masked events only. A test compared **400 random scenarios** (random sizes, number of files, quotas, severities, messages with line breaks, quotes and non-ASCII, several batches) in both languages: the files and the counters are **identical**.
- **Not done:** compression of the rotated files, rotation by time, a reader contract (the file is for operators and log shippers; the SQLite destination is T-06.6), and the minimum severity of the HTTP destination (T-04.5).

### 2.22 The HTTP destination (built, T-04.5)

`internal/infrastructure/outbound/logging/http/` (Go and Python) is the destination that sends batches of events to **the URL the consumer configures** (REQ-OUT-002), on top of the shared client of section 2.19: it adds only the meaning of the payload. It implements the log and the event sink ports. It is public as `go/pkg/logging/http` (`NewDestination(settings)` returns a `*Destination` to pass to `telemetry.WithLogSink`; `NewDestinationWithLimits` takes the limits) and `telemetry.logging.http` (`new_destination`, `new_destination_with_limits`). It has no `external/`: it does no I/O itself.

- **Settings:** the 11 settings of the shared client ([http_client.schema.json](../../contracts/config/http_client.schema.json): the URL, the method, the timeout, compression, TLS verification, the authentication and the retry policy) and **`minimum_severity`** ([http_destination.schema.json](../../contracts/config/http_destination.schema.json), `TELEMETRY_HTTP_MINIMUM_SEVERITY`, default `INFO`). **The URL is always required**: a destination without it fails at start with the field `url` (and every other problem together; the problems of the client come first). In Go the 12 fields are declared explicitly (the canonical JSON writer refuses an embedded struct) with a `ClientSettings()` method; in Python the dataclass is flat with `client_settings()`.
- **One request per batch, no queue, no retry.** `Deliver` (and `Send`, which returns only the result) filters the events below the minimum severity (they count as accepted), makes **one** request with the rest (no request if nothing is left) and maps the **class of the outcome** to the result: **success** accepts the events; a **transient** failure (a network error, a timeout, a refused certificate, 408, 429, 5xx) **fails** them, with the reason (`status_503`, `timeout`, `network_error`...): the worker may retry them (T-05.3, with `NextDelay` and `Retry-After`); a **permanent** rejection (another 4xx, a redirect, a batch over the limit) **drops** them, not to be retried. `Deliver` also returns the full classified outcome for the worker of T-05.1. The queue is T-05.1.
- **A failure never stops the other destinations** (ADR-005, REQ-DLV-005): a core with an HTTP destination whose receiver answers 503 (or does not answer) and a console destination still writes the line to the console, and the result of the event says one accepted and one failed with the reason. A panic or an exception inside the destination is `destination_failed`; **nothing reaches the log call**.
- **What it sends** is the contract of T-04.4: the canonical batch body `{ "events": [...], "schema_version": "0.1" }`, optionally gzipped, with the token or key the consumer configured. The receiver does not exist yet (Q03-1, Q07-9): every test uses a local server on `127.0.0.1`, and no real address is in the repository. 13 shared cases with exact bytes compare Go and Python.
- **Not done:** the queue, the batching and the retry loop (T-05), splitting a batch larger than the limit (the batcher uses `fits_in_batch`), and the profiles selecting the HTTP destination (the project passes adapters in code, ADR-013).

### 2.23 The fake receiver (built, T-04.8)

The real receiver is a separate project (Q03-1, Q07-9), so **every test of delivery plays the receiver with a fake**: `go/internal/tests/fakes/fake_receiver/` and `python/internal/tests/fakes/fake_receiver/` (a deviation from the task, which named `tests/fixtures/`: the fixtures moved to `integration-tests/` and the other fakes of this repository are in `tests/fakes/`). It is under the tests tree and **never part of shipped code**; it is not a receiver design: it asserts the contract of T-04.4 (REQ-OUT-010) and nothing more.

- **It listens only on `127.0.0.1`**, on a free port, and exposes its URL. It holds no credential beyond the token or key the test configures.
- **Per request** it records the method, path, headers, the raw and the decompressed body and the events, then: (1) reads the body and decodes gzip; (2) checks the authentication the test expects (a wrong or missing bearer token or API key is **401**, and the script is not used); (3) rejects a body over 8 MiB with **413**; (4) rejects a body that does not gunzip with **400**; (5) rejects a body that is not `{ "schema_version": "0.1", "events": [ ... ] }` with **400** and **the reason as text** (`schema_version is missing`, `events must be a list`, `an unknown key: x`, `the body is not valid JSON`...). A rejected request is still recorded with its reason.
- **A scripted answer per valid request**, consumed in order, so a test can say "fail twice, then succeed": a status (200, 408, 429 with and without `Retry-After`, 4xx, 5xx, 302 with `Location`), a **delay** longer than the timeout, a **closed connection**, and a **truncated answer**. When the script is used up the default status applies. A test reads back the requests (a copy, in arrival order) and the events, and **waits for N events with a timeout** without a busy loop. It is safe for concurrent use and can be closed (also releasing a request that waits on a delay).
- **Go and Python answer the same script the same way**: 14 shared cases (`integration-tests/fixtures/http/receiver_script_examples.json`) and a **differential test of 500 random scenarios** (the random mix of valid and invalid bodies, gzip good, bad and with an upper case header, wrong and missing authentication, and random scripts): the statuses and the records are identical. That test also found one difference (the order of the oversize check against the authentication), now aligned: authentication first.
- **Where it is used:** the two-destinations test of the HTTP destination now uses it in both languages instead of an ad hoc server; T-05.1 to T-05.6 and T-06.5 will use it. Question left to the maintainer, with the default taken: one fake per language is enough (no separate process for the cross tests yet).

## 3. Current state (exists)

| Layer | Package | What it holds |
|---|---|---|
| domain | `internal/logging/domain` | `rules.go` (levels, the minimum level by environment, the masking detectors as data) and `errors.go` |
| domain | `.../domain/entities` | `LogEntry` (one finished line) |
| domain | `.../domain/value_objects` | `Level`, `Message`, `Fields`, `Location`, `Indentation`, the identifiers |
| domain | `.../domain/operations` | `SensitiveDataMasker` and its detectors (AWS access key, bearer credential, JWT, private key block, connection string password, credential pair) and `IsSensitiveFieldName`, `MaskValue` |
| application | `.../application/services` | `LoggerService`: builds the entry, masks it, sends it to every sink, reports a failing sink to the fallback |
| application | `.../application/call_depth` | `CallContext` carried in `context.Context` (application, instance, process id, indentation) |
| application | `.../application/ports/inbound` | `LoggerPort`: one method per severity |
| application | `.../application/ports/outbound` | `LogSinkPort`, `ClockPort`, `IDGeneratorPort` |
| infrastructure | `.../outbound/console` | one JSON line per entry on the standard output |
| infrastructure | `.../outbound/http_sink` | posts the flat JSON entry to a generic log receiver |
| infrastructure | `.../outbound/aws_http` | adapts the entry to the `add_log` contract of aws_microservice and posts it ([ADR-008](../decisions/ADR-008-aws-http-sink-adapts-to-add-log.md)) |
| infrastructure | `.../outbound/systemclock`, `.../uuidgen` | the clock and the generator of process ids (random v4 UUIDs) |
| composition | `internal/composition_root/containers` | `LoggingContainer`: `NewLoggingContainer` (from a `Config`) and `NewLoggingContainerFromSinks` (from sinks the caller built) |
| composition | `internal/composition_root/dependencies` | one file per adapter: console, HTTP sink, AWS HTTP sink, clock, UUID generator |

Every outbound adapter keeps its I/O in an `external/` folder, its error translation in `error/` and its pure helpers in `helpers/`; the adapter file itself only holds the client and implements the port.

## 4. Data flow of one log call

```text
caller ──► LoggerPort.Info(ctx, Entry)
             │ below the minimum level of APP_ENV? ──► dropped (nil)
             ▼
        take application, instance id, process id and indentation from the CallContext of ctx, when it carries one
             ▼
        build the LogEntry (validates the value objects; the only error the caller can get)
             ▼
        stamp it with ClockPort.Now, MASK it (SensitiveDataMasker)
             ▼
        for every sink: Send(ctx, LogEntryOutboundDTO)
             │ a sink fails ──► the entry is dropped for THAT sink, an ERROR report goes to the fallback (console)
             ▼
        nil
```

A sink never fails the caller and never stops the other sinks ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)). The entry is masked once, before any sink ([ADR-006](../decisions/ADR-006-masking-before-every-sink.md)). The minimum level follows the environment, not a variable ([ADR-007](../decisions/ADR-007-minimum-level-follows-the-environment.md)).

## 5. The two ways to build a logger

| Way | Function | Use it when |
|---|---|---|
| From settings | `logging.New(ctx, app_env, Config)` | the consumer loads `Config` from its environment and lets the settings switch the sinks on |
| From sinks | `logging.NewWithSinks(ctx, app_env, mask_patterns, sinks...)` | the consumer builds the sinks itself with `NewConsoleSink`, `NewHTTPSink`, `NewAWSHTTPSink` (or its own `Sink`) ([ADR-009](../decisions/ADR-009-ready-made-sink-constructors.md)) |

Both return the logger and a shutdown function to call on exit. Both use the console as the fallback sink.

## 6. Decision records at a glance

| ADR | Decision |
|---|---|
| [ADR-001](../decisions/ADR-001-adopt-shared-go-standard.md) | follow the shared Go Clean/Hexagonal standard |
| [ADR-002](../decisions/ADR-002-one-module-capabilities-side-by-side.md) | one module, capabilities side by side |
| [ADR-003](../decisions/ADR-003-public-surface-is-pkg-logging.md) | the public surface is `pkg/logging`, by aliases |
| [ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md) | the library never reads the environment |
| [ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md) | a failing sink never fails the caller |
| [ADR-006](../decisions/ADR-006-masking-before-every-sink.md) | masking happens before every sink |
| [ADR-007](../decisions/ADR-007-minimum-level-follows-the-environment.md) | the minimum level follows `APP_ENV` |
| [ADR-008](../decisions/ADR-008-aws-http-sink-adapts-to-add-log.md) | the AWS HTTP sink adapts to the `add_log` contract |
| [ADR-009](../decisions/ADR-009-ready-made-sink-constructors.md) | ready-made constructors for each sink |
| [ADR-010](../decisions/ADR-010-no-deployment-release-by-tags.md) | no deployment; release by tags |
| [ADR-011](../decisions/ADR-011-adopt-the-telemetry-standard-go-python.md) | adopt the telemetry standard (accepted, not built) |
| [ADR-012](../decisions/ADR-012-two-times-in-every-event.md) | two times in every event (accepted, not built) |
| [ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md) | one Go module per adapter, chosen in code (accepted, not built) |

## 7. External services

| Service | Direction | How |
|---|---|---|
| A generic log receiver | out (HTTP) | `http_sink`: `POST host:port/path` with the flat JSON entry, 3 s timeout, no retry |
| aws_microservice (`add_log`) | out (HTTP) | `aws_http`: `POST host:port/path` (default `/api/v1/cloudwatch/logs/add`) with `log_group_name`, `log_stream_name`, `message`, `timestamp` (ms), 3 s timeout, no retry |

The library holds no credential: the receiver is reached by address only. CloudWatch Logs is reached through aws_microservice, never directly.

## 8. Persistence, errors, startup

- **Persistence:** none. The library stores nothing; an entry that cannot be delivered is dropped.
- **Errors:** sentinels in `application/errors.go` and `domain/errors.go`, composed with `errors.Join` so the root cause is kept (`errors.Is` works on both). The only error a log call can return is a failure to build the entry (`ErrLoggerEntryBuildFailed`).
- **Startup:** `New` / `NewWithSinks` fail with the reason when a sink that is switched on lacks a required value, when `app_env` has no level, or when an extra mask pattern is not a regular expression.

## 9. Discrepancies

None known. Planned and not built: the `metrics` capability (`internal/metrics`, `pkg/metrics`).
