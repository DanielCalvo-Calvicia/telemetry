# capabilities

The registry of capabilities ([registry.yaml](registry.yaml), [T-02.7](../../docs/tasks/T-02_contract-and-generator/T-02.7_capabilities-registry-and-its-check.md), REQ-CNF-007). A capability is a unit of the library that is built, tested and released as a whole: contract, architecture, configuration, logging, delivery, spool, outbound adapters, executions, health, faults and reports, devices, ai, ai frameworks, metrics, traces, security and audit. Every `REQ-*` identifier that the specification ([PR-001](../../docs/proposals/PR-001-telemetry-standard-go-python.md)) defines is listed by exactly one capability, with the conformance tests that cover it or the task that will write them.

The registry is checked by [tools/registry_check](../../tools/registry_check/README.md) and CI fails when a rule below is broken. The file is data: examples and descriptions use placeholders and describe what is built or planned, never an API that does not exist.

## The format

```yaml
registry_version: 1
capabilities:
  - id: logging
    title: "Structured logging"
    level: beta
    status: planned
    description: "..."
    public_api: ["planned: ..."]
    schemas: ["contracts/schemas/log/entry_recorded.schema.json"]
    configuration: []
    errors: []
    ports: []
    adapters: []
    languages:
      go: { state: todo }
      python: { state: not_applicable, reason: "Why it never applies." }
    requirements:
      - id: REQ-LOG-001
        tests:
          - { language: go, path: go/internal/tests/.../some_test.go }
      - id: REQ-LOG-002
        pending: T-04.1
```

| Field | Required | Meaning |
|---|---|---|
| `registry_version` | yes | The version of this format. It is `1`; the check refuses any other value. |
| `id` | yes | The stable identifier of the capability: lowercase `snake_case`. Unique in the file. The event registry (`contracts/events/registry.yaml`) refers to it in its `capability` field. |
| `title` | yes | A short name for people. Not empty. |
| `level` | yes | `stable` or `beta` (section 13.3 of the specification). |
| `status` | yes | `supported`, `in_progress` or `planned`. See below. |
| `description` | yes | What the capability is for and what of it is built. Not empty. |
| `public_api` | yes | A list of strings: the public API of the capability, written as `built: ...` or `planned: ...`. May be empty. |
| `schemas` | yes | A list of strings: the schemas of `contracts/` that belong to the capability. May be empty. |
| `configuration` | yes | A list of strings: the configuration options of the capability. May be empty. |
| `errors` | yes | A list of strings: the error codes and semantics of the capability. May be empty. |
| `ports` | yes | A list of strings: the ports of the capability. May be empty. |
| `adapters` | yes | A list of strings: the adapters of the capability. May be empty. |
| `languages.go`, `languages.python` | yes | The state of each implementation: `state` and, for `not_applicable`, a `reason`. |
| `requirements` | yes | A non-empty list of the requirements the capability owns, with their evidence. |

The six lists are descriptions for people: the check does not parse them. Write what is built or what is planned; do not invent an API.

### The state of a language

| State | Meaning |
|---|---|
| `todo` | The language has not started the capability. |
| `built` | The language implements part or all of the capability, and its conformance tests do not all pass yet. |
| `passing` | Every conformance test of the capability passes in the language. |
| `not_applicable` | The capability does not exist in that language and never will, for example the LangChain and LlamaIndex adapters in Go (REQ-AI-011, Q07-14). A `reason` that is not empty is **required**; a test of that language in the capability is an error. |

`reason` is read only for `not_applicable`; for any other state it is ignored.

### The status of a capability

| Status | Meaning | Rule the check applies |
|---|---|---|
| `planned` | Not started. | none |
| `in_progress` | Being built. | none |
| `supported` | Released as working in both languages (REQ-CNF-007). | Each language is `passing`, or `not_applicable` with a reason, and at least one is `passing`; and **every** requirement of the capability has `tests` that exist. A `pending` requirement or a `todo` or `built` language makes it an error. |

The overall maturity of the library is the one of its least complete capability (REQ-CNF-008): the lowest `level`, then the lowest `status`, then the smallest share of requirements covered by tests. The check prints it. While any capability is `beta` or not `supported` the library is beta (Q09-6).

### A requirement entry

Each entry is `id` plus exactly one of:

- `tests`: a non-empty list of `{ language, path }` with `language` `go` or `python` and `path` the slash path, relative to the root of the repository, of a test file that exists (a file, not a folder). The requirement is **covered**. Link only a test that really exercises the requirement.
- `pending: T-04.7`: the task that will write the test. The task must exist in `docs/tasks/`. The requirement is **pending**, never counted as covered.
- `pending: unassigned`: no task mentions the requirement yet. It is allowed so the check can run from the first day, but it is listed apart as a gap of the plan (`no task yet:`) and never counted as covered. Replace it with a task when one is written.

An entry with neither, or with both, is an error.

## The rules the check applies

1. Every `REQ-*` the specification defines is listed by a capability; every listed id is defined in the specification.
2. **A requirement is listed once, in one capability only.** A second entry, in the same or in another capability, is an error.
3. Every requirement has `tests` or `pending`, never both and never neither.
4. Every listed test file exists and the paths are clean, relative and forward-slashed.
5. Every `pending` task exists, or the value is `unassigned`.
6. `level`, `status` and every `state` are among the values above; a `not_applicable` has a reason; a capability id is unique and `snake_case`.
7. A `supported` capability meets the rule above (REQ-CNF-007).
8. An unknown key anywhere in the file is an error, so a misspelled field is never ignored.

## Where the identifiers come from

From the specification, not from `docs/requirements/requirements.md` (which lists only a part of them): every bold paragraph start `**REQ-XXX-nnn.**` of PR-001. The path of the source is a flag of the check, so it can change later.

## How to change it

Add a capability or a requirement entry in the same change that adds the code or the test; replace `pending` with `tests` when the test exists and move the language to `built` or `passing`; never mark a capability `supported` by hand without running the check. Run `cd tools/registry_check && go run . --spec ../../docs/proposals/PR-001-telemetry-standard-go-python.md --registry ../../contracts/capabilities/registry.yaml --root ../.. --tasks ../../docs/tasks`.
