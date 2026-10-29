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

The Go function walks the value with `reflect` and honours the `json` tags (`-`, `omitempty`), so an int and a float stay different; the Python function accepts a dict or the generated dataclass (an optional field that is `None` is omitted). The errors are small classified errors that carry a code of the catalog (`domain/errors.go`, `domain/errors.py`); the full error model is T-03.7. The shared fixtures and the comparison script are in `integration-tests/` and `go/internal/tests/fixtures/canonical_json/`.

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
- **Today.** 17 capabilities, all `beta`, none `supported`; 130 requirements: **10 covered by real tests** (the architecture, canonical JSON, event validation and generator requirements), **112 pending** on a task and **8 with no task yet**. Run it with `cd tools/registry_check && go run . --spec ../../docs/proposals/PR-001-telemetry-standard-go-python.md --registry ../../contracts/capabilities/registry.yaml --root ../.. --tasks ../../docs/tasks`.

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
