# Documentation of telemetry

State of the documents: 2026-10-27. Start with [architecture](architecture/architecture.md) for the structure and [api](api/api.md) to use the library.

| Document | What it answers |
|---|---|
| [architecture/architecture.md](architecture/architecture.md) | the layers, the packages, the data flow of one log call, the two ways to build a logger |
| [api/api.md](api/api.md) | the public API of `pkg/logging` and the contract the AWS HTTP sink sends |
| [configuration/environment-variables.md](configuration/environment-variables.md) | every `LOGGER_*` variable, its default and what happens when a value is missing |
| [decisions/](decisions/) | the decision records ADR-001 to ADR-010 |
| [requirements/requirements.md](requirements/requirements.md) | what the library must do, and the state of each requirement |
| [security/security.md](security/security.md) | masking, what the library holds, the checklist before anything is pushed to the public repository |
| [operations/monitoring.md](operations/monitoring.md) | what a consumer can observe, the failure report, timeouts, the manual end-to-end check |
| [deployment/deployment.md](deployment/deployment.md) | there is no deployment: build, CI, releases by tag and how a consumer takes one |
| [proposals/](proposals/) | the proposals that are not built yet |

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

## Proposals

| Proposal | Status |
|---|---|
| [PR-001](proposals/PR-001-telemetry-standard-go-python.md) | the new telemetry standard (one contract for Go and Python, device and AI modules). Proposed, draft v0.1, nothing built |

## Not written

The `setup/` intake forms and questionnaires of the Engram context (the numbered sub-forms and the Q&A with the maintainer) and the `tasks/` board of dynamics_microservice have no equivalent here yet. They are tied to the Engram intake process and to a task plan this library does not have; they would be written when the library is enrolled in Engram Cloud.
