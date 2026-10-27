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

The race detector (`-race`) needs cgo and a C compiler; it was not run on the maintainer's Windows machine (no `gcc`).

## Pipeline (`.github/workflows/ci.yml`)

Runs on a push to `main` and on every pull request, with a read-only `GITHUB_TOKEN`; the actions are pinned by commit SHA.

| Step | What |
|---|---|
| checkout, setup-go | Go version from `go.mod` |
| `go vet ./...` | static check |
| `go test ./... -coverpkg=./internal/... -coverprofile=coverage.out` | all tests |
| coverage gate | the total over `internal/`, **excluding the `external/` folders**, must be at least **95%**; measured 98.4% on 2026-10-20 |

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
