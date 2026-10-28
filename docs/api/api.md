# Public API of the library

The only public package is `github.com/DanielCalvo-Calvicia/telemetry/pkg/logging`. A consumer never imports `internal/` (the compiler forbids it). Every exported identifier is an alias or a thin function over the internal code, so there is one definition of each type ([ADR-003](../decisions/ADR-003-public-surface-is-pkg-logging.md)). Run `go doc ./pkg/logging` for the same list with the doc comments.

## Types

| Name | Is | Use |
|---|---|---|
| `Logger` | `inbound.LoggerPort` | what a consumer asks of the logger: `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Fatal`, each `(ctx, Entry) error` |
| `Entry` | `dtos.LogEntryInboundDTO` | the entry a consumer builds for every call |
| `Config` | `config.TelemetryConfig` | the settings (with their env tags) a consumer embeds in its own config |
| `CallContext` | `call_depth.CallContext` | identity and depth of a call, carried in the context |
| `Sink` | `outbound.LogSinkPort` | a destination of entries; implement it for a destination of your own |
| `SinkEntry` | `dtos.LogEntryOutboundDTO` | the finished entry a `Sink` receives |

### `Entry`

| Field | Meaning |
|---|---|
| `ProducerID` | who produced the line (free text) |
| `ApplicationID` | the running instance; taken from the `CallContext` of `ctx` when it carries one |
| `ProcessID` | the request; taken from the `CallContext` of `ctx` when it carries one |
| `Application` | the application name; taken from the `CallContext` of `ctx` when it carries one |
| `Category` | the layer or area (for example `http`, `service`) |
| `Function` | the function that logs |
| `Indentation` | depth of the layer in the call tree; taken from the `CallContext` of `ctx` when it carries one |
| `Depth` | the step inside one layer, reported by the caller |
| `Message` | the text of the line |
| `Fields` | extra data, `map[string]any`; masked before any sink |

The level is chosen by the method that is called and the timestamp is set by the logger, so neither is part of `Entry`. When `ctx` carries a `CallContext`, its application, instance id, process id and indentation **replace** the values of the entry; without one the entry keeps what the caller set.

## Functions

| Function | Returns | Notes |
|---|---|---|
| `New(ctx, app_env, cfg Config)` | `(Logger, shutdown func(ctx) error, error)` | builds the sinks `cfg` switches on; `app_env` is `development` or `production` |
| `NewWithSinks(ctx, app_env, mask_extra_patterns, sinks ...Sink)` | `(Logger, shutdown, error)` | builds the logger over sinks the caller built |
| `NewConsoleSink()` | `Sink` | one JSON line per entry on the standard output |
| `NewHTTPSink(host, port, path)` | `(Sink, error)` | posts the flat JSON entry to a generic receiver; host includes the scheme |
| `NewAWSHTTPSink(host, port, path, log_group_name, log_stream_name)` | `(Sink, error)` | adapts the entry to the `add_log` endpoint of aws_microservice; any empty value is an error |
| `NewRequestContext(ctx, application_name, application_id)` | `(context.Context, error)` | starts the `CallContext` of a request with a new random process id |
| `WithCallContext(ctx, CallContext)` | `context.Context` | carries a call context |
| `FromContext(ctx)` | `(CallContext, bool)` | reads it |
| `NextIndentation(ctx)` | `(context.Context, bool)` | one layer deeper; `false` and the same context when there is no call context |

Constants and variables: `StartingIndentation` (1, the indentation of the inbound layer) and `ErrCallDepthProcessIDFailed`.

## Behaviour a consumer can rely on

- A call below the minimum level of `app_env` returns `nil` and sends nothing (`development`: DEBUG and above; `production`: INFO and above; TRACE is never sent).
- A failing sink never makes the call fail and never stops the other sinks; the failure is reported to the console ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)).
- Sensitive data is masked (`*****MASKED*****`) before any sink receives the entry ([ADR-006](../decisions/ADR-006-masking-before-every-sink.md)).
- `Fatal` logs; it does not stop the process.
- The only error a log call returns is a failure to build the entry (`ErrLoggerEntryBuildFailed`).
- A construction function fails with the reason (the sentinels of the internal `application` and `containers` packages are joined, so `errors.Is` finds the root cause).

## Example

```go
cfg := logging.Config{ConsoleEnabled: true}
logger, shutdown, err := logging.New(ctx, "production", cfg)
if err != nil { /* handle */ }
defer shutdown(ctx)

request_ctx, _ := logging.NewRequestContext(ctx, "orders", "instance-1")
_ = logger.Info(request_ctx, logging.Entry{Category: "http", Function: "Handle", Message: "request received"})
```

Building the sinks yourself:

```go
aws_sink, err := logging.NewAWSHTTPSink("http://aws-microservice", "8080", "/api/v1/cloudwatch/logs/add", "my-group", "my-stream")
logger, shutdown, err := logging.NewWithSinks(ctx, "production", nil, logging.NewConsoleSink(), aws_sink)
```

A runnable sample is in `examples/aws_http_smoke`; the test that sends one line to a deployed aws_microservice is `internal/tests/e2e/deployed_aws_microservice_test.go`.

## The contract the AWS HTTP sink sends

`POST host:port/path` with `Content-Type: application/json`:

```json
{"log_group_name": "my-group", "log_stream_name": "my-stream", "message": "<the whole entry as flat JSON>", "timestamp": 1792491116421}
```

`message` is the entry encoded as the flat JSON payload (`ProducerID`, `ApplicationID`, `ProcessID`, `Application`, `Category`, `Function`, `Indentation`, `Depth`, `Level`, `Message`, `Fields`, `Timestamp`), so the level and the fields are kept in CloudWatch. `timestamp` is the entry's time in milliseconds. A status of 400 or above is a delivery failure; the answer body is not interpreted.

> **Planned (T-02.1):** the public names of the new standard follow the rule in [contracts/conventions/naming.md](../../contracts/conventions/naming.md): Go `PascalCase`, Python `snake_case` functions and `PascalCase` classes, `snake_case` in JSON. This document describes the old `pkg/logging`, which does not change.

> **Planned (T-02.2):** every event of the new standard shares one envelope, defined by [contracts/schemas/envelope.schema.json](../../contracts/schemas/envelope.schema.json): `schema_version`, `event_id` (UUIDv7), `event_type`, `occurred_at` and `recorded_at` (both UTC), `severity`, `resource`, `actor`, `context`, `data` and `attributes`. It does not describe the old `pkg/logging`, which does not change.
