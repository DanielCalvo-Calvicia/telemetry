# Tasks

Hierarchical task plan used to build the whole telemetry standard of [PR-001](../proposals/PR-001-telemetry-standard-go-python.md): **T-01 repository foundation, T-02 the contract and the generator, T-03 core domain and application, T-04 logging and the core adapters, T-05 delivery, T-06 the spool and the SQLite destination, T-07 executions, faults, reports and health, T-08 devices, T-09 AI agents, T-10 metrics and traces, T-11 audit, T-12 cross-language conformance, T-13 documentation, examples and release**. Nothing of it is built: T-01.1 to T-01.4 (the acceptance of PR-001, the repository layout, the Python skeleton and the architecture tests) are done and every other task is `todo`; two pilot runs and a few decisions wait for the maintainer.

The status of every task is on the board in section 5 (and in [ORDER.md](ORDER.md)).

**Plan revision 1 (2026-10-28):** written from PR-001 version 0.2, [ADR-011](../decisions/ADR-011-adopt-the-telemetry-standard-go-python.md) to [ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md) and the answers of [Q01](../setup/questionnaires/Q01_scope-repository-layout-and-rollout.md) to [Q09](../setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md). The build order is the one of Q07-16, and every capability is built one at a time (Q02-5).

**The order in which the tasks must be done, with the state of each one, is in [ORDER.md](ORDER.md)** (kept in step with the board of section 5 and with every task file).

## 1. Structure and naming

- Every task is a Markdown file. A task with subtasks lives in a folder of the same name, and its own file has the same name inside that folder. Leaves are single files.
- IDs are hierarchical and stable: `T-04` → `T-04.7`. Never reuse or renumber an ID.
- A parent task describes goal, order and exit criteria; **leaves carry the implementation detail**.

```text
docs/tasks/
├── README.md                                   (this file)
├── ORDER.md                                    the order and the state of every task
├── PROMPTS.md                                  prompts to start, pause and resume the build
├── T-01_repository-foundation/                 big task 1
│   ├── T-01_repository-foundation.md
│   ├── T-01.1_accept-pr-001-version-0-2.md     leaf
│   └── T-01.2_repository-layout.md             leaf
└── ...
```

## 2. Execution protocol (how the assistant works through this plan)

1. Pick the first task whose status is `todo` and whose `Depends on` tasks are all `done`. Decision tasks (`decision`) that are `awaiting-user` **stop the chain**: ask the user, record the answer in the task's *Resolution* section, then continue.
2. Before writing Go code, apply the `go-architecture-standard` skill and follow it literally. Before writing Python code, follow the rules of the architecture document (the layers and folder names of Go, PEP 8 names) until the Python standard of the organization can be read (Q07-8).
3. Create or modify only the files listed under *Files* (tests and fixtures included: a task that creates code lists the fixtures it needs). Adding another dependency, folder or file requires updating the task first.
4. Implement, then run the task's *Verification* and the baseline in section 3. A task is `done` only when every acceptance criterion passes, **in every language the task names**.
5. Set the task's `Status` to `done` (and its row in section 5 and in [ORDER.md](ORDER.md)), update parent tasks if their exit criteria are met, update the documents the task lists and the ones `CLAUDE.md` requires, and record any new fact or deviation in the task's *Notes*.
6. Never edit generated files by hand; change the contract or the generator and run it again.
7. Do not commit, push, tag or open a pull request unless the user asks. A tag is permanent.

Status values: `todo`, `awaiting-user` (a decision or a run needs the maintainer), `deferred` (waits for a project that needs it), `blocked` (depends on unfinished or undecided work), `in-progress`, `done`.

## 3. Baseline definition of done (applies to every code task)

- **Go**, in every module the task touches: `gofmt -l .` prints nothing; `goimports -l .` prints nothing; `go vet ./...` and `go build ./...` succeed; `go test ./... -count=1` passes; `go test ./... -coverpkg=./internal/... -coverprofile=coverage.out` gives at least 95% without the `external/` folders. `-race` needs cgo.
- **Python**: the tests pass and the coverage is at least 95% without the `external/` folders (Q06-7); the code follows the layers of Go and PEP 8.
- After a change of `contracts/`: the generator runs and the committed generated code does not differ (T-02.4). After a capability: the cross tests of T-12 pass.
- Every exported Go identifier has a doc comment that starts with its own name; every public Python class and function has a docstring.
- **Names:** locals, parameters, unexported fields and import aliases are descriptive snake_case in Go; Go identifiers that are exported keep the Go convention; the JSON fields are `snake_case` in both languages (Q02-7).
- **Errors:** sentinels composed with `errors.Join` in Go (in `errors.go` of the owning layer) and an exception hierarchy in Python, with the same error codes in both languages (REQ-CNF-004).
- **Layers:** domain imports nothing from the other layers; application defines the ports; only a composition root imports concrete adapters (the core's for its own adapters, the consumer's for the optional ones, REQ-ARC-013); the I/O of an adapter lives in its `external/` folder, its error translation in `error/`, its pure helpers in `helpers/`.
- **The library never reads the environment or a file** (ADR-004), a failing destination never fails the caller (ADR-005) and masking happens before any destination (ADR-006): do not add a code path that sends an event unmasked.
- **Tests:** every test lives under `internal/tests/` mirroring the code tree, black-box (`package x_test` in Go), and shared data under `integration-tests/fixtures/` and the fixtures folder of each language.
- **Public repository:** no secret, no internal host name or IP address, no account id, no customer name, no binary; placeholders only. The switch `run_against_deployed_service` of the e2e test is `false` in everything that is committed.
- **Documents:** the change updates the documents `CLAUDE.md` lists for its kind, marks what is planned as planned, and every link resolves.

### 3.1 Layout of the code (what every task must follow)

```text
contracts/                  schemas/ (envelope, log, execution, fault, report, device/, ai/, audit, metric, trace),
                            enums/ (severity, execution_transitions.yaml, device/, ai/), capabilities/registry.yaml,
                            conventions/naming.md, storage/ (spool and SQLite destination layouts)
go.work                     joins the Go modules for local work and CI
go/                         core module github.com/DanielCalvo-Calvicia/telemetry/go
├── internal/               domain/ application/ (dtos, mappers, ports/{inbound,outbound}, services)
│                           infrastructure/{config, inbound/{sdk,cli}, outbound/<functionality>/<adapter>}
│                           composition_root/{config,containers,dependencies}
│                           tests/{architecture,fixtures,contract,domain,application,infrastructure,composition_root,e2e}
└── pkg/<functionality>/<adapter>/    public packages; an optional adapter is a module of its own (go.mod)
python/                     pyproject.toml (Python >= 3.10, extras), src/telemetry/{internal/..., <functionality>/<adapter>/},
                            internal/tests/ mirroring the code
integration-tests/fixtures/ events and files that both languages run with the same expected result
```

## 4. Facts the tasks rely on

Verified on 2026-10-28 against the repository and the answers of the questionnaires.

| # | Fact |
|---|---|
| F1 | The repository today is the Go logging library: module `github.com/DanielCalvo-Calvicia/telemetry`, public surface `pkg/logging`, Go 1.25.5, console, HTTP and AWS HTTP sinks, masking, call context, 98.4% coverage (2026-10-20). Tags `v0.1.0` and `v0.2.0`. It has no `contracts/`, no `python/` and no `go.work`. |
| F2 | The new Go module is `github.com/DanielCalvo-Calvicia/telemetry/go` (a different path, so the old tags never clash). Its tags are `go/vX.Y.Z`; an adapter's are `go/pkg/<functionality>/<adapter>/vX.Y.Z`; Python's are `python-vX.Y.Z` (Q07-1, Q06-8). |
| F3 | The two known projects that will use the standard are in Python: hardware on a Raspberry Pi that runs Python 3.11.4, and an AI agent that already uses Langfuse (Q04-1, Q05-1, Q07-6). There is no Go pilot (Q07-5). The minimum Python is 3.10. |
| F4 | The receiver of the HTTP events does not exist and is a separate project; the tests use a fake (Q03-1, Q07-9). The only consumer of the old library takes it from the published package (Q01-5). |
| F5 | Defaults: queue 10,000 events, batch 100, flush every 2 s, timeout 3 s; 32 attributes per event, key 64 and value 256 characters, event 64 KiB, HTTP batch 1 MiB; spool 256 MiB per destination (Q03-4, Q03-5, Q08-9). |
| F6 | Every event has `occurred_at` and `recorded_at`, both UTC with six decimals, and `resource` may have a `time_zone` for display only ([ADR-012](../decisions/ADR-012-two-times-in-every-event.md), Q09-1). |
| F7 | SQLite is used twice: the spool (internal) and the SQLite destination (a layout in `contracts/` that a reader only reads). The Go driver is pure Go, without cgo (Q09-4, Q09-5). |
| F8 | The library has no hardware drivers; the device module is contracts plus typed source helpers (Q04-3, D-05). LangChain and LlamaIndex exist only in Python (Q07-14). |
| F9 | `1.0` is released only when every capability works; until then every release is a `0.x` beta (Q09-6). |
| F10 | No metrics or tracing backend and no audit need exist today, so those adapters are deferred (Q08-8, Q06-6). |

## 5. Status board

| ID | Task | Kind | Status |
|---|---|---|---|
| [T-01](T-01_repository-foundation/T-01_repository-foundation.md) | **Repository foundation** | epic | in-progress (T-01.1 to T-01.4 and T-01.6 done 2026-10-28; T-01.5 waits for a GitHub run) |
| [T-01.1](T-01_repository-foundation/T-01.1_accept-pr-001-version-0-2.md) | Accept PR-001 version 0.2 | decision | done (2026-10-28) |
| [T-01.2](T-01_repository-foundation/T-01.2_repository-layout.md) | Repository layout | code | done (2026-10-28) |
| [T-01.3](T-01_repository-foundation/T-01.3_python-project-skeleton.md) | Python project skeleton | code | done (2026-10-28) |
| [T-01.4](T-01_repository-foundation/T-01.4_architecture-tests.md) | Architecture tests | test | done (2026-10-28) |
| [T-01.5](T-01_repository-foundation/T-01.5_ci.md) | CI | code | in-progress (written and checked locally on 2026-10-28; the run on GitHub needs a pull request) |
| [T-01.6](T-01_repository-foundation/T-01.6_tags-and-the-release-rule.md) | Tags and the release rule | docs | done (2026-10-28) |
| [T-02](T-02_contract-and-generator/T-02_contract-and-generator.md) | **The contract and the generator** | epic | done (T-02.1 to T-02.8, 2026-10-28 and 2026-10-29) |
| [T-02.1](T-02_contract-and-generator/T-02.1_layout-of-contracts.md) | Layout of `contracts/` | docs | done (2026-10-28) |
| [T-02.2](T-02_contract-and-generator/T-02.2_envelope-schema.md) | Envelope schema | code | done (2026-10-28) |
| [T-02.3](T-02_contract-and-generator/T-02.3_event-types-and-error-codes.md) | Event types and error codes | code | done (2026-10-28) |
| [T-02.4](T-02_contract-and-generator/T-02.4_the-generator.md) | The generator | code | done (2026-10-28) |
| [T-02.5](T-02_contract-and-generator/T-02.5_canonical-json.md) | Canonical JSON | code | done (2026-10-28) |
| [T-02.6](T-02_contract-and-generator/T-02.6_validation-at-the-source.md) | Validation at the source | code | done (2026-10-29) |
| [T-02.7](T-02_contract-and-generator/T-02.7_capabilities-registry-and-its-check.md) | Capabilities registry and its check | code | done (2026-10-29) |
| [T-02.8](T-02_contract-and-generator/T-02.8_shared-fixtures.md) | Shared fixtures | test | done (2026-10-29) |
| [T-03](T-03_core-domain-and-application/T-03_core-domain-and-application.md) | **Core domain and application** | epic | todo |
| [T-03.1](T-03_core-domain-and-application/T-03.1_value-objects-and-base-value-objects.md) | Value objects and base value objects | code | done (2026-10-29) |
| [T-03.2](T-03_core-domain-and-application/T-03.2_ports.md) | Ports | code | done (2026-10-29) |
| [T-03.3](T-03_core-domain-and-application/T-03.3_building-an-event.md) | Building an event | code | done (2026-10-29) |
| [T-03.4](T-03_core-domain-and-application/T-03.4_masking-and-redaction.md) | Masking and redaction | code | done (2026-10-30) |
| [T-03.5](T-03_core-domain-and-application/T-03.5_limits.md) | Limits | code | done (2026-10-30) |
| [T-03.6](T-03_core-domain-and-application/T-03.6_configuration-and-profiles.md) | Configuration and profiles | code | todo |
| [T-03.7](T-03_core-domain-and-application/T-03.7_error-model.md) | Error model | code | todo |
| [T-03.8](T-03_core-domain-and-application/T-03.8_call-context.md) | Call context | code | todo |
| [T-03.9](T-03_core-domain-and-application/T-03.9_core-composition-root-and-the-public-entry-point.md) | Core composition root and the public entry point | code | todo |
| [T-04](T-04_logging-and-core-adapters/T-04_logging-and-core-adapters.md) | **Logging and the core adapters** | epic | todo |
| [T-04.1](T-04_logging-and-core-adapters/T-04.1_logger-and-the-log-entry-recorded-event.md) | Logger and the `log.entry.recorded` event | code | todo |
| [T-04.2](T-04_logging-and-core-adapters/T-04.2_console-adapter.md) | Console adapter | code | todo |
| [T-04.3](T-04_logging-and-core-adapters/T-04.3_file-adapter.md) | File adapter | code | todo |
| [T-04.4](T-04_logging-and-core-adapters/T-04.4_shared-http-infrastructure.md) | Shared HTTP infrastructure | code | todo |
| [T-04.5](T-04_logging-and-core-adapters/T-04.5_http-destination.md) | HTTP destination | code | todo |
| [T-04.6](T-04_logging-and-core-adapters/T-04.6_aws-http-adapter.md) | AWS HTTP adapter | code | todo |
| [T-04.7](T-04_logging-and-core-adapters/T-04.7_replace-and-remove-the-old-library.md) | Replace and remove the old library | code | todo |
| [T-04.8](T-04_logging-and-core-adapters/T-04.8_fake-receiver-for-tests.md) | Fake receiver for tests | test | todo |
| [T-05](T-05_delivery/T-05_delivery.md) | **Delivery, buffering and failures** | epic | todo |
| [T-05.1](T-05_delivery/T-05.1_bounded-queue-and-worker.md) | Bounded queue and worker | code | todo |
| [T-05.2](T-05_delivery/T-05.2_backpressure-and-priorities.md) | Backpressure and priorities | code | todo |
| [T-05.3](T-05_delivery/T-05.3_delivery-modes-best-effort-and-retry.md) | Delivery modes `best_effort` and `retry` | code | todo |
| [T-05.4](T-05_delivery/T-05.4_flush-and-shutdown.md) | `flush` and `shutdown` | code | todo |
| [T-05.5](T-05_delivery/T-05.5_diagnostics.md) | Diagnostics | code | todo |
| [T-05.6](T-05_delivery/T-05.6_failure-matrix-tests.md) | Failure-matrix tests | test | todo |
| [T-06](T-06_spool-and-sqlite-destination/T-06_spool-and-sqlite-destination.md) | **The spool (`durable` mode) and the SQLite destination** | epic | todo |
| [T-06.1](T-06_spool-and-sqlite-destination/T-06.1_layouts-in-contracts-storage.md) | Layouts in `contracts/storage/` | docs | todo |
| [T-06.2](T-06_spool-and-sqlite-destination/T-06.2_choose-the-go-driver.md) | Choose the Go driver | decision | awaiting-user |
| [T-06.3](T-06_spool-and-sqlite-destination/T-06.3_spool-adapter-in-go.md) | Spool adapter in Go | code | todo |
| [T-06.4](T-06_spool-and-sqlite-destination/T-06.4_spool-adapter-in-python.md) | Spool adapter in Python | code | todo |
| [T-06.5](T-06_spool-and-sqlite-destination/T-06.5_durable-mode.md) | `durable` mode | code | todo |
| [T-06.6](T-06_spool-and-sqlite-destination/T-06.6_sqlite-destination-in-go-and-python.md) | SQLite destination in Go and Python | code | todo |
| [T-06.7](T-06_spool-and-sqlite-destination/T-06.7_cross-tests-of-the-files.md) | Cross tests of the files | test | todo |
| [T-07](T-07_executions-faults-reports-and-health/T-07_executions-faults-reports-and-health.md) | **Executions, faults, reports and health** | epic | todo |
| [T-07.1](T-07_executions-faults-reports-and-health/T-07.1_execution-model-and-transitions.md) | Execution model and transitions | code | todo |
| [T-07.2](T-07_executions-faults-reports-and-health/T-07.2_execution-events-and-api.md) | `execution.*` events and API | code | todo |
| [T-07.3](T-07_executions-faults-reports-and-health/T-07.3_faults-and-generic-reports.md) | Faults and generic reports | code | todo |
| [T-07.4](T-07_executions-faults-reports-and-health/T-07.4_health-and-heartbeats.md) | Health and heartbeats | code | todo |
| [T-07.5](T-07_executions-faults-reports-and-health/T-07.5_cross-tests.md) | Cross tests | test | todo |
| [T-08](T-08_devices/T-08_devices.md) | **Devices and hardware** | epic | todo |
| [T-08.1](T-08_devices/T-08.1_device-schemas-and-enums.md) | Device schemas and enums | code | todo |
| [T-08.2](T-08_devices/T-08.2_source-helpers.md) | Source helpers | code | todo |
| [T-08.3](T-08_devices/T-08.3_sampling-and-batching.md) | Sampling and batching | code | todo |
| [T-08.4](T-08_devices/T-08.4_command-correlation.md) | Command correlation | code | todo |
| [T-08.5](T-08_devices/T-08.5_fixtures-from-annex-b.md) | Fixtures from Annex B | test | todo |
| [T-08.6](T-08_devices/T-08.6_raspberry-pi-pilot.md) | Raspberry Pi pilot | test | todo |
| [T-08.7](T-08_devices/T-08.7_edit-the-examples-with-the-real-projects.md) | Edit the examples with the real projects | docs | todo |
| [T-09](T-09_ai-agents/T-09_ai-agents.md) | **AI agents** | epic | todo |
| [T-09.1](T-09_ai-agents/T-09.1_ai-schemas-and-enums.md) | AI schemas and enums | code | todo |
| [T-09.2](T-09_ai-agents/T-09.2_generic-callbacks.md) | Generic callbacks | code | todo |
| [T-09.3](T-09_ai-agents/T-09.3_privacy-of-prompts.md) | Privacy of prompts | code | todo |
| [T-09.4](T-09_ai-agents/T-09.4_hosted-model-providers.md) | Hosted model providers | code | todo |
| [T-09.5](T-09_ai-agents/T-09.5_check-langfuse-support-in-go.md) | Check Langfuse support in Go | decision | todo |
| [T-09.6](T-09_ai-agents/T-09.6_langfuse-adapter.md) | Langfuse adapter | code | todo |
| [T-09.7](T-09_ai-agents/T-09.7_langchain-adapter.md) | LangChain adapter | code | todo |
| [T-09.8](T-09_ai-agents/T-09.8_llamaindex-adapter.md) | LlamaIndex adapter | code | todo |
| [T-09.9](T-09_ai-agents/T-09.9_agent-pilot.md) | Agent pilot | test | todo |
| [T-10](T-10_metrics-and-traces/T-10_metrics-and-traces.md) | **Metrics and traces** | epic | todo (T-10.4 and T-10.5 deferred) |
| [T-10.1](T-10_metrics-and-traces/T-10.1_metrics-api.md) | Metrics API | code | todo |
| [T-10.2](T-10_metrics-and-traces/T-10.2_traces-api.md) | Traces API | code | todo |
| [T-10.3](T-10_metrics-and-traces/T-10.3_w3c-trace-context.md) | W3C Trace Context | code | todo |
| [T-10.4](T-10_metrics-and-traces/T-10.4_opentelemetry-adapter.md) | OpenTelemetry adapter | code | deferred |
| [T-10.5](T-10_metrics-and-traces/T-10.5_prometheus-adapter.md) | Prometheus adapter | code | deferred |
| [T-10.6](T-10_metrics-and-traces/T-10.6_cross-tests.md) | Cross tests | test | todo |
| [T-11](T-11_audit/T-11_audit.md) | **Audit events** | epic | todo (T-11.4 deferred) |
| [T-11.1](T-11_audit/T-11.1_audit-schemas.md) | Audit schemas | code | todo |
| [T-11.2](T-11_audit/T-11.2_separate-routing.md) | Separate routing | code | todo |
| [T-11.3](T-11_audit/T-11.3_the-warning.md) | The warning | docs | todo |
| [T-11.4](T-11_audit/T-11.4_audit-adapter.md) | Audit adapter | code | deferred |
| [T-12](T-12_cross-language-conformance/T-12_cross-language-conformance.md) | **Cross-language conformance** | epic | todo (recurring: it grows with every capability) |
| [T-12.1](T-12_cross-language-conformance/T-12.1_cross-test-runner.md) | Cross-test runner | test | todo |
| [T-12.2](T-12_cross-language-conformance/T-12.2_registry-gate.md) | Registry gate | code | todo |
| [T-12.3](T-12_cross-language-conformance/T-12.3_mechanical-parity-check.md) | Mechanical parity check | test | todo |
| [T-12.4](T-12_cross-language-conformance/T-12.4_maturity-report.md) | Maturity report | code | todo |
| [T-13](T-13_documentation-examples-and-release/T-13_documentation-examples-and-release.md) | **Documentation, examples and release** | epic | todo (T-13.1 is recurring: it runs after every epic) |
| [T-13.1](T-13_documentation-examples-and-release/T-13.1_documentation-sync-recurring.md) | Documentation sync (recurring) | docs | todo |
| [T-13.2](T-13_documentation-examples-and-release/T-13.2_examples.md) | Examples | docs | todo |
| [T-13.3](T-13_documentation-examples-and-release/T-13.3_security-checklist.md) | Security checklist | docs | todo |
| [T-13.4](T-13_documentation-examples-and-release/T-13.4_releases-0x.md) | Releases `0.x` | code | todo |
| [T-13.5](T-13_documentation-examples-and-release/T-13.5_the-1-0-check.md) | The `1.0` check | decision | awaiting-user |
| [T-13.6](T-13_documentation-examples-and-release/T-13.6_note-for-the-old-consumer.md) | Note for the old consumer | docs | todo |

## 6. Realignment of the earlier plan

None: this is revision 1. The old Go logging library keeps working until T-04.7 replaces it; the old module path and its tags `v0.1.0` and `v0.2.0` stay available afterwards.
