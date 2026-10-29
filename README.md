# telemetry

Go logging library for Snow Factory services: structured entries, a minimum level that follows the environment, masking of sensitive data, and several sinks (the console, a generic HTTP receiver and the `add_log` endpoint of aws_microservice, which stores the entry in CloudWatch Logs).

Public API: `github.com/DanielCalvo-Calvicia/telemetry/pkg/logging`. Licensed under MIT. Report a vulnerability privately (see [SECURITY.md](SECURITY.md)).

```text
your service ──► logging.Logger ──► mask ──► console
   (pkg/logging)   (level by APP_ENV)    ├──► HTTP receiver
                                         └──► aws_microservice add_log ──► CloudWatch Logs
```

## Status

Released as `v0.1.0` (2026-10-19) and `v0.2.0` (2026-10-20). The API may change while the library is on `v0.x`.

| Area | State |
|---|---|
| Logger with one method per severity, the level chosen by `app_env` (`development` DEBUG and above, `production` INFO and above) | Implemented |
| Call context (application, instance, process id, indentation) carried in `context.Context` | Implemented |
| Masking of sensitive data before any sink (keys, tokens, JWTs, private keys, passwords, credential pairs, sensitive field names, extra patterns) | Implemented |
| Console sink and generic HTTP sink | Implemented |
| AWS HTTP sink: adapts the entry to the `add_log` contract of aws_microservice | Implemented (v0.2.0) |
| Ready-made constructors for each sink, the `Sink` interface and `NewWithSinks` | Implemented (v0.2.0) |
| A failing sink never fails the caller; 3 s timeout, no retry | Implemented |
| Runnable sample (`examples/aws_http_smoke`) and a test that sends one line to a deployed aws_microservice | Implemented |
| CI (format, vet, build, tests and a coverage gate of 95%, for every Go module and for Python 3.10 and 3.11) | Written on 2026-10-28 and checked locally (98.6% for `go/`, 98.0% for the old module); not yet run on GitHub, it needs a pull request |
| Metrics (`internal/metrics`, `pkg/metrics`) | Planned |
| The new telemetry standard (one contract for Go and Python, device and AI modules, a queue and a spool): [PR-001](docs/proposals/PR-001-telemetry-standard-go-python.md), [ADR-011](docs/decisions/ADR-011-adopt-the-telemetry-standard-go-python.md) to [ADR-013](docs/decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md) | Accepted (2026-10-28), nothing built |
| Deployment | None, by design: it is a library ([ADR-010](docs/decisions/ADR-010-no-deployment-release-by-tags.md)) |

## Requirements

- Go **1.25.5** or newer.
- For the AWS HTTP sink: a reachable aws_microservice with the CloudWatch Logs service switched on.

## Quick start

```bash
go get github.com/DanielCalvo-Calvicia/telemetry@v0.2.0
```

This is the old library, the only one released. The new standard will be installed from `github.com/DanielCalvo-Calvicia/telemetry/go` (and Python from the Git repository): **planned, nothing is tagged yet**; see the tag rule in [docs/deployment/deployment.md](docs/deployment/deployment.md).

```go
import "github.com/DanielCalvo-Calvicia/telemetry/pkg/logging"

logger, shutdown, err := logging.New(ctx, "production", logging.Config{ConsoleEnabled: true})
if err != nil { /* handle */ }
defer shutdown(ctx)

request_ctx, _ := logging.NewRequestContext(ctx, "my-app", "instance-1")
_ = logger.Info(request_ctx, logging.Entry{Category: "http", Function: "Handle", Message: "request received"})
```

The library **never reads environment variables**: embed `logging.Config` (env tags included) in your own configuration struct and load it yourself. `app_env` is `development` or `production`. The variables are listed in [docs/configuration/environment-variables.md](docs/configuration/environment-variables.md) and in `.env.example`.

### Building the sinks yourself

```go
aws_sink, err := logging.NewAWSHTTPSink("http://aws-microservice", "8080", "/api/v1/cloudwatch/logs/add", "my-group", "my-stream")
if err != nil { /* handle */ }

logger, shutdown, err := logging.NewWithSinks(ctx, "production", nil, logging.NewConsoleSink(), aws_sink)
```

`NewConsoleSink`, `NewHTTPSink` and `NewAWSHTTPSink` are the ready-made sinks; implement `logging.Sink` for a destination of your own. The whole API is in [docs/api/api.md](docs/api/api.md).

## Try it

```bash
# a sample service that logs in production to the add_log endpoint of an aws_microservice
LOGGER_AWS_HTTP_HOST=http://localhost LOGGER_AWS_HTTP_PORT=8080 \
LOGGER_AWS_HTTP_LOG_GROUP=my-group LOGGER_AWS_HTTP_LOG_STREAM=my-stream \
go run ./examples/aws_http_smoke
```

`internal/tests/e2e/deployed_aws_microservice_test.go` sends one line to a deployed aws_microservice and fails when the endpoint does not accept it; set its constants first (see [docs/operations/monitoring.md](docs/operations/monitoring.md)).

## Layout

```text
pkg/logging/                public API (the old library; T-04.7 of the new standard removes it)
internal/logging/           domain / application / infrastructure (the standard Go layers)
internal/composition_root/  containers/ and dependencies/ (the only place that wires adapters)
internal/tests/             mirrors the code tree; black-box tests; e2e/
examples/aws_http_smoke/    sample service
docs/                       the documentation (index in docs/README.md)
go/                         the Go core of the new standard ([PR-001](docs/proposals/PR-001-telemetry-standard-go-python.md)): a module of its own, only the empty layers so far
go.work                     joins the root module, go/ and tools/contract_generator for local work and CI
tools/contract_generator/   the generator: turns contracts/ into the Go and Python code (its own module; see its README to regenerate)
tools/registry_check/       the check that every requirement of the specification has a conformance test or a pending task (its own module)
contracts/                  the contract between Go and Python: the envelope, the first event type, the registry, the error codes; the code generated from it is in go/ and python/
python/                     the Python implementation (only the skeleton: an installable package with the empty layers and one test)
integration-tests/          the cross-language fixtures and tests (planned, empty)
```

## Checks

```bash
gofmt -l .          # prints nothing
go vet ./...
go build ./...
go test ./... -count=1
```

## Documentation

[docs/README.md](docs/README.md) is the index: architecture, API, configuration, decisions (ADR-001 to ADR-013; the last three are accepted and not built), proposals, requirements, security, operations and deployment (releases).
