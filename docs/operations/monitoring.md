# Monitoring and operations

The library is not a service, so it has no probes and no metrics endpoint. What a consumer can observe today is what the destinations write and the result that each call answers. The diagnostics of the library itself (counters by destination, reason and error code, and a report of the health of the library) are **planned** (T-05.5): nothing is emitted yet.

## What the library reports

| What | Where | State |
|---|---|---|
| The events of the consumer | every destination that was passed to the core | built: the console writes them (canonical JSON or a readable line), the file appends JSON Lines, the HTTP destination and the AWS HTTP adapter send them |
| The result of a call | `Emit` returns an `EmitResult`; a destination returns a `SendResult` (accepted, dropped, failed and a reason); the HTTP destinations also return a `Delivery` with the full outcome | built; a log call returns nothing |
| The counters of the file destination | `Counters()` (Go), `counters` (Python) | built |
| Counters and diagnostics of the library (events dropped by reason and destination, rejected, truncated; a health report) | the diagnostics of T-05.5 | **planned**; the results above are their source |

A log call never fails the caller and a failing destination never stops another one ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)). Until the diagnostics exist, a failure is visible only in the result of the call (a consumer that uses `Emit` sees it) and in the destination that did receive the event; a log call by itself shows nothing.

## What to watch in a consumer

| Symptom | Likely cause | Where to look |
|---|---|---|
| A start error from `New` (Go) or `new` (Python) listing fields | an invalid configuration or invalid settings of a destination, or a `durable` mode without a spool | the list names every problem with its field and code; a message never holds a configured value |
| Lines missing in the console or the file | the event is below the `minimum_severity` of that destination (the default is `INFO`), or it was rejected by the validation or the limits | the settings of the destination ([environment-variables.md](../configuration/environment-variables.md)); the `EmitResult` |
| `*****MASKED*****` in a value | the masker found sensitive data or a sensitive field name | [ADR-006](../decisions/ADR-006-masking-before-every-sink.md) |
| Events reach the console but not the receiver | the receiver is down, slow (over the timeout, 3 s by default) or answers with an error | the reason in the result: `timeout`, `network_error`, `status_<code>` (see below) |
| A 400 from the AWS endpoint | the body does not fit the `add_log` contract, usually an empty log group or log stream | the settings of the adapter, the contract in [api.md](../api/api.md) |
| The file stops growing or old files disappear | rotation and the quota of the file destination | `max_file_bytes`, `max_files`, `quota_bytes`; the counters |

## Reasons in the result of a send

The reasons are fixed texts: they never hold a URL, a path or a value the caller gave.

- **Shared HTTP client** (built, T-04.4). Outcome classes: `success`, `transient`, `permanent`. Reasons: `status_<code>`, `network_error`, `timeout`, `tls_verification_failed`, `response_unreadable`, `batch_too_large`, `serialization_failed`, `client_failed`. Timeouts: 3000 ms by default (`TELEMETRY_HTTP_TIMEOUT_MS`); a server that never answers ends at it. Retries: at most `max_attempts` (5), waiting the equal-jitter backoff from 200 ms up to 30 s or the `Retry-After` if larger (cut to 24 h); **the destinations send once and do not retry yet**: the delivery mode decides who retries (T-05.3).
- **Console** (built, T-04.2). On the standard output: one line per event written, as the canonical JSON of the envelope or as a readable line (`time severity [category] function: message key=value ...`); an event below the minimum severity is not written and counts as accepted. Reasons: `write_failed` (the output could not be written), `serialization_failed` and `destination_failed`. A control character in a text line is shown as an escape, so a line is always one line.
- **File** (built, T-04.3). The files are in the directory the project configured: `telemetry.jsonl` (the current one) and `telemetry.jsonl.1` ... `telemetry.jsonl.<max_files - 1>` (rotated, the newest first). The file rotates when the next line would pass `max_file_bytes` (10 MiB by default); the total of all the files is kept under `quota_bytes` (50 MiB by default) by deleting the oldest rotated file first. Counters: `FilesRotated`, `FilesDeletedForQuota`, `WriteErrors`, `RotationErrors`, `QuotaErrors`. Reasons: `write_failed`, `rotation_failed`, `serialization_failed`, `destination_failed`. A log shipper reads the current file and the rotated ones.
- **HTTP destination** (built, T-04.5). Accepted: the events the receiver took (2xx) and the ones below the minimum severity (not sent). Failed (transient, a worker may retry later): `status_408`, `status_429`, `status_5xx`, `timeout`, `network_error`, `tls_verification_failed`, `response_unreadable`. Dropped (permanent, not retried): `status_4xx` (other than 408 and 429), `status_3xx` (a redirect is never followed), `batch_too_large`, `serialization_failed`. `destination_failed`: an unexpected failure inside the destination. The full outcome (class, status, `Retry-After`) is in the `Delivery` that `Deliver` returns.
- **AWS HTTP adapter** (built, T-04.6). Accepted: the events whose request got a 2xx and the events that are not `log.*` (skipped, because the `add_log` contract has no place for them; they are also in `Delivery.Skipped`). Failed (transient): the reasons of the shared client. Dropped (permanent): `status_4xx`, `status_3xx`, `batch_too_large` and `serialization_failed` (an event that cannot be written; no request is made). `destination_failed` is the reason when the adapter itself fails. There is one request per `log.*` event, so a batch of N events is N requests.

## Timeouts, retries and shutdown

- The HTTP destinations time out after **3 seconds** by default (`TELEMETRY_HTTP_TIMEOUT_MS`, `TELEMETRY_AWS_HTTP_TIMEOUT_MS`) and **do not retry yet**: a worker sends each batch once and a failed event is not sent again (the retry by delivery mode is T-05.3). Every destination has a **bounded queue** (10,000 events by default) and a worker that sends a batch when it holds 100 events or when the oldest is 2 seconds old (`delivery.queue_size`, `batch_size`, `flush_interval_ms`, `timeout_ms`); a log call only queues and never waits for the network. What a full queue does is `delivery.queue_policy`: `drop_newest` (the default) drops the new event (`queue_full_newest`), `drop_oldest` drops the oldest event of a class not higher (`queue_full_oldest`) and `block_with_timeout` makes the caller wait up to `delivery.block_timeout_ms` and then drops it (`block_timeout`, the one case where a log call waits). A `critical` event displaces a `bulk` one first, then a `normal` one (`displaced_by_critical`). Every drop is counted per destination and reason in the stats of the destination (public with T-05.5); a drop a sink reports counts under its own reason or `unspecified`. A class is a priority for a full queue, not a guarantee of delivery.
- **Call `Shutdown` (Go) or `shutdown` (Python) before the process exits**: it flushes the queues within its timeout (5,000 ms by default, or the deadline of the context) and returns how many events were sent and dropped. **Events still queued when the process ends without it are lost.** A destination that hangs makes the call return at the timeout, with that destination's queued events counted as dropped. After it every log call is dropped with the reason `shutdown`. `Flush` / `flush` does the same without stopping.

## Manual end-to-end check of a deployment

1. `examples/aws_http_smoke`: a sample service, a Go module of its own, that logs through the AWS HTTP adapter to the `add_log` endpoint. It reads `TELEMETRY_AWS_HTTP_HOST`, `TELEMETRY_AWS_HTTP_PORT`, `TELEMETRY_AWS_HTTP_PATH`, `TELEMETRY_AWS_HTTP_LOG_GROUP_NAME` and `TELEMETRY_AWS_HTTP_LOG_STREAM_NAME` from its own environment (the library itself reads none) and passes them in.
2. `go/pkg/logging/aws_http/internal/tests/e2e/deployed_aws_microservice_test.go`: sends one line to a deployed aws_microservice through the adapter and fails when the endpoint does not accept it. Set the constants at the top and the switch `run_against_deployed_service`; the test prints a unique text to search for in the CloudWatch log stream. **The switch is `false` and the inputs are placeholders in anything that is committed**; a developer keeps the real values (the switch on, the real log group) only in a copy that git ignores, named `deployed_aws_microservice_test.go.local` (the `*.local` rule of `.gitignore`), and copies the constants from it when running the test.

## Not present

Metrics, tracing, a retry queue and a disk buffer for delivery (the spool). Delivery is best effort today ([ADR-005](../decisions/ADR-005-a-failing-sink-never-fails-the-caller.md)); the queue and the retries are T-05 and the spool is T-06.

> **Planned (T-02.6, T-03.5, T-05.5):** the diagnostics count the events rejected by the validation at the source (a counter by event type and error code, REQ-DGN-001), the results of the sink calls (accepted, dropped and failed by destination and reason, REQ-DLV-008) and the counters of the limits (attribute keys dropped, attribute keys truncated, attribute values truncated, events rejected; they count items, never text). The validator, the results and the counters exist and are returned with each call; wiring them into the diagnostics is T-05.5. Nothing is emitted yet.
