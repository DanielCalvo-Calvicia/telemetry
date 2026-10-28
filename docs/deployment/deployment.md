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

Runs on a push to `main` and on every pull request, with a read-only `GITHUB_TOKEN` (`permissions: contents: read`); every action is pinned by commit SHA (`actions/checkout` v4.2.2, `actions/setup-go` v5.5.0, `actions/setup-python` v5.6.0). A push to any other branch runs nothing until a pull request is opened. Four jobs (T-01.5, 2026-10-28; written and run locally, **not yet run on GitHub**):

| Job | What it does |
|---|---|
| `modules` | reads `go.work` (`go work edit -json`) and lists its modules: today `.` (the old library) and `./go` (the new core); a module added to `go.work` is checked without editing the file |
| `go` (one per module) | in the module folder: `gofmt -l .` and `goimports -l .` (`golang.org/x/tools/cmd/goimports@v0.35.0`) print nothing; `go vet ./...`; `go build ./...`; `go test ./... -count=1 -coverpkg=./internal/... -coverprofile=coverage.out`; the coverage gate: the total over `internal/`, **excluding the `external/` folders**, must be at least **95%** (measured locally on 2026-10-28: 98.6% for `go/`, 98.0% for the old module without its e2e package) |
| `python` (3.10 and 3.11) | in `python/`: `pip install . coverage[toml]==7.17.0 ruff==0.17.3`; `ruff check .` and `ruff format --check .`; `coverage run -m unittest discover -s internal/tests -t .` and `coverage report`, whose gate is `fail_under = 95` with `omit */external/*` in `pyproject.toml`. 3.10 is the declared minimum and 3.11 is the version of the Raspberry Pi of the pilot |
| `cross-tests` | passes while empty (it only prints a line); T-02.8 creates the first Go and Python fixtures and T-12 the runner (REQ-CNF-006) |

The job for generated code (it regenerates from `contracts/` and fails on a difference) is added by T-02.4. The race detector is not run (it needs cgo). The e2e test of the deployed service is skipped in CI: its switch is `false` in what is committed. The old module keeps its job until T-04.7 removes it.

## Releases

| Tag | Date | Contents |
|---|---|---|
| `v0.1.0` | 2026-10-19 | logging: console and HTTP sinks, masking, call context, `pkg/logging` |
| `v0.2.0` | 2026-10-20 | the AWS HTTP sink, the ready-made sink constructors and `NewWithSinks`, the end-to-end tests |

How a release is made:

1. Work on a `feature_*` branch; push it; open or merge into the branch `feature`.
2. Merge `feature` into `main`.
3. Run the checks above on `main`. **Only after they pass**, create an annotated tag on `main` (`git tag -a v0.x.y -m "..."`) and push it. A tag is never moved or reused.
4. Before anything is pushed, check the public audit list in [security.md](../security/security.md).

## How a consumer takes a release

```bash
go get github.com/DanielCalvo-Calvicia/telemetry@v0.2.0
go mod tidy
```

Pin tagged versions only, commit `go.sum`, and use `GOFLAGS=-mod=readonly` in CI. Do **not** set `GOPRIVATE` for this module: the public proxy and the checksum database are the protection against tampering. To try an unreleased change before a tag, use a `go.work` file that points at a local clone; it is git-ignored and never committed.

## Rollback

A tag cannot be taken back (the proxy caches versions forever). A bad release gets a `retract` line in `go.mod` and a new patch version; consumers go back to the previous tag with `go get ...@v0.x.y`.

## Not yet verified

- The CI run on GitHub of the coverage gate after `v0.2.0` (the same command passes locally).
- The race detector.
