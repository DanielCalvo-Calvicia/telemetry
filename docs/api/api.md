# Public API of the library

The library is `telemetry`, in Go and in Python, with one contract for both ([PR-001](../proposals/PR-001-telemetry-standard-go-python.md)). A project builds the core **once**, in code, passes the destinations it chose, and logs or emits events. A project never imports an `internal/` package (the Go compiler forbids it) and nothing registers itself on import ([ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md)). The old `pkg/logging` API (`logging.New`, `NewWithSinks`, `Sink`, `Entry`) was removed in T-04.7 (2026-11-03); the tags `v0.1.0` and `v0.2.0` keep it on the Go proxy and it is not maintained.

**Nothing of the new library is tagged or released yet**; until it is, use a `go.work` file or a clone of this repository (see the README).

| Language | Public packages | Module |
|---|---|---|
| Go | `github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry` (the entry point), `.../go/pkg/logging/console`, `.../go/pkg/logging/file`, `.../go/pkg/logging/http` | the core, `github.com/DanielCalvo-Calvicia/telemetry/go` |
| Go | `github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http` | a module of its own, so a project that does not use it does not download it |
| Python | `telemetry` (the entry point) and `telemetry.logging.{console,file,http,aws_http}` | one installable package, standard library only, no extra to install |

Run `go doc ./pkg/telemetry` in `go/` (and `go doc .` in `go/pkg/logging/aws_http`) for the same list with the doc comments. The names that both languages export are listed in `integration-tests/fixtures/core/public_names.json`; the rule that maps one to the other is in [contracts/conventions/naming.md](../../contracts/conventions/naming.md): Go `PascalCase` exported names, Python `snake_case` functions and `PascalCase` classes, `snake_case` in JSON. In Go the public types are **aliases** of the internal ones and the functions are thin ([ADR-003 and ADR-013](../decisions/ADR-013-go-modules-per-adapter-chosen-in-code.md)).

## The entry point

Go, package `telemetry`:

| Name | Is | Use |
|---|---|---|
| `New(config Config, options ...Option)` | `(*Telemetry, error)` | validates the configuration and builds the core with the adapters of the options; an invalid configuration returns a `*ValidationError` with every problem, a `durable` delivery mode without `WithLocalSpool` is one of them |
| `Microservice()`, `Cronjob()`, `Worker()`, `Robotics()`, `OfflineDevice()` | `Config` | the profiles; they only change the delivery mode (`best_effort`, `retry` or `durable`) |
| `WithLogSink`, `WithEventSink`, `WithMetricsSink`, `WithTraceSink`, `WithDeviceTelemetrySink`, `WithAITelemetrySink` | `Option` | the destinations; a sink given twice is a second destination |
| `WithLocalSpool`, `WithClock`, `WithIDGenerator` | `Option` | the spool port, the clock and the id generator (for tests and for the project's own adapters) |
| `(*Telemetry).Emit(ctx, NewEventInput)` | `EmitResult` | builds, validates, masks, limits and queues one event; never fails: the result says how many destinations queued it (accepted), dropped it (for example `queue_full`) and why |
| `(*Telemetry).Logger()` | `Logger` | the logger (below) |
| `(*Telemetry).Shutdown(ctx)` | `ShutdownResult` | call it when the program ends; it is a stub until the queue of T-05.4 exists |

Types: `Telemetry`, `Config` (with `ResourceConfig`, `DeliveryConfig`, `LimitsConfig`, `SpoolConfig`, `MaskingConfig`), `Option`, `Logger`, `LogEntry`, `NewEventInput`, `EmitResult`, `SendResult`, `ShutdownResult`, `Severity` (with the constants `SeverityTrace` ... `SeverityCritical`), `Envelope`, `EventID`, `UTCInstant`, `ActorIdentity`, `CorrelationContext`, `EventAttributes`, `Identifier`, `Limits` (`DefaultLimits()`, `NewLimits(...)`), `ValidationError`, `CallContext`, and the ports a project implements for a destination of its own: `LogSinkPort`, `EventSinkPort`, `MetricsSinkPort`, `TraceSinkPort`, `DeviceTelemetrySinkPort`, `AITelemetrySinkPort`, `LocalSpoolPort`, `ClockPort` and `IDGeneratorPort`.

Python, package `telemetry`: `new(config, *options)`, the profiles `microservice()`, `cronjob()`, `worker()`, `robotics()`, `offline_device()`, the options `with_log_sink`, `with_event_sink`, `with_metrics_sink`, `with_trace_sink`, `with_device_telemetry_sink`, `with_ai_telemetry_sink`, `with_local_spool`, `with_clock` and `with_id_generator`, `core.emit(...)`, `core.logger`, `core.shutdown()`, and the same types (`Telemetry`, `Config`, `LogEntry`, `NewEventInput`, `Severity`, `Limits`, `CallContext`, the ports, ...). An invalid configuration raises `ConfigurationError` with every problem.

The settings are plain values: the project loads them with its own tool (the env names are in [environment-variables.md](../configuration/environment-variables.md)) and passes them in. The library never reads the environment or a file ([ADR-004](../decisions/ADR-004-library-never-reads-the-environment.md)). A log call enqueues the event and a worker per destination sends it in batches (T-05.1); the spool port is accepted but not used until T-06.

## The logger

`core.Logger()` (Go) and `core.logger` (Python) give a `Logger` with one method per severity. Go: `Trace`, `Debug`, `Info`, `Warn`, `Error` and `Critical`, each `(ctx, LogEntry)` and with no return value. Python: `trace`, `debug`, `info`, `warn`, `error` and `critical`, each taking a `LogEntry`.

| `LogEntry` field | Meaning |
|---|---|
| `Category` | the layer or area (for example `http`, `service`) |
| `Function` | the function that logs (optional) |
| `Message` | the text of the line |
| `Fields` | extra data; they become the attributes of the event |

- A log call builds a `log.entry.recorded` event: `category`, `message` and, when given, `function` go to `data`; the extra fields go to `attributes`; the severity is the one of the method; the application is the `service_name` of the resource configuration; the ids of the call context are attached by the core. The data of the event type is in [contracts/schemas/log/entry_recorded.schema.json](../../contracts/schemas/log/entry_recorded.schema.json); the event types are registered in [contracts/events/registry.yaml](../../contracts/events/registry.yaml).
- **A log call returns nothing and never fails**: no error, no exception, no panic ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)). `Critical` logs; it does not stop the process. A rejected event is counted; the counters become diagnostics in T-05.5.
- **Fields:** scalars and lists of scalars are kept; anything else is written as its canonical JSON text; null values and empty keys are dropped. The limits (32 attributes, a key of 64 characters, a value of 256 characters, an event of 64 KiB) are applied after the masking.
- **Native logging.** Go: `telemetry.NewSlogHandler(logger, default_category)` returns a `slog.Handler`. Python: `telemetry.LoggingHandler(core.logger, default_category)` is a `logging.Handler`. The attributes named `category` and `function` set the category and the function. The handler does not filter by level.
- No request body, payload or traceback is recorded unless the caller passes it in the fields.

## The call context

A `CallContext` carries the identity and the position of a call (application, instance id, execution id, correlation id, trace id, span id and indentation) in the context of the call; the core attaches it to the event.

- Go: `NewCallContext(options...)` with the options `WithApplication`, `WithInstanceID`, `WithExecutionID`, `WithCorrelationID`, `WithTraceID` and `WithSpanID`; `WithCallContext(ctx, call_context)` carries it; `FromContext(ctx)` reads it; `NextIndentation(ctx)` returns the context one layer deeper and `true`, or the same context and `false` when there is no call context. An invalid field is an error that names the field and never the value. The trace and span ids must be 32 and 16 lowercase hexadecimal digits and not all zeros.
- Python: `new_call_context(...)`, `use_call_context(call_context)` (a context manager), `current_call_context()` and `deeper_indentation()` (a context manager).
- The old `NewRequestContext(ctx, application_name, application_id)` is **not** part of the public API: the helper of the new core needs an id generator and is internal. A project builds the context with `NewCallContext` and gives the correlation id itself with `WithCorrelationID`.

## Destinations

A destination is built from its settings and given to the core as `telemetry.WithLogSink(destination)` (Go) or `telemetry.with_log_sink(destination)` (Python). It never fails the caller and never stops another destination. An invalid setting fails when the destination is built, with every problem together, each with its field and a reason that holds no configured value. The env names of the settings are in [environment-variables.md](../configuration/environment-variables.md).

### Console (built, T-04.2)

Go: `console.NewDestination(console.Settings{Format: "json", MinimumSeverity: "INFO"})` from `go/pkg/logging/console`; `console.DefaultSettings()` gives the defaults; `NewDestinationWithWriter(settings, writer)` writes to another `io.Writer`. Python: `telemetry.logging.console.new_destination(Settings(...))`, `default_settings()` and `new_destination_with_writer(settings, writer)`. `Format` is `json` (the canonical JSON of the envelope, one line per event) or `text` (a readable line); `MinimumSeverity` is `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR` or `CRITICAL` in upper case. An event below the minimum is not written (it counts as accepted).

### File (built, T-04.3)

Go: `file.NewDestination(file.Settings{Directory: "logs", ...})` from `go/pkg/logging/file` returns a `*Destination`; `file.DefaultSettings()` gives the defaults (no directory) and `destination.Counters()` the counters. Python: `telemetry.logging.file.new_destination(Settings(directory="logs", ...))`, `default_settings()` and `destination.counters`. The directory is required (the library picks no path); the files are `telemetry.jsonl` and, rotated, `telemetry.jsonl.1` (the newest) ... `telemetry.jsonl.<max_files - 1>`; each line is the canonical JSON of one event. A directory that cannot be written fails when the destination is built.

### HTTP (built, T-04.5)

Go: `http.NewDestination(http.Settings{URL: "https://...", ...})` from `go/pkg/logging/http` (package `http`: import it with an alias next to `net/http`) returns a `*Destination`; `http.DefaultSettings()` gives the defaults (no URL: it is required) and `NewDestinationWithLimits(settings, limits)` takes the limits (`telemetry.Limits`, `telemetry.DefaultLimits()`, `telemetry.NewLimits(...)`). Python: `telemetry.logging.http.new_destination(Settings(url="https://...", ...))`, `default_settings()`, `new_destination_with_limits(settings, limits)` and `telemetry.Limits`. The settings are those of the shared client and `minimum_severity`. The destination sends **one request per batch** and returns the result: events below the minimum are accepted without being sent; a transient failure is `failed`, a permanent rejection `dropped`, with the reason (`status_503`, `timeout`, `network_error`, `batch_too_large`...). `Deliver` also returns the full outcome (a `Delivery`). **It does not queue and does not retry yet** (T-05).

**The contract it sends** (the shared HTTP client of the core, built in T-04.4): `POST` (or `PUT` or `PATCH`) to the URL the consumer configures, with `Content-Type: application/json` and, when asked, `Content-Encoding: gzip`, and the body `{"events": [ ... ], "schema_version": "0.1"}` (canonical JSON, at most the batch limit of 1 MiB). A bearer token goes in `Authorization` or an API key in a header the consumer names; the timeout is 3 s by default and the TLS verification is on. The receiver answers 2xx for success; 408, 429 and 5xx are transient (and `Retry-After` is honored by the retry policy that T-05 will run); other 4xx and any redirect are permanent rejections. A token is never sent over plain `http` except to `localhost`.

### AWS HTTP (built, T-04.6)

The adapter for the `add_log` endpoint of aws_microservice, which stores the line in CloudWatch Logs ([ADR-008](../decisions/ADR-008-aws-http-sink-adapts-to-add-log.md)). Go: a module of its own, `github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http` (not tagged yet), package `aws_http`: `aws_http.NewDestination(aws_http.Settings{Host: "http://localhost", Port: "8080", Path: "/api/v1/cloudwatch/logs/add", LogGroupName: "my-log-group", LogStreamName: "my-log-stream"})` returns a `*Destination`; `DefaultSettings()` gives the defaults, `NewDestinationWithLimits(settings, limits)` takes the limits and `ValidateSettings(settings)` checks them. Python: `telemetry.logging.aws_http.new_destination(Settings(host=..., ...))` and `default_settings()`. The five required settings are never defaulted and an empty one is a validation error that names it. Only `log.*` events are sent (one request each; any other event is skipped and counted as accepted); `Deliver` answers a `Delivery` with the result, the outcome of the last request, `Sent` and `Skipped`. See [architecture section 2.24](../architecture/architecture.md).

**The request it sends:** `POST host:port/path` with `Content-Type: application/json`:

```json
{"log_group_name": "my-group", "log_stream_name": "my-stream", "message": "<the whole entry as flat JSON>", "timestamp": 1792491116421}
```

`message` is the entry as the flat JSON payload that aws_microservice has always received (`ProducerID`, `ApplicationID`, `ProcessID`, `Application`, `Category`, `Function`, `Indentation`, `Depth`, `Level`, `Message`, `Fields`, `Timestamp`), so the level and the fields are kept in CloudWatch; `timestamp` is `occurred_at` in milliseconds. The mapping from the event to each key is in architecture section 2.24. A status of 2xx accepts; the answer body is not interpreted.

## What a consumer can rely on

- A log call never returns an error, never raises and never panics; `Emit` never fails and reports how many sinks accepted, dropped or failed the event ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)).
- A failing destination never stops another one; the failure is in the result of the event.
- Every event is **masked** before any destination receives it ([ADR-006](../decisions/ADR-006-masking-before-every-sink.md)), then limited. The masking has options in the configuration: extra field names, extra patterns, a replacement of the default list of names, classes of fields and a rule per class (`drop`, `redact`, `allow`).
- A caller sets the event type, the severity, the data and, optionally, `occurred_at`, the actor, the context and the attributes; it cannot set `recorded_at` (the library's clock does), `event_id`, `schema_version` or `resource` (the configuration gives it).
- An event is serialized in the canonical form of RFC 8785 (JCS): sorted keys, no whitespace, the same bytes in Go and Python; the times are written `YYYY-MM-DDTHH:MM:SS.ffffffZ` (UTC, six decimals). Every event shares one envelope, defined by [contracts/schemas/envelope.schema.json](../../contracts/schemas/envelope.schema.json): `schema_version`, `event_id` (UUIDv7), `event_type`, `occurred_at` and `recorded_at`, `severity`, `resource`, `actor`, `context`, `data` and `attributes`.
- An event that does not match the schema of its type is **rejected, not thrown**: the validator returns a classified error (`event_type_unknown` or `event_schema_invalid`) whose message names the event type and the field path and never the value ([architecture section 2.6](../architecture/architecture.md)).
- The errors a consumer can see are those of the building of the core (an invalid configuration: every problem with its field and code) and the diagnostics (counts by code, T-05.5). The codes are those of [the catalog](../../contracts/errors/catalog.yaml), the same in both languages; a message never holds a value the caller gave.

## Example

```go
import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/console"
	"github.com/DanielCalvo-Calvicia/telemetry/go/pkg/telemetry"
)

sink, err := console.NewDestination(console.DefaultSettings())
if err != nil { /* handle */ }

config := telemetry.Microservice()
config.Resource.ServiceName = "orders"
core, err := telemetry.New(config, telemetry.WithLogSink(sink))
if err != nil { /* handle */ }
defer core.Shutdown(ctx)

core.Logger().Info(ctx, telemetry.LogEntry{Category: "http", Function: "Handle", Message: "request received"})
```

A runnable sample that logs through the AWS HTTP adapter is in `examples/aws_http_smoke` (a Go module of its own). The test that sends one line to a deployed aws_microservice is `go/pkg/logging/aws_http/internal/tests/e2e/deployed_aws_microservice_test.go` (see [monitoring](../operations/monitoring.md)).

> **Update 2026-11-03 (T-05.1):** delivery is asynchronous. `Emit` and the log methods return when the event is **queued**: *accepted* in an `EmitResult` means queued, and the outcome of the send (a failure of a destination, for example) is not in it: it is counted by the destination and will be public with the diagnostics (T-05.5). A short program must wait for its events until `Shutdown` flushes (T-05.4). The `delivery` settings (queue 10,000, batch 100, flush 2,000 ms, timeout 3,000 ms) are the ones of the configuration; nothing starts before `New` / `new` is called and no function was added to the public API.