# Documentation of telemetry

State of the documents: 2026-10-27. Start with [architecture](architecture/architecture.md) for the structure and [api](api/api.md) to use the library.

| Document | What it answers |
|---|---|
| [architecture/architecture.md](architecture/architecture.md) | the layers, the packages, the data flow of one log call, the two ways to build a logger |
| [api/api.md](api/api.md) | the public API of `pkg/logging` and the contract the AWS HTTP sink sends |
| [configuration/environment-variables.md](configuration/environment-variables.md) | every `LOGGER_*` variable, its default and what happens when a value is missing |
| [decisions/](decisions/) | the decision records ADR-001 to ADR-013 |
| [requirements/requirements.md](requirements/requirements.md) | what the library must do, and the state of each requirement |
| [security/security.md](security/security.md) | masking, what the library holds, the checklist before anything is pushed to the public repository |
| [operations/monitoring.md](operations/monitoring.md) | what a consumer can observe, the failure report, timeouts, the manual end-to-end check |
| [deployment/deployment.md](deployment/deployment.md) | there is no deployment: build, CI, releases by tag and how a consumer takes one |
| [proposals/](proposals/) | the proposals that are not built yet |
| [tasks/](tasks/README.md) | the task plan to build the new standard: 13 epics, 85 subtasks, the order and the board |
| [setup/questionnaires/](setup/questionnaires/) | the questions to the maintainer that settle the open decisions of a proposal |

## Decisions

| ADR | Decision |
|---|---|
| [ADR-001](decisions/ADR-001-adopt-shared-go-standard.md) | adopt the shared Go Clean/Hexagonal standard |
| [ADR-002](decisions/ADR-002-one-module-capabilities-side-by-side.md) | one module, capabilities side by side |
| [ADR-003](decisions/ADR-003-public-surface-is-pkg-logging.md) | the public surface is `pkg/logging`, by aliases |
| [ADR-004](decisions/ADR-004-library-never-reads-the-environment.md) | the library never reads the environment |
| [ADR-005](decisions/ADR-005-a-failing-sink-never-fails-the-caller.md) | a failing sink never fails the caller |
| [ADR-006](decisions/ADR-006-masking-before-every-sink.md) | masking happens before every sink |
| [ADR-007](decisions/ADR-007-minimum-level-follows-the-environment.md) | the minimum level follows the environment |
| [ADR-008](decisions/ADR-008-aws-http-sink-adapts-to-add-log.md) | the AWS HTTP sink adapts the entry to the `add_log` contract |
| [ADR-009](decisions/ADR-009-ready-made-sink-constructors.md) | ready-made constructors for each sink |
| [ADR-010](decisions/ADR-010-no-deployment-release-by-tags.md) | no deployment; release by tags |
| [ADR-011](decisions/ADR-011-adopt-the-telemetry-standard-go-python.md) | adopt the telemetry standard, one contract for Go and Python |
| [ADR-012](decisions/ADR-012-two-times-in-every-event.md) | two times in every event, generated and recorded |
| [ADR-013](decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md) | one Go module per adapter, chosen in code |

## Proposals

| Proposal | Status |
|---|---|
| [PR-001](proposals/PR-001-telemetry-standard-go-python.md) | the new telemetry standard (one contract for Go and Python, device and AI modules). Accepted on 2026-10-28 (version 0.2), nothing built |

## Questionnaires

| Questionnaire | Settles |
|---|---|
| [Q01](setup/questionnaires/Q01_scope-repository-layout-and-rollout.md) | scope, repository layout, rollout across the projects |
| [Q02](setup/questionnaires/Q02_contract-format-identifiers-and-generation.md) | D-02, D-04, code generation, first event types |
| [Q03](setup/questionnaires/Q03_delivery-receiver-and-spool.md) | D-01, the HTTP receiver, delivery defaults |
| [Q04](setup/questionnaires/Q04_devices-and-hardware.md) | D-03, the device module (D-05 closed: no hardware drivers) |
| [Q05](setup/questionnaires/Q05_ai-agents-and-langfuse.md) | D-06, D-09, the AI module |
| [Q06](setup/questionnaires/Q06_security-configuration-and-release.md) | D-07, D-08, configuration, security, releases |
| [Q07](setup/questionnaires/Q07_open-points-after-the-answers.md) | the points the answers to Q01-Q06 left open: Go module path, Python version, parity, receiver, spool quota |
| [Q08](setup/questionnaires/Q08_last-points-before-rewriting-the-proposal.md) | the last points before PR-001 is rewritten: the two times, public names, Go modules, SQLite, HTTP contract, limits |
| [Q09](setup/questionnaires/Q09_adapters-modules-and-v1-meaning.md) | the conflicts between the answers: how adapters are chosen, the Go modules, the SQLite file, what 1.0 means |

Q01 to Q09 were asked and answered on 2026-10-27 and 2026-10-28 about [PR-001](proposals/PR-001-telemetry-standard-go-python.md) (Q08 questions 1 and 10 moved to Q09). Their decisions are written into PR-001 version 0.2 and into ADR-011 to ADR-013.

## Not written

The `setup/` intake forms of the Engram context (the numbered sub-forms) have no equivalent here yet. They are tied to the Engram intake process and would be written when the library is enrolled in Engram Cloud. The task plan of the new standard is in [tasks/](tasks/README.md).
