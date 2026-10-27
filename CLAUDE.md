# telemetry

Go module `github.com/DanielCalvo-Calvicia/telemetry`: a **public** logging library (console, HTTP and AWS HTTP sinks, masking, call context). It is a library: no `cmd/`, no image, no manifest, no deployment. Read [README.md](README.md) for the state, [docs/architecture/architecture.md](docs/architecture/architecture.md) for the structure and [docs/README.md](docs/README.md) for the index.

## MANDATORY: every change updates the documentation

**No change is finished until the documents that describe it are true again.** This applies to every change: code, configuration, tests, a decision. Do it in the same piece of work, not later. A change that leaves a document saying something false is not done.

| If you change... | Update, in the same change |
|---|---|
| Code, a package, a folder or a file name | [docs/architecture/architecture.md](docs/architecture/architecture.md) (sections 2 and 3) and the layout of [README.md](README.md) |
| The public API (`pkg/logging`: a type, a function, a constant) | [docs/api/api.md](docs/api/api.md), the quick start of [README.md](README.md), the API rows of [docs/requirements/requirements.md](docs/requirements/requirements.md) and `go doc ./pkg/logging` (every exported identifier has a doc comment that starts with its own name) |
| A `Config` field, an env tag, a default | `.env.example`, [docs/configuration/environment-variables.md](docs/configuration/environment-variables.md), the config test that lists the tags |
| A sink, the contract it sends, a timeout | [docs/api/api.md](docs/api/api.md), [docs/operations/monitoring.md](docs/operations/monitoring.md), [docs/architecture/architecture.md](docs/architecture/architecture.md) section 7 |
| Masking (a detector, a field name) | [docs/decisions/ADR-006-masking-before-every-sink.md](docs/decisions/ADR-006-masking-before-every-sink.md) (a dated note), [docs/security/security.md](docs/security/security.md) |
| A requirement (new, changed, done) | [docs/requirements/requirements.md](docs/requirements/requirements.md) and the status table of [README.md](README.md) |
| A decision | an ADR in `docs/decisions/` (new, or a dated update of the one it changes), its row in [docs/README.md](docs/README.md) and in the architecture document |
| A release (a tag) | the table of releases in [docs/deployment/deployment.md](docs/deployment/deployment.md), the status of [README.md](README.md), the `go get` line of the quick start |
| CI | [docs/deployment/deployment.md](docs/deployment/deployment.md) |

How to do it:

1. Change the documents **with** the change, and say what you updated when you report the work.
2. Use dated notes (`> **Update 2026-10-27:** ...`) in documents that are history (the ADRs); correct the text in documents that describe the present (architecture, API, configuration, README).
3. Mark what is planned as planned and what is built as built; never describe as working something that is not.
4. After editing, check that the links resolve.

## Standing rules

- **Public repository.** Everything pushed is permanent. No secret, no internal host name or IP address, no account id, no customer name, no binary (`__debug_bin*` is ignored); placeholders only (`http://localhost`, `my-log-group`). The inputs of `internal/tests/e2e/deployed_aws_microservice_test.go` are placeholders and its switch `run_against_deployed_service` is `false` in anything that is committed. Run the checklist of [docs/security/security.md](docs/security/security.md) before a push.
- **Never commit, push, tag or open a pull request unless asked.** A tag is permanent: never move or reuse one; a bad release gets a `retract` and a new patch version. Flow: `feature_*` branch, then `feature`, then `main`; the tag goes on `main` after the checks pass.
- **The library never reads the environment** (ADR-004): no `os.Getenv`, no `envconfig.Process`, no `.env`. `Config` carries the tags; the consumer loads it.
- **Public surface:** only `pkg/logging`. A consumer never imports `internal/`. Functions are thin; types are aliases (ADR-003).
- **Layers:** domain imports nothing from the other layers; application defines the ports; only `internal/composition_root` imports the concrete outbound adapters; one dependencies file per adapter; the I/O of an adapter lives in its `external/` folder, its error translation in `error/`, its pure helpers in `helpers/`.
- **A sink never fails the caller** (ADR-005) and masking happens before any sink (ADR-006): do not add a code path that logs an entry unmasked.
- **Tests:** every test lives under `internal/tests/` mirroring the code tree (or beside `pkg/logging` for the public API), black-box (`package x_test`). Coverage of `internal/`, excluding `external/` folders, stays at 95% or more (CI gate).
- **Names:** locals, parameters, unexported fields and import aliases are descriptive snake_case; Go identifiers that are exported keep the Go convention.
- **Errors:** sentinels in `application/errors.go` and `domain/errors.go`, composed with `errors.Join` so the root cause is kept.
- Code, comments and documents are in English.

## Checks before saying a task is done

```bash
gofmt -l .          # prints nothing
go vet ./...
go build ./...
go test ./... -count=1
go test ./... -coverpkg=./internal/... -coverprofile=coverage.out   # the total, without the external/ folders, must be at least 95%
```

`goimports -l .` must also print nothing (the group of the import block: standard library, then the rest). `-race` needs cgo.
