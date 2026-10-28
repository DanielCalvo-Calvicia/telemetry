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

`contracts/` is the single source of truth between the two languages (REQ-CNF-001). Its layout, the naming rule, the envelope schema, the first event type (`log.entry.recorded`), the registry of event types and the catalog of error codes exist (T-02.1 to T-02.3); there is no generator yet (T-02.4).

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
