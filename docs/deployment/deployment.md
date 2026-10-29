# Deployment

The library is **not deployed** ([ADR-010](../decisions/ADR-010-no-deployment-release-by-tags.md)): there is no image, no manifest, no pipeline that touches a cluster. This document describes what exists instead: the build and checks, the CI, how a release is made and how a consumer takes it.

## Environments

None. The library runs inside the consumer's process, in the consumer's environment.

## Tools and versions

| Tool | Version |
|---|---|
| Go | 1.25.5 or newer (`go.mod`) |
| Third-party dependencies | `github.com/google/uuid` only |

## Build and checks

```bash
gofmt -l .                 # prints nothing
go vet ./...
go build ./...
go test ./... -count=1
go test ./... -coverpkg=./internal/... -coverprofile=coverage.out   # the coverage gate of CI
```

These commands run in the root module (the old library). The new Go core is a module of its own: run the same commands inside `go/` (`cd go`). `go.work` at the root joins the two modules for local work (`go work sync`, `go list -m all`) and is committed (ADR-013). The pipeline runs both modules and the Python package (see "Pipeline" below).

The architecture tests of the new core run with the others (`go test ./...` inside `go/`; `python -m unittest discover ...` in `python/`); they are part of the `go` and `python` jobs of the pipeline. They check the direction of the dependencies: see the architecture document, section 2.2.

The Python package is checked in a virtual environment, from `python/`:

```bash
pip install .                                                    # the package, with no third-party dependency
python -m unittest discover -s internal/tests -t .               # the tests
coverage run -m unittest discover -s internal/tests -t . && coverage report   # at least 95%
ruff check . && ruff format --check .                            # lint and format (development only)
```

A Python project installs the library from the Git repository: `pip install "telemetry[sqlite] @ git+https://github.com/DanielCalvo-Calvicia/telemetry@python-vX.Y.Z#subdirectory=python"` (no tag exists yet; the first one is T-13.4).

The race detector (`-race`) needs cgo and a C compiler; it was not run on the maintainer's Windows machine (no `gcc`).

## Pipeline (`.github/workflows/ci.yml`)

Runs on a push to `main` and on every pull request, with a read-only `GITHUB_TOKEN` (`permissions: contents: read`); every action is pinned by commit SHA (`actions/checkout` v4.2.2, `actions/setup-go` v5.5.0, `actions/setup-python` v5.6.0). A push to any other branch runs nothing until a pull request is opened. Six jobs (T-01.5, T-02.4 and T-02.7, 2026-10-28; written and run locally, **not yet run on GitHub**):

| Job | What it does |
|---|---|
| `modules` | reads `go.work` (`go work edit -json`) and lists its modules: today `.` (the old library), `./go` (the new core) , `./tools/contract_generator` and `./tools/registry_check`; a module added to `go.work` is checked without editing the file |
| `go` (one per module) | in the module folder: `gofmt -l .` and `goimports -l .` (`golang.org/x/tools/cmd/goimports@v0.35.0`) print nothing; `go vet ./...`; `go build ./...`; `go test ./... -count=1 -coverpkg=./internal/... -coverprofile=coverage.out`; the coverage gate: the total over `internal/`, **excluding the `external/` folders**, must be at least **95%** (measured locally on 2026-10-29: 99.0% for `go/`, 98.7% for the generator, 98.7% for the registry check, 98.0% for the old module without its e2e package) |
| `python` (3.10 and 3.11) | in `python/`: `pip install . coverage[toml]==7.17.0 ruff==0.17.3`; `ruff check .` and `ruff format --check .`; `coverage run -m unittest discover -s internal/tests -t .` and `coverage report`, whose gate is `fail_under = 95` with `omit */external/*` in `pyproject.toml`. 3.10 is the declared minimum and 3.11 is the version of the Raspberry Pi of the pilot |
| `generated-code` | the generated code is committed (REQ-DST-005): it runs the generator on `contracts/` (`cd tools/contract_generator && go run . --contracts ../../contracts --go-out ../../go/internal/domain/value_objects --python-out ../../python/src/telemetry/internal/domain/value_objects --go-validation-out ../../go/internal/domain/operations/event_validation --python-validation-out ../../python/src/telemetry/internal/domain/operations/event_validation`) and fails on `git diff --exit-code` or if `git status --porcelain` shows a new file, so a contract change without its generated code, or a hand edit of a generated file, cannot be merged |
| `registry-check` | runs `tools/registry_check` on the specification, the registry, the repository and the tasks (`cd tools/registry_check && go run . --spec ../../docs/proposals/PR-001-telemetry-standard-go-python.md --registry ../../contracts/capabilities/registry.yaml --root ../.. --tasks ../../docs/tasks`): it fails if a requirement has neither a conformance test nor a pending task, if a listed test file or task does not exist, or if a capability is marked `supported` without both languages passing; it prints the uncovered requirements and the library maturity |
| `cross-tests` | passes while empty (it only prints a line); T-02.8 creates the first Go and Python fixtures and T-12 the runner (REQ-CNF-006) |

To regenerate after a change of `contracts/`: `cd tools/contract_generator && go run . --contracts ../../contracts --go-out ../../go/internal/domain/value_objects --python-out ../../python/src/telemetry/internal/domain/value_objects --go-validation-out ../../go/internal/domain/operations/event_validation --python-validation-out ../../python/src/telemetry/internal/domain/operations/event_validation`, then commit the generated files with the change. The race detector is not run (it needs cgo). The e2e test of the deployed service is skipped in CI: its switch is `false` in what is committed. The old module keeps its job until T-04.7 removes it.

## Releases

| Tag | Date | Contents |
|---|---|---|
| `v0.1.0` | 2026-10-19 | logging: console and HTTP sinks, masking, call context, `pkg/logging` |
| `v0.2.0` | 2026-10-20 | the AWS HTTP sink, the ready-made sink constructors and `NewWithSinks`, the end-to-end tests |

These are the only tags that exist. They belong to the old library, whose module path is `github.com/DanielCalvo-Calvicia/telemetry`, and they are never moved.

### The rule for the new layout (written in T-01.6, 2026-10-28; nothing is tagged yet)

The new standard ([PR-001](../proposals/PR-001-telemetry-standard-go-python.md), [ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md)) has several things that are versioned on their own (REQ-VER-001). Each has its own tag shape. The examples are placeholders: **none of these tags exists**.

| What is released | Tag shape | Example (not created) |
|---|---|---|
| The Go core module `github.com/DanielCalvo-Calvicia/telemetry/go` | `go/vX.Y.Z` | `go/v0.3.0` (the first one planned) |
| A Go adapter module, one per adapter | `go/pkg/<functionality>/<adapter>/vX.Y.Z` | `go/pkg/spool/sqlite/v0.1.0` |
| The Python package `telemetry` | `python-vX.Y.Z` | `python-v0.1.0` |
| The old library `github.com/DanielCalvo-Calvicia/telemetry` | `vX.Y.Z` | `v0.1.0` and `v0.2.0` exist; no new one is planned |

The contract is versioned on its own by `schema_version` (`MAJOR.MINOR`, REQ-VER-003): it starts at `0.1` and reaches `1.0` with the library.

The rules:

1. **Order.** The core is released first, then the adapters, then the Python package. Each Go adapter pins a minimum version of the core in its own `go.mod`. `go.work` is for local work and CI only: **a released adapter must build without it**.
2. **Nothing is tagged unless the maintainer asks** (ADR-010). The flow is the one of `CLAUDE.md`: a `feature_*` branch, then `feature`, then `main`; the tag goes on `main`, **after the checks below pass**.
3. **A tag is permanent.** It is never moved or reused (the Go proxy caches a version forever). A bad release gets a `retract` line in the `go.mod` of its module and a new patch version.
4. **`0.x` until everything works.** The library stays a `0.x` beta until every capability works; `1.0` is the maintainer's decision (T-13.5, Q09-6, REQ-CNF-008).
5. **Before anything is pushed,** check the public audit list in [security.md](../security/security.md).
6. **A contract change is released in both languages** (see below).

**Decided by the maintainer on 2026-10-28:** every contract change, even one that only adds an optional field (a MINOR change of `schema_version`, REQ-VER-003), is implemented in **both** languages and released in both. One language never releases a contract change alone.

How a release is made today (the old library, and the same steps for the new tags):

1. Work on a `feature_*` branch; push it; open or merge into the branch `feature`.
2. Merge `feature` into `main`.
3. Run the checks above on `main`. **Only after they pass**, create an annotated tag on `main` (`git tag -a <tag> -m "..."`, with the tag shape of the table) and push it. A tag is never moved or reused.
4. Before anything is pushed, check the public audit list in [security.md](../security/security.md).

## How a consumer takes a release

```bash
go get github.com/DanielCalvo-Calvicia/telemetry@v0.2.0     # the old library; the only tags that exist today
go mod tidy
```

For the new layout, planned and **not released**: `go get github.com/DanielCalvo-Calvicia/telemetry/go@v0.3.0` for the core and `go get github.com/DanielCalvo-Calvicia/telemetry/go/pkg/<functionality>/<adapter>@v0.1.0` for an adapter (the `go/...` prefix of the tag is not written in the version). Python is installed from the Git repository, for example `pip install "telemetry @ git+https://github.com/DanielCalvo-Calvicia/telemetry@python-vX.Y.Z#subdirectory=python"`.

Pin tagged versions only, commit `go.sum`, and use `GOFLAGS=-mod=readonly` in CI. Do **not** set `GOPRIVATE` for this module: the public proxy and the checksum database are the protection against tampering. To try an unreleased change before a tag, use a `go.work` file of your own that points at a local clone; it stays in your project and is never committed there. (This repository does commit its own `go.work`, for local work and CI.)

## Rollback

A tag cannot be taken back (the proxy caches versions forever). A bad release gets a `retract` line in `go.mod` and a new patch version; consumers go back to the previous tag with `go get ...@v0.x.y`.

## Not yet verified

- The CI run on GitHub of the coverage gate after `v0.2.0` (the same command passes locally).
- The race detector.
