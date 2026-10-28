# Task order

The order in which the tasks must be done, with the state of each one and the date its file was last updated. It follows the execution protocol of the [tasks README](README.md) (section 2): the first task whose state is Todo and whose dependencies are all Done.

**Keep this file true:** every change to a task (a status, a dependency, a new task, an order change) updates this table, the board of the [tasks README](README.md) section 5 and the task file itself in the same piece of work.

## States

| State | Meaning |
|---|---|
| Done | Implemented and verified (or, for a decision or a run, taken and recorded) |
| In progress | Some of its subtasks are done and some are not |
| Todo | Ready to start when its dependencies are done |
| Awaiting the maintainer | A decision or a run on real hardware needs the maintainer |
| Blocked | Waits for another task or for a decision |
| Deferred | Waits for a project that needs it (a metrics backend, an audit need) |
| Recurring | Repeated after every epic (T-12, T-13.1) |

Written on 2026-10-28 from PR-001 version 0.2 and the answers of Q01 to Q09, with the build order of Q07-16 (the core and the delivery, then executions and faults, then devices, then AI, then metrics and traces, then audit).

## Still to do, in the order it must be done

| # | Task | Name | Kind | State | Detail | Depends on | Last updated |
|---|---|---|---|---|---|---|---|
| 1 | [T-01](T-01_repository-foundation/T-01_repository-foundation.md) | Repository foundation | epic | In progress | in-progress (T-01.1 done 2026-10-28) | PR-001 version 0.2 accepted (done, T-01.1) | 2026-10-28 |
| 2 | [T-01.1](T-01_repository-foundation/T-01.1_accept-pr-001-version-0-2.md) | Accept PR-001 version 0.2 | decision | Done | done (2026-10-28) | — | 2026-10-28 |
| 3 | [T-01.2](T-01_repository-foundation/T-01.2_repository-layout.md) | Repository layout | code | Todo | todo | T-01.1 | 2026-10-28 |
| 4 | [T-01.3](T-01_repository-foundation/T-01.3_python-project-skeleton.md) | Python project skeleton | code | Todo | todo | T-01.2 | 2026-10-28 |
| 5 | [T-01.4](T-01_repository-foundation/T-01.4_architecture-tests.md) | Architecture tests | test | Todo | todo | T-01.2, T-01.3 | 2026-10-28 |
| 6 | [T-01.5](T-01_repository-foundation/T-01.5_ci.md) | CI | code | Todo | todo | T-01.2, T-01.3 | 2026-10-28 |
| 7 | [T-01.6](T-01_repository-foundation/T-01.6_tags-and-the-release-rule.md) | Tags and the release rule | docs | Todo | todo | T-01.2 | 2026-10-28 |
| 8 | [T-02](T-02_contract-and-generator/T-02_contract-and-generator.md) | The contract and the generator | epic | Todo | todo | T-01 | 2026-10-28 |
| 9 | [T-02.1](T-02_contract-and-generator/T-02.1_layout-of-contracts.md) | Layout of `contracts/` | docs | Todo | todo | T-01.2 | 2026-10-28 |
| 10 | [T-02.2](T-02_contract-and-generator/T-02.2_envelope-schema.md) | Envelope schema | code | Todo | todo | T-02.1 | 2026-10-28 |
| 11 | [T-02.3](T-02_contract-and-generator/T-02.3_event-types-and-error-codes.md) | Event types and error codes | code | Todo | todo | T-02.2 | 2026-10-28 |
| 12 | [T-02.4](T-02_contract-and-generator/T-02.4_the-generator.md) | The generator | code | Todo | todo | T-02.2, T-02.3 | 2026-10-28 |
| 13 | [T-02.5](T-02_contract-and-generator/T-02.5_canonical-json.md) | Canonical JSON | code | Todo | todo | T-02.4 | 2026-10-28 |
| 14 | [T-02.6](T-02_contract-and-generator/T-02.6_validation-at-the-source.md) | Validation at the source | code | Todo | todo | T-02.4 | 2026-10-28 |
| 15 | [T-02.7](T-02_contract-and-generator/T-02.7_capabilities-registry-and-its-check.md) | Capabilities registry and its check | code | Todo | todo | T-02.1 | 2026-10-28 |
| 16 | [T-02.8](T-02_contract-and-generator/T-02.8_shared-fixtures.md) | Shared fixtures | test | Todo | todo | T-02.5 | 2026-10-28 |
| 17 | [T-03](T-03_core-domain-and-application/T-03_core-domain-and-application.md) | Core domain and application | epic | Todo | todo | T-02 | 2026-10-28 |
| 18 | [T-03.1](T-03_core-domain-and-application/T-03.1_value-objects-and-base-value-objects.md) | Value objects and base value objects | code | Todo | todo | T-02.4 | 2026-10-28 |
| 19 | [T-03.2](T-03_core-domain-and-application/T-03.2_ports.md) | Ports | code | Todo | todo | T-03.1 | 2026-10-28 |
| 20 | [T-03.3](T-03_core-domain-and-application/T-03.3_building-an-event.md) | Building an event | code | Todo | todo | T-03.2 | 2026-10-28 |
| 21 | [T-03.4](T-03_core-domain-and-application/T-03.4_masking-and-redaction.md) | Masking and redaction | code | Todo | todo | T-03.3 | 2026-10-28 |
| 22 | [T-03.5](T-03_core-domain-and-application/T-03.5_limits.md) | Limits | code | Todo | todo | T-03.3 | 2026-10-28 |
| 23 | [T-03.6](T-03_core-domain-and-application/T-03.6_configuration-and-profiles.md) | Configuration and profiles | code | Todo | todo | T-02.4 | 2026-10-28 |
| 24 | [T-03.7](T-03_core-domain-and-application/T-03.7_error-model.md) | Error model | code | Todo | todo | T-02.3 | 2026-10-28 |
| 25 | [T-03.8](T-03_core-domain-and-application/T-03.8_call-context.md) | Call context | code | Todo | todo | T-03.2 | 2026-10-28 |
| 26 | [T-03.9](T-03_core-domain-and-application/T-03.9_core-composition-root-and-the-public-entry-point.md) | Core composition root and the public entry point | code | Todo | todo | T-03.2 to T-03.8 | 2026-10-28 |
| 27 | [T-04](T-04_logging-and-core-adapters/T-04_logging-and-core-adapters.md) | Logging and the core adapters | epic | Todo | todo | T-03 | 2026-10-28 |
| 28 | [T-04.1](T-04_logging-and-core-adapters/T-04.1_logger-and-the-log-entry-recorded-event.md) | Logger and the `log.entry.recorded` event | code | Todo | todo | T-03.9 | 2026-10-28 |
| 29 | [T-04.2](T-04_logging-and-core-adapters/T-04.2_console-adapter.md) | Console adapter | code | Todo | todo | T-04.1 | 2026-10-28 |
| 30 | [T-04.3](T-04_logging-and-core-adapters/T-04.3_file-adapter.md) | File adapter | code | Todo | todo | T-04.1 | 2026-10-28 |
| 31 | [T-04.4](T-04_logging-and-core-adapters/T-04.4_shared-http-infrastructure.md) | Shared HTTP infrastructure | code | Todo | todo | T-03.5, T-03.6 | 2026-10-28 |
| 32 | [T-04.5](T-04_logging-and-core-adapters/T-04.5_http-destination.md) | HTTP destination | code | Todo | todo | T-04.4 | 2026-10-28 |
| 33 | [T-04.6](T-04_logging-and-core-adapters/T-04.6_aws-http-adapter.md) | AWS HTTP adapter | code | Todo | todo | T-04.4 | 2026-10-28 |
| 34 | [T-04.7](T-04_logging-and-core-adapters/T-04.7_replace-and-remove-the-old-library.md) | Replace and remove the old library | code | Todo | todo | T-04.1 to T-04.6 | 2026-10-28 |
| 35 | [T-04.8](T-04_logging-and-core-adapters/T-04.8_fake-receiver-for-tests.md) | Fake receiver for tests | test | Todo | todo | T-04.4 | 2026-10-28 |
| 36 | [T-05](T-05_delivery/T-05_delivery.md) | Delivery, buffering and failures | epic | Todo | todo | T-04 | 2026-10-28 |
| 37 | [T-05.1](T-05_delivery/T-05.1_bounded-queue-and-worker.md) | Bounded queue and worker | code | Todo | todo | T-04.5 | 2026-10-28 |
| 38 | [T-05.2](T-05_delivery/T-05.2_backpressure-and-priorities.md) | Backpressure and priorities | code | Todo | todo | T-05.1 | 2026-10-28 |
| 39 | [T-05.3](T-05_delivery/T-05.3_delivery-modes-best-effort-and-retry.md) | Delivery modes `best_effort` and `retry` | code | Todo | todo | T-05.1 | 2026-10-28 |
| 40 | [T-05.4](T-05_delivery/T-05.4_flush-and-shutdown.md) | `flush` and `shutdown` | code | Todo | todo | T-05.1 | 2026-10-28 |
| 41 | [T-05.5](T-05_delivery/T-05.5_diagnostics.md) | Diagnostics | code | Todo | todo | T-05.1 | 2026-10-28 |
| 42 | [T-05.6](T-05_delivery/T-05.6_failure-matrix-tests.md) | Failure-matrix tests | test | Todo | todo | T-05.3, T-05.5, T-04.8 | 2026-10-28 |
| 43 | [T-06](T-06_spool-and-sqlite-destination/T-06_spool-and-sqlite-destination.md) | The spool (`durable` mode) and the SQLite destination | epic | Todo | todo | T-05 | 2026-10-28 |
| 44 | [T-06.1](T-06_spool-and-sqlite-destination/T-06.1_layouts-in-contracts-storage.md) | Layouts in `contracts/storage/` | docs | Todo | todo | T-02.1, T-02.2 | 2026-10-28 |
| 45 | [T-06.2](T-06_spool-and-sqlite-destination/T-06.2_choose-the-go-driver.md) | Choose the Go driver | decision | Awaiting the maintainer | awaiting-user | T-06.1 | 2026-10-28 |
| 46 | [T-06.3](T-06_spool-and-sqlite-destination/T-06.3_spool-adapter-in-go.md) | Spool adapter in Go | code | Todo | todo | T-06.1, T-06.2 | 2026-10-28 |
| 47 | [T-06.4](T-06_spool-and-sqlite-destination/T-06.4_spool-adapter-in-python.md) | Spool adapter in Python | code | Todo | todo | T-06.1 | 2026-10-28 |
| 48 | [T-06.5](T-06_spool-and-sqlite-destination/T-06.5_durable-mode.md) | `durable` mode | code | Todo | todo | T-06.3, T-06.4, T-05.3 | 2026-10-28 |
| 49 | [T-06.6](T-06_spool-and-sqlite-destination/T-06.6_sqlite-destination-in-go-and-python.md) | SQLite destination in Go and Python | code | Todo | todo | T-06.1, T-05.3, T-06.2 | 2026-10-28 |
| 50 | [T-06.7](T-06_spool-and-sqlite-destination/T-06.7_cross-tests-of-the-files.md) | Cross tests of the files | test | Todo | todo | T-06.5, T-06.6 | 2026-10-28 |
| 51 | [T-07](T-07_executions-faults-reports-and-health/T-07_executions-faults-reports-and-health.md) | Executions, faults, reports and health | epic | Todo | todo | T-03, T-05 | 2026-10-28 |
| 52 | [T-07.1](T-07_executions-faults-reports-and-health/T-07.1_execution-model-and-transitions.md) | Execution model and transitions | code | Todo | todo | T-03.1 | 2026-10-28 |
| 53 | [T-07.2](T-07_executions-faults-reports-and-health/T-07.2_execution-events-and-api.md) | `execution.*` events and API | code | Todo | todo | T-07.1, T-03.3, T-03.9 | 2026-10-28 |
| 54 | [T-07.3](T-07_executions-faults-reports-and-health/T-07.3_faults-and-generic-reports.md) | Faults and generic reports | code | Todo | todo | T-02.3, T-03.3, T-03.9 | 2026-10-28 |
| 55 | [T-07.4](T-07_executions-faults-reports-and-health/T-07.4_health-and-heartbeats.md) | Health and heartbeats | code | Todo | todo | T-03.9 | 2026-10-28 |
| 56 | [T-07.5](T-07_executions-faults-reports-and-health/T-07.5_cross-tests.md) | Cross tests | test | Todo | todo | T-07.1 to T-07.4 | 2026-10-28 |
| 57 | [T-08](T-08_devices/T-08_devices.md) | Devices and hardware | epic | Todo | todo | T-02, T-03, T-05, T-07 | 2026-10-28 |
| 58 | [T-08.1](T-08_devices/T-08.1_device-schemas-and-enums.md) | Device schemas and enums | code | Todo | todo | T-02.2 | 2026-10-28 |
| 59 | [T-08.2](T-08_devices/T-08.2_source-helpers.md) | Source helpers | code | Todo | todo | T-08.1, T-03.9, T-07.2 | 2026-10-28 |
| 60 | [T-08.3](T-08_devices/T-08.3_sampling-and-batching.md) | Sampling and batching | code | Todo | todo | T-08.2, T-05.1 | 2026-10-28 |
| 61 | [T-08.4](T-08_devices/T-08.4_command-correlation.md) | Command correlation | code | Todo | todo | T-08.2 | 2026-10-28 |
| 62 | [T-08.5](T-08_devices/T-08.5_fixtures-from-annex-b.md) | Fixtures from Annex B | test | Todo | todo | T-08.1, T-02.8 | 2026-10-28 |
| 63 | [T-08.6](T-08_devices/T-08.6_raspberry-pi-pilot.md) | Raspberry Pi pilot | test | Todo | todo | T-08.2, T-06.5 | 2026-10-28 |
| 64 | [T-08.7](T-08_devices/T-08.7_edit-the-examples-with-the-real-projects.md) | Edit the examples with the real projects | docs | Todo | todo | T-08.6 | 2026-10-28 |
| 65 | [T-09](T-09_ai-agents/T-09_ai-agents.md) | AI agents | epic | Todo | todo | T-02, T-03, T-05, T-07 | 2026-10-28 |
| 66 | [T-09.1](T-09_ai-agents/T-09.1_ai-schemas-and-enums.md) | AI schemas and enums | code | Todo | todo | T-02.2 | 2026-10-28 |
| 67 | [T-09.2](T-09_ai-agents/T-09.2_generic-callbacks.md) | Generic callbacks | code | Todo | todo | T-09.1, T-07.2 | 2026-10-28 |
| 68 | [T-09.3](T-09_ai-agents/T-09.3_privacy-of-prompts.md) | Privacy of prompts | code | Todo | todo | T-09.2, T-03.4 | 2026-10-28 |
| 69 | [T-09.4](T-09_ai-agents/T-09.4_hosted-model-providers.md) | Hosted model providers | code | Todo | todo | T-09.2 | 2026-10-28 |
| 70 | [T-09.5](T-09_ai-agents/T-09.5_check-langfuse-support-in-go.md) | Check Langfuse support in Go | decision | Todo | todo | T-09.1 | 2026-10-28 |
| 71 | [T-09.6](T-09_ai-agents/T-09.6_langfuse-adapter.md) | Langfuse adapter | code | Todo | todo | T-09.2, T-09.5 | 2026-10-28 |
| 72 | [T-09.7](T-09_ai-agents/T-09.7_langchain-adapter.md) | LangChain adapter | code | Todo | todo | T-09.2 | 2026-10-28 |
| 73 | [T-09.8](T-09_ai-agents/T-09.8_llamaindex-adapter.md) | LlamaIndex adapter | code | Todo | todo | T-09.2 | 2026-10-28 |
| 74 | [T-09.9](T-09_ai-agents/T-09.9_agent-pilot.md) | Agent pilot | test | Todo | todo | T-09.6, T-06.5 | 2026-10-28 |
| 75 | [T-10](T-10_metrics-and-traces/T-10_metrics-and-traces.md) | Metrics and traces | epic | Todo | todo (T-10.4 and T-10.5 deferred) | T-03, T-05 | 2026-10-28 |
| 76 | [T-10.1](T-10_metrics-and-traces/T-10.1_metrics-api.md) | Metrics API | code | Todo | todo | T-03.9 | 2026-10-28 |
| 77 | [T-10.2](T-10_metrics-and-traces/T-10.2_traces-api.md) | Traces API | code | Todo | todo | T-03.9 | 2026-10-28 |
| 78 | [T-10.3](T-10_metrics-and-traces/T-10.3_w3c-trace-context.md) | W3C Trace Context | code | Todo | todo | T-10.2 | 2026-10-28 |
| 79 | [T-10.4](T-10_metrics-and-traces/T-10.4_opentelemetry-adapter.md) | OpenTelemetry adapter | code | Deferred | deferred | T-10.1, T-10.2 | 2026-10-28 |
| 80 | [T-10.5](T-10_metrics-and-traces/T-10.5_prometheus-adapter.md) | Prometheus adapter | code | Deferred | deferred | T-10.1 | 2026-10-28 |
| 81 | [T-10.6](T-10_metrics-and-traces/T-10.6_cross-tests.md) | Cross tests | test | Todo | todo | T-10.3 | 2026-10-28 |
| 82 | [T-11](T-11_audit/T-11_audit.md) | Audit events | epic | Todo | todo (T-11.4 deferred) | T-03, T-05 | 2026-10-28 |
| 83 | [T-11.1](T-11_audit/T-11.1_audit-schemas.md) | Audit schemas | code | Todo | todo | T-02.2 | 2026-10-28 |
| 84 | [T-11.2](T-11_audit/T-11.2_separate-routing.md) | Separate routing | code | Todo | todo | T-11.1, T-05.2 | 2026-10-28 |
| 85 | [T-11.3](T-11_audit/T-11.3_the-warning.md) | The warning | docs | Todo | todo | T-11.2 | 2026-10-28 |
| 86 | [T-11.4](T-11_audit/T-11.4_audit-adapter.md) | Audit adapter | code | Deferred | deferred | T-11.2 | 2026-10-28 |
| 87 | [T-12](T-12_cross-language-conformance/T-12_cross-language-conformance.md) | Cross-language conformance | epic | Todo | todo (recurring: it grows with every capability) | T-02.5 and, for each check, the capability it checks | 2026-10-28 |
| 88 | [T-12.1](T-12_cross-language-conformance/T-12.1_cross-test-runner.md) | Cross-test runner | test | Todo | todo | T-02.8 | 2026-10-28 |
| 89 | [T-12.2](T-12_cross-language-conformance/T-12.2_registry-gate.md) | Registry gate | code | Todo | todo | T-02.7, T-12.1 | 2026-10-28 |
| 90 | [T-12.3](T-12_cross-language-conformance/T-12.3_mechanical-parity-check.md) | Mechanical parity check | test | Todo | todo | T-01.4 | 2026-10-28 |
| 91 | [T-12.4](T-12_cross-language-conformance/T-12.4_maturity-report.md) | Maturity report | code | Todo | todo | T-12.2 | 2026-10-28 |
| 92 | [T-13](T-13_documentation-examples-and-release/T-13_documentation-examples-and-release.md) | Documentation, examples and release | epic | Todo | todo (T-13.1 is recurring: it runs after every epic) | T-01 and, for the release, T-02 to T-12 | 2026-10-28 |
| 93 | [T-13.1](T-13_documentation-examples-and-release/T-13.1_documentation-sync-recurring.md) | Documentation sync (recurring) | docs | Todo | todo | T-01 to T-12 (after each epic) | 2026-10-28 |
| 94 | [T-13.2](T-13_documentation-examples-and-release/T-13.2_examples.md) | Examples | docs | Todo | todo | T-04, T-07 to T-11 (each capability) | 2026-10-28 |
| 95 | [T-13.3](T-13_documentation-examples-and-release/T-13.3_security-checklist.md) | Security checklist | docs | Todo | todo | T-03.4 | 2026-10-28 |
| 96 | [T-13.4](T-13_documentation-examples-and-release/T-13.4_releases-0x.md) | Releases `0.x` | code | Todo | todo | T-01.6 | 2026-10-28 |
| 97 | [T-13.5](T-13_documentation-examples-and-release/T-13.5_the-1-0-check.md) | The `1.0` check | decision | Awaiting the maintainer | awaiting-user | T-12.4 | 2026-10-28 |
| 98 | [T-13.6](T-13_documentation-examples-and-release/T-13.6_note-for-the-old-consumer.md) | Note for the old consumer | docs | Todo | todo | T-04.7 | 2026-10-28 |

## Why this order

1. **T-01 and T-02 first.** The layout and CI come before any code, and `contracts/` with the generator is what both languages depend on (REQ-CNF-001, REQ-CNF-002). T-01.1 is the maintainer's acceptance of PR-001.
2. **T-03 then T-04.** The shared core is built once in both languages, then logging and the first adapters on top of it. The old library stays until T-04.7 removes it.
3. **T-05 and T-06 before the capabilities.** Devices and AI agents need the queue, the delivery modes and the spool, so those come first (Q07-16).
4. **T-07 before T-08 and T-09.** Both reuse `execution.*`, `fault.*` and `report.*`. T-08 and T-09 do not depend on each other and can run in parallel; each has a pilot that needs the maintainer.
5. **T-10 and T-11 last of the capabilities.** There is no metrics or tracing backend and no audit need today, so their adapters are deferred.
6. **T-12 and T-13 are recurring.** The cross-language tests grow with every capability, and the documentation is synchronized after every epic; the `1.0` decision (T-13.5) is last.
