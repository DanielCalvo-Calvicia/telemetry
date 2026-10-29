# registry_check

The check of the registry of capabilities of [T-02.7](../../docs/tasks/T-02_contract-and-generator/T-02.7_capabilities-registry-and-its-check.md): it reads [contracts/capabilities/registry.yaml](../../contracts/capabilities/README.md) and fails when a requirement of the specification has no conformance test and no pending task (REQ-CNF-007). It is a Go module of its own, like the [contract generator](../contract_generator/README.md), so its only dependency (`gopkg.in/yaml.v3`) never reaches the core module (REQ-ARC-005). It is not part of the library: a consumer never imports it.

## Run

```bash
cd tools/registry_check && go run . --spec ../../docs/proposals/PR-001-telemetry-standard-go-python.md --registry ../../contracts/capabilities/registry.yaml --root ../.. --tasks ../../docs/tasks
```

| Flag | Default (relative to the working folder) | Meaning |
|---|---|---|
| `--spec` | `docs/proposals/PR-001-telemetry-standard-go-python.md` | The file that defines the `REQ-*` identifiers. |
| `--registry` | `contracts/capabilities/registry.yaml` | The registry to check. |
| `--root` | `.` | The root of the repository: the test paths of the registry are resolved against it. |
| `--tasks` | `docs/tasks` | The folder of the tasks: a `pending: T-04.7` must have its file there. |

CI passes all four explicitly (job `registry-check`). Exit code `0`: no rule is broken; `1`: a rule is broken or a file cannot be read; `2`: wrong arguments.

## The source of the identifiers

The specification, not [docs/requirements/requirements.md](../../docs/requirements/requirements.md): that table lists only a part of the requirements. The check reads every line that starts with a bold identifier, matched by `(?m)^\*\*(REQ-[A-Z]{2,4}-[0-9]{3})\.\*\*`, deduplicates them and sorts them. A mention in the middle of a line, a quoted or indented one, and an identifier without the final dot are not definitions. A specification that defines none is an error (a wrong `--spec` must not pass). The path is a flag so the source can change later.

## The rules

The rules, the states and the fields are documented in [contracts/capabilities/README.md](../../contracts/capabilities/README.md). In short: every identifier of the specification is listed by exactly one capability; every requirement has `tests` (files that exist) or `pending` (a task that exists, or `unassigned`); a `not_applicable` language has a reason; a `supported` capability has both languages `passing` (or one `not_applicable` with a reason) and every requirement tested; the level, the status and the states have allowed values; ids are not duplicated. The check does not stop at the first breach: it reports all of them.

## The output

On success and on failure it prints, to the standard output:

1. the registry and the specification, how many identifiers the specification defines and how many per area;
2. the counts: total, covered by tests, pending, no task yet, not accounted for (listed nowhere or without valid evidence);
3. a table of capabilities: level, status, the state of each language and the requirements covered of the total;
4. the requirements that wait for a task, grouped by area, one per line: `REQ-ARC-014 (ARC): pending T-03.9`;
5. a separate section `no task yet:` with the `unassigned` ones, the real gaps of the plan;
6. `library maturity: beta (least complete capability: ...)` (REQ-CNF-008), and `result: PASSED` or `result: FAILED`.

Every broken rule goes to the standard error, one per line, starting with the registry file and naming the entry: `registry.yaml: capability "logging": requirement REQ-LOG-001: write tests or pending: requirement without tests and without pending`.

## Layout

The layers of the generator: `internal/domain` (the model, the rules and the report, no I/O), `internal/application` (the ports and the use case), `internal/infrastructure` (`inbound/cli`, and `outbound/` with `registry_file`, `spec_file`, `test_files` and `task_files`, whose I/O lives in their `external/` folders), `internal/composition_root` (the only package that imports the concrete adapters) and a tiny `main.go`.

## Tests

```bash
cd tools/registry_check && go test ./... -count=1
```

Black-box, mirroring the code under `internal/tests/`: the fixtures under `internal/tests/fixtures/registry/` hold a small specification (three requirements), a fixture root with test files, a fixture tasks folder and one registry per case (a valid one, an uncovered requirement, a missing test file, a supported capability with a language in `todo`, a `not_applicable` without and with a reason, an unknown requirement, a requirement no capability lists, a pending task that does not exist, `pending: unassigned`, invalid values, duplicates, broken YAML). One test runs the check on the **real** registry, specification and tasks of the repository and expects it to pass; it is skipped, with a message, if the root of the repository (`go.work`) is not found.
