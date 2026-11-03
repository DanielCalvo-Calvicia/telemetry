# telemetry

Telemetry library for Snow Factory services, in Go and in Python, with **one contract** for both: structured events (the first one is the log line), masking of sensitive data before any destination, limits, a call context, and destinations the project passes in code (the console, a file, a generic HTTP receiver and the `add_log` endpoint of aws_microservice, which stores the line in CloudWatch Logs).

Licensed under MIT. Report a vulnerability privately (see [SECURITY.md](SECURITY.md)).

```text
your service ──► logger ──► event ──► mask ──► limits ──► console
  (go/pkg/telemetry,                                  ├──► file
   telemetry in Python)                               ├──► HTTP receiver
                                                      └──► aws_microservice add_log ──► CloudWatch Logs
```

## Status

The new standard ([PR-001](docs/proposals/PR-001-telemetry-standard-go-python.md), [ADR-011](docs/decisions/ADR-011-adopt-the-telemetry-standard-go-python.md) to [ADR-013](docs/decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md)) is being built task by task ([docs/tasks/README.md](docs/tasks/README.md)). **Nothing of it is tagged or released yet.** The old library (`pkg/logging`, released as `v0.1.0` and `v0.2.0`) was removed from this branch in T-04.7 (2026-11-03); its tags stay available on the Go proxy for the project that uses them, and it is not maintained.

| Area | State |
|---|---|
| The contract (envelope, event registry, error catalog, config schemas) and the generator of the code of both languages | Built (T-02) |
| The core in Go and in Python: value objects, ports, event builder, masking, limits, configuration and profiles, errors, call context, the entry point (`go/pkg/telemetry`, `telemetry.new`) | Built (T-03) |
| The logger with one method per severity, `log/slog` and `logging` integration | Built (T-04.1) |
| Destinations: console, file (rotation and quota), HTTP (shared client), AWS HTTP (a Go module of its own) | Built (T-04.2 to T-04.6); no queue and no retry loop yet (T-05) |
| The fake receiver for the tests of delivery | Built (T-04.8) |
| The old library | Removed (T-04.7) |
| The queue, batching, retries, flush and shutdown (T-05), the spool (T-06), events, devices, AI, OpenTelemetry and the rest | Planned |
| CI (format, vet, build, tests and a coverage gate of 95%, for every Go module and for Python 3.10 and 3.11) | Written and checked locally; not yet run on GitHub, it needs a pull request |
| Deployment | None, by design: it is a library ([ADR-010](docs/decisions/ADR-010-no-deployment-release-by-tags.md)) |

## Requirements

- Go **1.25.5** or newer, or Python **3.10** or newer. The core of each has no third-party dependency.
- For the AWS HTTP adapter: a reachable aws_microservice with the CloudWatch Logs service switched on.

## Quick start

**Not released yet.** When it is, Go is installed with `go get github.com/DanielCalvo-Calvicia/telemetry/go@<tag>` (and `.../go/pkg/logging/aws_http@<tag>` for the AWS adapter) and Python from the Git repository; see the tag rule in [docs/deployment/deployment.md](docs/deployment/deployment.md). Until then use a `go.work` file or a clone of this repository.

```go
import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/console"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

sink, err := console.NewDestination(console.DefaultSettings())
if err != nil { /* handle */ }

config := telemetry.Microservice()
config.Resource.ServiceName = "my-app"
core, err := telemetry.New(config, telemetry.WithLogSink(sink))
if err != nil { /* handle */ }
defer core.Shutdown(ctx)

core.Logger().Info(ctx, telemetry.LogEntry{Category: "http", Function: "Handle", Message: "request received"})
```

```python
import dataclasses
from telemetry import LogEntry, microservice, new, with_log_sink
from telemetry.logging import console

config = microservice()
config = dataclasses.replace(config, resource=dataclasses.replace(config.resource, service_name="my-app"))
core = new(config, with_log_sink(console.new_destination(console.default_settings())))
core.logger.info(LogEntry(category="http", function="handle", message="request received"))
core.shutdown()
```

The library **never reads environment variables** (ADR-004): the project loads its configuration, with the env names listed in [docs/configuration/environment-variables.md](docs/configuration/environment-variables.md) and in `.env.example`, and passes the settings in. The project also passes the destinations it chose, in code: nothing registers itself. A destination that fails never fails the log call. The whole API is in [docs/api/api.md](docs/api/api.md).

## Try it

```bash
# a sample service that logs through the AWS HTTP adapter to the add_log endpoint of an aws_microservice
cd examples/aws_http_smoke
TELEMETRY_AWS_HTTP_HOST=http://localhost TELEMETRY_AWS_HTTP_PORT=8080 \
TELEMETRY_AWS_HTTP_LOG_GROUP_NAME=my-log-group TELEMETRY_AWS_HTTP_LOG_STREAM_NAME=my-log-stream \
go run .
```

The sample is a module of its own that imports the core and the adapter, so it also proves that a project can use the adapter module. `go/pkg/logging/aws_http/internal/tests/e2e/deployed_aws_microservice_test.go` sends one line to a deployed aws_microservice and fails when the endpoint does not accept it; set its constants first (see [docs/operations/monitoring.md](docs/operations/monitoring.md)); its switch is `false` in everything that is committed.

## Layout

```text
go/                         the Go core: a module of its own; `pkg/telemetry` is its public entry point, `pkg/logging/{console,file,http}` the destinations of the core, the layers are in `internal/`
go/pkg/logging/aws_http/    the AWS HTTP adapter: a Go module of its own (T-04.6)
python/                     the Python implementation of the same core: an installable package `telemetry` (the public entry point is `telemetry/__init__.py`)
contracts/                  the contract between Go and Python: the envelope, the event types, the registry, the error codes, the config schemas; the code generated from it is in go/ and python/
integration-tests/          the cross-language fixtures that both implementations read (`fixtures/`) and the seed of the runner (T-12)
examples/aws_http_smoke/    a sample service, a Go module of its own
tools/contract_generator/   the generator: turns contracts/ into the Go and Python code (its own module; see its README to regenerate)
tools/registry_check/       the check that every requirement of the specification has a conformance test or a pending task (its own module)
docs/                       the documentation (index in docs/README.md)
go.work                     joins the Go modules (the core, the adapters, the sample and the tools) for local work and CI
```

## Checks

```bash
# in each Go module (go/, go/pkg/logging/aws_http, tools/...)
gofmt -l .          # prints nothing
go vet ./...
go build ./...
go test ./... -count=1

# in python/
ruff check . && ruff format --check .
coverage run -m unittest discover -s internal/tests -t . && coverage report
```

## Documentation

[docs/README.md](docs/README.md) is the index: architecture, API, configuration, decisions (ADR-001 to ADR-013), proposals, requirements, security, operations, deployment (releases) and the task plan.
